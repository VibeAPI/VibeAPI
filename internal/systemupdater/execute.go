package systemupdater

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	contract "github.com/QuantumNous/new-api/pkg/systemupdate"
)

// execute returns true only when the terminal operation state was durably
// journaled. A false result deliberately leaves activeID set, blocking another
// update until the updater is restarted and RecoverInterrupted reconciles it.
func (e *Engine) execute(ctx context.Context, operation contract.Operation, release contract.Release) bool {
	releaseHostLock, err := e.lock.TryLock()
	if err != nil {
		return e.fail(&operation, "lock_failed", err)
	}
	defer releaseHostLock()

	current, err := e.currentDeployment(ctx)
	if err != nil {
		return e.fail(&operation, "inspect_failed", err)
	}
	operation.CurrentImageRef = current.imageRef
	operation.CurrentDigest = current.digest
	operation.PreviousImageRef = current.immutableRef
	operation.PreviousDigest = current.digest
	operation.RollbackAvailable = current.digest != ""
	operation.AutomaticRollback = operation.RollbackAvailable
	if err := e.saveOperation(&operation, "pulling", 15, "pulling immutable release image"); err != nil {
		e.logPersistenceFailure(operation.ID, err)
		return false
	}

	targetImage := e.config.ImageRepository + "@" + release.Digest
	targetOverride, err := e.writeOverride(operation.ID+"-target", targetImage)
	if err != nil {
		return e.fail(&operation, "override_failed", err)
	}
	defer os.Remove(targetOverride)

	pullCtx, pullCancel := context.WithTimeout(ctx, e.config.CommandTimeoutDuration)
	_, err = e.runner.Run(pullCtx, e.config.DockerBinary, e.composeWithOverride(targetOverride, "pull", e.config.ComposeService)...)
	pullCancel()
	if err != nil {
		return e.fail(&operation, "pull_failed", err)
	}
	if err := e.verifyPulledDigest(ctx, targetImage, release.Digest); err != nil {
		return e.fail(&operation, "digest_verification_failed", err)
	}
	if err := e.saveOperation(&operation, "switching", 45, "recreating compose service with selected release"); err != nil {
		e.logPersistenceFailure(operation.ID, err)
		return false
	}

	switchCtx, switchCancel := context.WithTimeout(ctx, e.config.ShutdownWaitTimeoutDuration)
	_, err = e.runner.Run(switchCtx, e.config.DockerBinary, e.composeWithOverride(targetOverride, "up", "-d", "--no-deps", "--force-recreate", e.config.ComposeService)...)
	switchCancel()
	if err != nil {
		return e.rollback(ctx, &operation, fmt.Errorf("replace compose service: %w", err))
	}
	if err := e.saveOperation(&operation, "health_check", 65, "waiting for application readiness"); err != nil {
		return e.rollback(ctx, &operation, fmt.Errorf("persist post-switch state: %w", err))
	}
	if err := e.waitHealthy(ctx, release.Version, release.Digest); err != nil {
		return e.rollback(ctx, &operation, err)
	}
	if err := e.commitManagedOverride(targetImage); err != nil {
		return e.rollback(ctx, &operation, fmt.Errorf("persist selected image: %w", err))
	}
	operation.Status = contract.OperationSucceeded
	operation.Phase = "committed"
	operation.Progress = 100
	operation.Message = "release applied and stable readiness window completed"
	operation.CurrentImageRef = targetImage
	operation.CurrentDigest = release.Digest
	operation.UpdatedAt = e.now().Unix()
	if err := e.journal.Save(operation); err != nil {
		e.logPersistenceFailure(operation.ID, err)
		return false
	}
	return true
}

func (e *Engine) verifyPulledDigest(ctx context.Context, imageRef, expectedDigest string) error {
	commandCtx, cancel := context.WithTimeout(ctx, e.config.CommandTimeoutDuration)
	defer cancel()
	output, err := e.runner.Run(commandCtx, e.config.DockerBinary, "image", "inspect", imageRef)
	if err != nil {
		return fmt.Errorf("inspect pulled image: %w", err)
	}
	var inspected []struct {
		RepoDigests []string `json:"RepoDigests"`
	}
	if err := common.Unmarshal(output, &inspected); err != nil || len(inspected) != 1 {
		return fmt.Errorf("decode pulled image inspection")
	}
	for _, digestRef := range inspected[0].RepoDigests {
		if strings.HasSuffix(digestRef, "@"+expectedDigest) && repositoryFromImageRef(digestRef) == e.config.ImageRepository {
			return nil
		}
	}
	return fmt.Errorf("pulled image did not expose expected repository digest")
}

