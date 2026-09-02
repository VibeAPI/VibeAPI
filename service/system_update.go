package service

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/systemupdate"
)

const (
	defaultSystemUpdateReleaseRepository = "VibeAPI/VibeAPI"
	defaultSystemUpdateImageRepository   = "heself/vibeapi"
	defaultSystemUpdateSocketPath        = "/run/vibeapi-updater/updater.sock"
)

var (
	systemUpdateOnce    sync.Once
	systemUpdateService *SystemUpdateService
)

type SystemUpdateService struct {
	catalog        *systemupdate.Catalog
	updater        *systemUpdateUpdaterClient
	currentVersion string
}

func GetSystemUpdateService() *SystemUpdateService {
	systemUpdateOnce.Do(func() {
		timeout := envDurationSeconds("SYSTEM_UPDATE_HTTP_TIMEOUT_SECONDS", 10*time.Second)
		catalog := systemupdate.NewCatalog(systemupdate.CatalogConfig{
			ReleaseRepository: envString("SYSTEM_UPDATE_RELEASE_REPOSITORY", defaultSystemUpdateReleaseRepository),
			ImageRepository:   envString("SYSTEM_UPDATE_IMAGE_REPOSITORY", defaultSystemUpdateImageRepository),
			GitHubAPIBase:     envString("SYSTEM_UPDATE_GITHUB_API_BASE", "https://api.github.com"),
			DockerHubAPIBase:  envString("SYSTEM_UPDATE_DOCKER_HUB_API_BASE", "https://hub.docker.com"),
			GitHubToken:       strings.TrimSpace(os.Getenv("SYSTEM_UPDATE_GITHUB_TOKEN")),
			CacheTTL:          envDurationSeconds("SYSTEM_UPDATE_CACHE_TTL_SECONDS", 5*time.Minute),
			RequestTimeout:    timeout,
		}, nil)
		systemUpdateService = &SystemUpdateService{
			catalog: catalog,
			updater: newSystemUpdateUpdaterClient(
				envString("SYSTEM_UPDATE_UPDATER_SOCKET", defaultSystemUpdateSocketPath),
				strings.TrimSpace(os.Getenv("SYSTEM_UPDATE_UPDATER_TOKEN")), timeout,
			),
			currentVersion: common.Version,
		}
	})
	return systemUpdateService
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envDurationSeconds(name string, fallback time.Duration) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || seconds <= 0 || seconds > 3600 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func (service *SystemUpdateService) Capabilities(ctx context.Context) systemupdate.Capability {
	capability := systemupdate.Capability{
		ReleaseCatalogAvailable: true,
		CurrentVersion:          service.currentVersion,
		ImageRepository:         service.catalogConfigImageRepository(),
		ReleaseRepository:       service.catalogConfigReleaseRepository(),
		UpdaterConfigured:       service.updater.configured(),
	}
	if !service.updater.configured() {
		capability.Reason = "system updater is not configured"
		return capability
	}
	updaterCapability, err := service.updater.capabilities(ctx)
	if err != nil {
		capability.Reason = "system updater is unavailable"
		return capability
	}
	updaterCapability.ReleaseCatalogAvailable = true
	updaterCapability.UpdaterConfigured = true
	updaterCapability.UpdaterReachable = true
	updaterCapability.CurrentVersion = service.currentVersion
	updaterCapability.ImageRepository = service.catalogConfigImageRepository()
	updaterCapability.ReleaseRepository = service.catalogConfigReleaseRepository()
	return updaterCapability
}

func (service *SystemUpdateService) catalogConfigImageRepository() string {
	return envString("SYSTEM_UPDATE_IMAGE_REPOSITORY", defaultSystemUpdateImageRepository)
}
func (service *SystemUpdateService) catalogConfigReleaseRepository() string {
	return envString("SYSTEM_UPDATE_RELEASE_REPOSITORY", defaultSystemUpdateReleaseRepository)
}

func (service *SystemUpdateService) Releases(ctx context.Context) (systemupdate.ReleaseList, error) {
	return service.catalog.List(ctx, service.currentVersion)
}

func (service *SystemUpdateService) resolveRelease(ctx context.Context, releaseID, expectedDigest string) (*systemupdate.Release, error) {
	if !systemupdate.ValidReleaseID(releaseID) {
		return nil, errors.New("invalid release_id")
	}
	if !systemupdate.ValidDigest(expectedDigest) {
		return nil, errors.New("invalid expected_digest")
	}
	list, err := service.Releases(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list.Releases {
		release := &list.Releases[i]
		if release.ID != releaseID {
			continue
		}
		if !release.Available || release.Digest == "" {
			return nil, errors.New("release image is unavailable")
		}
		if release.Digest != expectedDigest {
			return nil, errors.New("release digest changed; refresh the release catalog")
		}
		return release, nil
	}
	return nil, errors.New("release not found")
}

func (service *SystemUpdateService) Preflight(ctx context.Context, request systemupdate.PreflightRequest) (*systemupdate.PreflightResult, error) {
	release, err := service.resolveRelease(ctx, request.ReleaseID, request.ExpectedDigest)
	if err != nil {
		return nil, err
	}
	if !service.updater.configured() {
		return &systemupdate.PreflightResult{ReleaseID: release.ID, Version: release.Version, ImageRef: release.ImageRef,
			Digest: release.Digest, CurrentVersion: service.currentVersion, Action: "update", Ready: false,
			BlockingReasons: []string{"system updater is not configured"}}, nil
	}
	return service.updater.preflight(ctx, systemupdate.PreflightRequest{ReleaseID: release.ID, ExpectedDigest: release.Digest})
}

func (service *SystemUpdateService) StartOperation(ctx context.Context, request systemupdate.StartOperationRequest) (*systemupdate.Operation, error) {
	if !systemupdate.ValidIdempotencyKey(request.IdempotencyKey) {
		return nil, errors.New("invalid idempotency_key")
	}
	release, err := service.resolveRelease(ctx, request.ReleaseID, request.ExpectedDigest)
	if err != nil {
		return nil, err
	}
	if !service.updater.configured() {
		return nil, errors.New("system updater is not configured")
	}
	return service.updater.start(ctx, systemupdate.StartOperationRequest{
		ReleaseID: release.ID, ExpectedDigest: release.Digest, IdempotencyKey: request.IdempotencyKey,
	})
}

func (service *SystemUpdateService) ListOperations(ctx context.Context) ([]systemupdate.Operation, error) {
	if !service.updater.configured() {
		return []systemupdate.Operation{}, nil
	}
	return service.updater.list(ctx)
}

func (service *SystemUpdateService) GetOperation(ctx context.Context, operationID string) (*systemupdate.Operation, error) {
	if !systemupdate.ValidIdempotencyKey(operationID) {
		return nil, errors.New("invalid operation_id")
	}
	if !service.updater.configured() {
		return nil, errors.New("system updater is not configured")
	}
	return service.updater.get(ctx, operationID)
}

type systemUpdateUpdaterClient struct {
	socketPath, token string
	client            *http.Client
}
type systemUpdateOperationList struct {
	Operations []systemupdate.Operation `json:"operations"`
}

func newSystemUpdateUpdaterClient(socketPath, token string, timeout time.Duration) *systemUpdateUpdaterClient {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "unix", socketPath)
	}}
	return &systemUpdateUpdaterClient{socketPath: socketPath, token: token, client: &http.Client{Transport: transport, Timeout: timeout}}
}
func (client *systemUpdateUpdaterClient) configured() bool {
	return client.socketPath != "" && client.token != ""
}
func (client *systemUpdateUpdaterClient) do(ctx context.Context, method, path string, input, output any) error {
	var body *bytes.Reader
	if input != nil {
		data, err := common.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.client.Do(req)
	if err != nil {
		return errors.New("system updater is unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("system updater rejected the request")
	}
	if output == nil {
		return nil
	}
	if err := common.DecodeJson(resp.Body, output); err != nil {
		return errors.New("system updater returned an invalid response")
	}
	return nil
}
func (client *systemUpdateUpdaterClient) capabilities(ctx context.Context) (systemupdate.Capability, error) {
	var out systemupdate.Capability
	err := client.do(ctx, http.MethodGet, "/v1/capabilities", nil, &out)
	return out, err
}
func (client *systemUpdateUpdaterClient) preflight(ctx context.Context, in systemupdate.PreflightRequest) (*systemupdate.PreflightResult, error) {
	var out systemupdate.PreflightResult
	err := client.do(ctx, http.MethodPost, "/v1/preflight", in, &out)
	return &out, err
}
func (client *systemUpdateUpdaterClient) start(ctx context.Context, in systemupdate.StartOperationRequest) (*systemupdate.Operation, error) {
	var out systemupdate.Operation
	err := client.do(ctx, http.MethodPost, "/v1/operations", in, &out)
	return &out, err
}
func (client *systemUpdateUpdaterClient) list(ctx context.Context) ([]systemupdate.Operation, error) {
	var out systemUpdateOperationList
	err := client.do(ctx, http.MethodGet, "/v1/operations", nil, &out)
	return out.Operations, err
}
func (client *systemUpdateUpdaterClient) get(ctx context.Context, id string) (*systemupdate.Operation, error) {
	var out systemupdate.Operation
	err := client.do(ctx, http.MethodGet, "/v1/operations/"+id, nil, &out)
	return &out, err
}
