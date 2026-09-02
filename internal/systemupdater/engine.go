package systemupdater

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	contract "github.com/QuantumNous/new-api/pkg/systemupdate"
	"golang.org/x/sys/unix"
)

type Engine struct {
	config     RuntimeConfig
	catalog    Catalog
	runner     CommandRunner
	journal    OperationJournal
	lock       UpdateLock
	httpClient *http.Client
	now        func() time.Time
	sleep      func(context.Context, time.Duration) error
	redactor   *Redactor

	activeMu sync.Mutex
	activeID string
	workers  sync.WaitGroup
}

type OperationJournal interface {
	Save(contract.Operation) error
	Get(string) (contract.Operation, error)
	List() ([]contract.Operation, error)
}

type containerInspect struct {
	Image  string `json:"Image"`
	Config struct {
		Image string `json:"Image"`
	} `json:"Config"`
}

func (e *Engine) managedOverridePath() string {
	return filepath.Join(e.config.StateDir, "managed-override.yaml")
}

func NewEngine(config RuntimeConfig, catalog Catalog, runner CommandRunner) *Engine {
	return &Engine{
		config:     config,
		catalog:    catalog,
		runner:     runner,
		journal:    NewJournal(config.StateDir),
		lock:       NewFileLock(config.StateDir),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		now:        time.Now,
		sleep: func(ctx context.Context, duration time.Duration) error {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		redactor: NewRedactor(config.APIToken, config.ReadinessToken),
	}
}

func (e *Engine) RecoverInterrupted(ctx context.Context) error {
	operations, err := e.journal.List()
	if err != nil {
		return err
	}
	for _, operation := range operations {
		if operation.Status != contract.OperationPending && operation.Status != contract.OperationRunning {
			continue
		}
		current, inspectErr := e.currentDeployment(ctx)
		if inspectErr == nil && current.digest == operation.Digest {
			if healthErr := e.waitHealthy(ctx, operation.Version, operation.Digest); healthErr == nil {
				image := e.config.ImageRepository + "@" + operation.Digest
				if persistErr := e.commitManagedOverride(image); persistErr == nil {
					operation.Status = contract.OperationSucceeded
					operation.Phase = "committed"
					operation.Progress = 100
					operation.Message = "interrupted update recovered and verified"
					operation.CurrentImageRef = image
					operation.CurrentDigest = operation.Digest
					operation.UpdatedAt = e.now().Unix()
					if err := e.journal.Save(operation); err != nil {
						return err
					}
					continue
				}
			}
		}
		if inspectErr == nil && operation.PreviousDigest != "" && current.digest == operation.PreviousDigest {
			image, imageErr := e.rollbackImage(&operation)
			healthErr := e.waitHealthy(ctx, operation.CurrentVersion, operation.PreviousDigest)
			if imageErr == nil && healthErr == nil {
				if persistErr := e.commitManagedOverride(image); persistErr != nil {
					return persistErr
				}
				operation.Status = contract.OperationRolledBack
				operation.Phase = "rolled_back"
				operation.Progress = 100
				operation.RollbackAttempted = true
				operation.RollbackSucceeded = true
				operation.ErrorCode = "agent_interrupted_rolled_back"
				operation.Error = "updater restarted after the previous image was restored"
				operation.Message = operation.Error
				operation.CurrentImageRef = image
				operation.CurrentDigest = operation.PreviousDigest
				operation.UpdatedAt = e.now().Unix()
				if err := e.journal.Save(operation); err != nil {
					return err
				}
				continue
			}
		}
		if operation.PreviousDigest != "" {
			if !e.rollback(ctx, &operation, errors.New("updater restarted before the operation completed")) {
				return errors.New("interrupted operation could not be durably reconciled")
			}
			continue
		}
		operation.Status = contract.OperationFailed
		operation.Phase = "interrupted"
		operation.Progress = 100
		operation.ErrorCode = "agent_interrupted"
		operation.Error = "updater stopped before a rollback image was recorded; inspect the deployment before retrying"
		operation.Message = operation.Error
		operation.UpdatedAt = e.now().Unix()
		if err := e.journal.Save(operation); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) Capabilities(ctx context.Context) (contract.Capability, error) {
	current, err := e.currentDeployment(ctx)
	capability := contract.Capability{
		ReleaseCatalogAvailable: true,
		UpdaterConfigured:       true,
		UpdaterReachable:        true,
		ImageRepository:         e.config.ImageRepository,
		ReleaseRepository:       e.config.ReleaseRepository,
		DeploymentKind:          "docker_compose",
		Owner:                   "vibeapi-updater",
		ComposeProject:          e.config.ComposeProject,
		ComposeService:          e.config.ComposeService,
		Architecture:            runtime.GOARCH,
		RollbackAvailable:       current.digest != "",
	}
	if err != nil {
		capability.Reason = e.redactor.String(err.Error())
	} else {
		capability.CurrentImageRef = current.imageRef
		capability.CurrentDigest = current.digest
		if readiness, readinessErr := e.readiness(ctx); readinessErr == nil {
			capability.CurrentVersion = readiness.Version
		}
	}
	e.activeMu.Lock()
	activeID := e.activeID
	e.activeMu.Unlock()
	if activeID != "" {
		if operation, operationErr := e.journal.Get(activeID); operationErr == nil {
			capability.ActiveOperation = &operation
		}
	}
	return capability, nil
}

func (e *Engine) Releases(ctx context.Context) (contract.ReleaseList, error) {
	list, err := e.catalog.List(ctx)
	if err != nil {
		return contract.ReleaseList{}, err
	}
	current, _ := e.currentDeployment(ctx)
	for i := range list.Releases {
		list.Releases[i].Current = list.Releases[i].Digest == current.digest
	}
	return list, nil
}

func (e *Engine) Preflight(ctx context.Context, request contract.PreflightRequest) (contract.PreflightResult, error) {
	release, err := e.resolveRelease(ctx, request.ReleaseID, request.ExpectedDigest)
	if err != nil {
		return contract.PreflightResult{}, err
	}
	result := contract.PreflightResult{
		ReleaseID:                  release.ID,
		Version:                    release.Version,
		ImageRef:                   release.ImageRef,
		Digest:                     release.Digest,
		Action:                     "update",
		ExpectedDisconnect:         true,
		AutomaticRollbackAvailable: false,
	}
	current, err := e.currentDeployment(ctx)
	if err != nil {
		result.BlockingReasons = append(result.BlockingReasons, e.redactor.String(err.Error()))
	} else {
		result.CurrentDigest = current.digest
		result.AutomaticRollbackAvailable = current.digest != ""
		if current.digest == release.Digest {
			result.Action = "reinstall"
		}
	}
	if readiness, readinessErr := e.readiness(ctx); readinessErr == nil {
		result.CurrentVersion = readiness.Version
	} else {
		result.Warnings = append(result.Warnings, "current application readiness endpoint is unavailable")
	}
	if !release.Available {
		reason := release.UnavailableReason
		if reason == "" {
			reason = "selected release is unavailable"
		}
		result.BlockingReasons = append(result.BlockingReasons, reason)
	}
	if len(release.Architectures) > 0 {
		architectureAvailable := false
		for _, architecture := range release.Architectures {
			if architecture == runtime.GOARCH {
				architectureAvailable = true
				break
			}
		}
		if !architectureAvailable {
			result.BlockingReasons = append(result.BlockingReasons, "selected release does not support this host architecture")
		}
	}
	if err := e.validateCompose(ctx); err != nil {
		result.BlockingReasons = append(result.BlockingReasons, e.redactor.String(err.Error()))
	}
	spaceCtx, spaceCancel := context.WithTimeout(ctx, e.config.CommandTimeoutDuration)
	dockerRootOutput, spaceErr := e.runner.Run(spaceCtx, e.config.DockerBinary, "info", "--format", "{{.DockerRootDir}}")
	spaceCancel()
	dockerRoot := strings.TrimSpace(string(dockerRootOutput))
	if spaceErr != nil || !filepath.IsAbs(dockerRoot) {
		result.BlockingReasons = append(result.BlockingReasons, "Docker storage free space could not be checked")
	} else {
		var filesystem unix.Statfs_t
		if statErr := unix.Statfs(dockerRoot, &filesystem); statErr != nil {
			result.BlockingReasons = append(result.BlockingReasons, "Docker storage free space could not be checked")
		} else if uint64(filesystem.Bavail)*uint64(filesystem.Bsize) < e.config.MinimumFreeBytes {
			result.BlockingReasons = append(result.BlockingReasons, fmt.Sprintf("Docker storage has less than %d bytes available", e.config.MinimumFreeBytes))
		}
	}
	for _, path := range e.config.ComposeFiles {
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() {
			result.BlockingReasons = append(result.BlockingReasons, fmt.Sprintf("compose file %q is not a readable regular file", path))
		}
	}
	if result.CurrentDigest != "" && result.CurrentDigest != release.Digest {
		result.Warnings = append(result.Warnings, "database migrations may make an image rollback incompatible; create a database backup before applying")
	}
	result.Ready = len(result.BlockingReasons) == 0
	return result, nil
}

func (e *Engine) Start(ctx context.Context, request contract.StartOperationRequest) (contract.Operation, error) {
	if !contract.ValidIdempotencyKey(request.IdempotencyKey) {
		return contract.Operation{}, fmt.Errorf("invalid idempotency_key")
	}
	release, err := e.resolveRelease(ctx, request.ReleaseID, request.ExpectedDigest)
	if err != nil {
		return contract.Operation{}, err
	}
	operations, err := e.journal.List()
	if err != nil {
		return contract.Operation{}, err
	}
	for _, operation := range operations {
		if operation.IdempotencyKey == request.IdempotencyKey {
			if operation.ReleaseID != release.ID || operation.Digest != release.Digest {
				return contract.Operation{}, fmt.Errorf("idempotency_key was already used for a different release")
			}
			return operation, nil
		}
	}

	e.activeMu.Lock()
	defer e.activeMu.Unlock()
	if e.activeID != "" {
		return contract.Operation{}, ErrUpdateLocked
	}
	preflight, err := e.Preflight(ctx, contract.PreflightRequest{ReleaseID: release.ID, ExpectedDigest: release.Digest})
	if err != nil {
		return contract.Operation{}, err
	}
	if !preflight.Ready {
		return contract.Operation{}, fmt.Errorf("preflight blocked: %s", strings.Join(preflight.BlockingReasons, "; "))
	}
	id, err := randomID()
	if err != nil {
		return contract.Operation{}, err
	}
	now := e.now().Unix()
	operation := contract.Operation{
		ID:                id,
		IdempotencyKey:    request.IdempotencyKey,
		ReleaseID:         release.ID,
		Version:           release.Version,
		ImageRef:          release.ImageRef,
		Digest:            release.Digest,
		CurrentVersion:    preflight.CurrentVersion,
		CurrentDigest:     preflight.CurrentDigest,
		Status:            contract.OperationPending,
		Phase:             "queued",
		Progress:          0,
		Message:           "update queued",
		RollbackAvailable: preflight.AutomaticRollbackAvailable,
		AutomaticRollback: preflight.AutomaticRollbackAvailable,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := e.journal.Save(operation); err != nil {
		return contract.Operation{}, err
	}
	e.activeID = operation.ID
	e.workers.Add(1)
	go func() {
		defer e.workers.Done()
		if !e.execute(context.WithoutCancel(ctx), operation, release) {
			return
		}
		e.activeMu.Lock()
		if e.activeID == operation.ID {
			e.activeID = ""
		}
		e.activeMu.Unlock()
	}()
	return operation, nil
}

func (e *Engine) Operation(operationID string) (contract.Operation, error) {
	return e.journal.Get(operationID)
}

func (e *Engine) Operations() ([]contract.Operation, error) {
	return e.journal.List()
}

type deploymentState struct {
	containerID  string
	imageRef     string
	digest       string
	immutableRef string
}

func (e *Engine) trustedImageRepository(repository string) bool {
	if repository == e.config.ImageRepository {
		return true
	}
	for _, trusted := range e.config.BootstrapRepositories {
		if repository == trusted {
			return true
		}
	}
	return false
}

func (e *Engine) currentDeployment(ctx context.Context) (deploymentState, error) {
	commandCtx, cancel := context.WithTimeout(ctx, e.config.CommandTimeoutDuration)
	defer cancel()
	output, err := e.runner.Run(commandCtx, e.config.DockerBinary, e.composeArgs("ps", "-q", e.config.ComposeService)...)
	if err != nil {
		return deploymentState{}, fmt.Errorf("locate compose service container: %w", err)
	}
	containerID := strings.TrimSpace(string(output))
	if containerID == "" || strings.ContainsAny(containerID, " \t\r\n") {
		return deploymentState{}, fmt.Errorf("compose service does not resolve to exactly one running container")
	}
	inspectCtx, inspectCancel := context.WithTimeout(ctx, e.config.CommandTimeoutDuration)
	defer inspectCancel()
	output, err = e.runner.Run(inspectCtx, e.config.DockerBinary, "inspect", containerID)
	if err != nil {
		return deploymentState{}, fmt.Errorf("inspect compose service container: %w", err)
	}
	var inspected []containerInspect
	if err := common.Unmarshal(output, &inspected); err != nil || len(inspected) != 1 {
		return deploymentState{}, fmt.Errorf("decode compose service container inspection")
	}
	if inspected[0].Image == "" {
		return deploymentState{}, fmt.Errorf("running container has no immutable image id")
	}
	digest := ""
	immutableRef := ""
	imageRef := inspected[0].Config.Image
	if at := strings.LastIndex(imageRef, "@sha256:"); at >= 0 && e.trustedImageRepository(repositoryFromImageRef(imageRef)) {
		digest = imageRef[at+1:]
		immutableRef = imageRef
	}
	if digest == "" {
		imageCtx, imageCancel := context.WithTimeout(ctx, e.config.CommandTimeoutDuration)
		defer imageCancel()
		imageOutput, imageErr := e.runner.Run(imageCtx, e.config.DockerBinary, "image", "inspect", inspected[0].Image)
		if imageErr != nil {
			return deploymentState{}, fmt.Errorf("inspect running image: %w", imageErr)
		}
		var images []struct {
			RepoDigests []string `json:"RepoDigests"`
		}
		if unmarshalErr := common.Unmarshal(imageOutput, &images); unmarshalErr != nil || len(images) != 1 {
			return deploymentState{}, fmt.Errorf("decode running image inspection")
		}
		configuredRepository := repositoryFromImageRef(imageRef)
		for _, repoDigest := range images[0].RepoDigests {
			repository := repositoryFromImageRef(repoDigest)
			if !e.trustedImageRepository(repository) || (e.trustedImageRepository(configuredRepository) && repository != configuredRepository) {
				continue
			}
			at := strings.LastIndexByte(repoDigest, '@')
			if at >= 0 && contract.ValidDigest(repoDigest[at+1:]) {
				digest = repoDigest[at+1:]
				immutableRef = repoDigest
				break
			}
		}
	}
	if digest == "" {
		return deploymentState{}, fmt.Errorf("running image has no digest in the allowed repository")
	}
	return deploymentState{containerID: containerID, imageRef: imageRef, digest: digest, immutableRef: immutableRef}, nil
}

func (e *Engine) validateCompose(ctx context.Context) error {
	commandCtx, cancel := context.WithTimeout(ctx, e.config.CommandTimeoutDuration)
	defer cancel()
	output, err := e.runner.Run(commandCtx, e.config.DockerBinary, e.composeArgs("config", "--services")...)
	if err != nil {
		return fmt.Errorf("validate compose deployment: %w", err)
	}
	for _, service := range strings.Fields(string(output)) {
		if service == e.config.ComposeService {
			return nil
		}
	}
	return fmt.Errorf("configured compose service was not found")
}

func (e *Engine) resolveRelease(ctx context.Context, releaseID, expectedDigest string) (contract.Release, error) {
	if !contract.ValidReleaseID(releaseID) {
		return contract.Release{}, fmt.Errorf("release_id is required")
	}
	release, err := e.catalog.Get(ctx, releaseID)
	if err != nil {
		return contract.Release{}, err
	}
	if expectedDigest == "" || expectedDigest != release.Digest {
		return contract.Release{}, fmt.Errorf("expected_digest does not match the catalog release")
	}
	return release, nil
}

func (e *Engine) composeArgs(command ...string) []string {
	args := []string{"compose", "--project-name", e.config.ComposeProject}
	for _, path := range e.config.ComposeFiles {
		args = append(args, "-f", path)
	}
	if _, err := os.Stat(e.managedOverridePath()); err == nil {
		args = append(args, "-f", e.managedOverridePath())
	}
	return append(args, command...)
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate operation id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func (e *Engine) readiness(ctx context.Context) (contract.Readiness, error) {
	endpoint := strings.TrimRight(e.config.AppURL, "/") + e.config.ReadinessPath
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return contract.Readiness{}, err
	}
	request.Header.Set("Authorization", "Bearer "+e.config.ReadinessToken)
	response, err := e.httpClient.Do(request)
	if err != nil {
		return contract.Readiness{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return e.legacyReadiness(ctx)
	}
	if response.StatusCode != http.StatusOK {
		return contract.Readiness{}, fmt.Errorf("readiness returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Success bool               `json:"success"`
		Data    contract.Readiness `json:"data"`
	}
	if err := common.DecodeJson(response.Body, &envelope); err != nil {
		return contract.Readiness{}, fmt.Errorf("decode readiness: %w", err)
	}
	if !envelope.Success {
		return contract.Readiness{}, errors.New("application readiness request failed")
	}
	readiness := envelope.Data
	if !readiness.Ready {
		return readiness, errors.New("application is not ready")
	}
	return readiness, nil
}

func (e *Engine) legacyReadiness(ctx context.Context) (contract.Readiness, error) {
	endpoint := strings.TrimRight(e.config.AppURL, "/") + e.config.LegacyStatusPath
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return contract.Readiness{}, err
	}
	response, err := e.httpClient.Do(request)
	if err != nil {
		return contract.Readiness{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return contract.Readiness{}, fmt.Errorf("legacy status returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := common.DecodeJson(response.Body, &envelope); err != nil {
		return contract.Readiness{}, fmt.Errorf("decode legacy status: %w", err)
	}
	if !envelope.Success || strings.TrimSpace(envelope.Data.Version) == "" {
		return contract.Readiness{}, errors.New("legacy application status is not ready")
	}
	return contract.Readiness{Ready: true, Version: envelope.Data.Version}, nil
}

func (e *Engine) saveOperation(operation *contract.Operation, phase string, progress int, message string) error {
	operation.Status = contract.OperationRunning
	operation.Phase = phase
	operation.Progress = progress
	operation.Message = message
	operation.UpdatedAt = e.now().Unix()
	return e.journal.Save(*operation)
}

func (e *Engine) writeOverride(operationID, image string) (string, error) {
	dir := filepath.Join(e.config.StateDir, "overrides")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, operationID+".yaml")
	content := fmt.Sprintf("services:\n  %s:\n    image: %s\n", e.config.ComposeService, image)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", err
	}
	return path, nil
}

func (e *Engine) commitManagedOverride(image string) error {
	if err := os.MkdirAll(e.config.StateDir, 0700); err != nil {
		return fmt.Errorf("create updater state directory: %w", err)
	}
	content := fmt.Sprintf("services:\n  %s:\n    image: %s\n", e.config.ComposeService, image)
	temporary, err := os.CreateTemp(e.config.StateDir, ".managed-override-*.tmp")
	if err != nil {
		return fmt.Errorf("create managed override: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure managed override: %w", err)
	}
	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write managed override: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync managed override: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close managed override: %w", err)
	}
	if err := os.Rename(temporaryPath, e.managedOverridePath()); err != nil {
		return fmt.Errorf("commit managed override: %w", err)
	}
	return nil
}

func (e *Engine) composeWithOverride(override string, command ...string) []string {
	args := []string{"compose", "--project-name", e.config.ComposeProject}
	for _, path := range e.config.ComposeFiles {
		args = append(args, "-f", path)
	}
	if _, err := os.Stat(e.managedOverridePath()); err == nil {
		args = append(args, "-f", e.managedOverridePath())
	}
	args = append(args, "-f", override)
	return append(args, command...)
}

func (e *Engine) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		e.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