func (e *Engine) waitHealthy(ctx context.Context, expectedVersion, expectedDigest string) error {
	deadline := e.now().Add(e.config.HealthTimeoutDuration)
	var stableSince time.Time
	for {
		if e.now().After(deadline) {
			return fmt.Errorf("application did not remain ready before health timeout")
		}
		readiness, readinessErr := e.readiness(ctx)
		deployment, deploymentErr := e.currentDeployment(ctx)
		ready := readinessErr == nil && deploymentErr == nil && deployment.digest == expectedDigest
		if ready && expectedVersion != "" && readiness.Version != expectedVersion {
			ready = false
		}
		if ready {
			if stableSince.IsZero() {
				stableSince = e.now()
			}
			if e.now().Sub(stableSince) >= e.config.StableWindowDuration {
				return nil
			}
		} else {
			stableSince = time.Time{}
		}
		if err := e.sleep(ctx, e.config.PollIntervalDuration); err != nil {
			return err
		}
	}
}

func (e *Engine) rollback(ctx context.Context, operation *contract.Operation, updateErr error) bool {
	operation.RollbackAttempted = true
	if err := e.saveOperation(operation, "rolling_back", 80, "update failed; restoring previous immutable image"); err != nil {
		e.logPersistenceFailure(operation.ID, err)
	}
	if operation.PreviousDigest == "" {
		return e.fail(operation, "rollback_unavailable", fmt.Errorf("%w; previous image digest is unavailable", updateErr))
	}
	rollbackImage, err := e.rollbackImage(operation)
	if err != nil {
		return e.fail(operation, "rollback_unavailable", fmt.Errorf("%w; %v", updateErr, err))
	}
	rollbackOverride, err := e.writeOverride(operation.ID+"-rollback", rollbackImage)
	if err == nil {
		defer os.Remove(rollbackOverride)
		rollbackCtx, rollbackCancel := context.WithTimeout(ctx, e.config.ShutdownWaitTimeoutDuration)
		_, err = e.runner.Run(rollbackCtx, e.config.DockerBinary, e.composeWithOverride(rollbackOverride, "up", "-d", "--no-deps", "--force-recreate", e.config.ComposeService)...)
		rollbackCancel()
	}
	if err != nil {
		return e.fail(operation, "rollback_failed", fmt.Errorf("%w; automatic rollback failed: %v", updateErr, err))
	}
	if err := e.waitHealthy(ctx, operation.CurrentVersion, operation.PreviousDigest); err != nil {
		return e.fail(operation, "rollback_health_failed", fmt.Errorf("%w; previous image was recreated but did not become healthy: %v", updateErr, err))
	}
	if err := e.commitManagedOverride(rollbackImage); err != nil {
		return e.fail(operation, "rollback_persist_failed", fmt.Errorf("%w; rollback succeeded but its managed override was not persisted: %v", updateErr, err))
	}
	operation.Status = contract.OperationRolledBack
	operation.Phase = "rolled_back"
	operation.Progress = 100
	operation.RollbackSucceeded = true
	operation.ErrorCode = "update_failed_rolled_back"
	operation.Error = e.redactor.String(updateErr.Error())
	operation.Message = "update failed and previous image was restored"
	operation.CurrentImageRef = rollbackImage
	operation.CurrentDigest = operation.PreviousDigest
	operation.UpdatedAt = e.now().Unix()
	if err := e.journal.Save(*operation); err != nil {
		e.logPersistenceFailure(operation.ID, err)
		return false
	}
	return true
}

func (e *Engine) rollbackImage(operation *contract.Operation) (string, error) {
	if !contract.ValidDigest(operation.PreviousDigest) {
		return "", fmt.Errorf("previous image digest is unavailable")
	}
	repository := repositoryFromImageRef(operation.PreviousImageRef)
	if !e.trustedImageRepository(repository) {
		return "", fmt.Errorf("previous image repository is not trusted")
	}
	return repository + "@" + operation.PreviousDigest, nil
}

func (e *Engine) fail(operation *contract.Operation, code string, err error) bool {
	operation.Status = contract.OperationFailed
	operation.Phase = "failed"
	operation.Progress = 100
	operation.ErrorCode = code
	operation.Error = e.redactor.String(err.Error())
	operation.Message = operation.Error
	operation.UpdatedAt = e.now().Unix()
	if saveErr := e.journal.Save(*operation); saveErr != nil {
		e.logPersistenceFailure(operation.ID, saveErr)
		return false
	}
	return true
}

func (e *Engine) logPersistenceFailure(operationID string, err error) {
	log.Printf("system updater operation %s journal persistence failed; blocking further updates until restart: %v", operationID, err)
}
