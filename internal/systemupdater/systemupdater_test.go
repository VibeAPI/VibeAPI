package systemupdater

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	contract "github.com/QuantumNous/new-api/pkg/systemupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticCatalog struct {
	release contract.Release
}

func (catalog staticCatalog) List(context.Context) (contract.ReleaseList, error) {
	return contract.ReleaseList{Releases: []contract.Release{catalog.release}}, nil
}

func (catalog staticCatalog) Get(_ context.Context, releaseID string) (contract.Release, error) {
	if releaseID != catalog.release.ID {
		return contract.Release{}, errors.New("release not found")
	}
	return catalog.release, nil
}

type recordedCommand struct {
	executable string
	args       []string
}

type fakeRunner struct {
	commands []recordedCommand
	outputs  func([]string) ([]byte, error)
}

type updaterRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn updaterRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func (runner *fakeRunner) Run(_ context.Context, executable string, args ...string) ([]byte, error) {
	runner.commands = append(runner.commands, recordedCommand{executable: executable, args: append([]string(nil), args...)})
	return runner.outputs(args)
}

func testConfig(t *testing.T) RuntimeConfig {
	t.Helper()
	stateDir := t.TempDir()
	composePath := filepath.Join(stateDir, "compose.yml")
	require.NoError(t, os.WriteFile(composePath, []byte("services: {}\n"), 0600))
	config, err := ValidateConfig(Config{
		SocketPath:          filepath.Join(stateDir, "updater.sock"),
		StateDir:            stateDir,
		ComposeFiles:        []string{composePath},
		ComposeProject:      "vibeapi",
		ComposeService:      "new-api",
		ImageRepository:     "heself/vibeapi",
		ReleaseRepository:   "VibeAPI/VibeAPI",
		AppURL:              "http://127.0.0.1:3000",
		ReadinessToken:      strings.Repeat("r", 32),
		APIToken:            strings.Repeat("a", 32),
		DockerBinary:        "/usr/bin/docker",
		HealthTimeout:       "2s",
		StableWindow:        "1s",
		PollInterval:        "1ms",
		CommandTimeout:      "2s",
		ShutdownWaitTimeout: "2s",
	})
	require.NoError(t, err)
	return config
}

func TestConfigRejectsUnscopedComposeAndShortTokens(t *testing.T) {
	config := Config{SocketPath: "relative.sock"}
	_, err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute path")
}

func TestHTTPHandlerRequiresBearerToken(t *testing.T) {
	config := testConfig(t)
	engine := NewEngine(config, staticCatalog{}, &fakeRunner{})
	request := httptest.NewRequest(http.MethodGet, "/v1/operations", nil)
	response := httptest.NewRecorder()

	NewHandler(engine, config.APIToken).ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestManagedOverrideIsAtomicAndAppliedLast(t *testing.T) {
	config := testConfig(t)
	engine := NewEngine(config, staticCatalog{}, &fakeRunner{})
	image := "heself/vibeapi@sha256:" + strings.Repeat("a", 64)

	require.NoError(t, engine.commitManagedOverride(image))
	data, err := os.ReadFile(engine.managedOverridePath())
	require.NoError(t, err)
	assert.Contains(t, string(data), "image: "+image)

	args := engine.composeArgs("config", "--services")
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, []string{"-f", engine.managedOverridePath(), "config", "--services"}, args[len(args)-4:])
}

func TestCurrentDeploymentResolvesAllowedRepositoryDigest(t *testing.T) {
	config := testConfig(t)
	digest := "sha256:" + strings.Repeat("b", 64)
	runner := &fakeRunner{outputs: func(args []string) ([]byte, error) {
		switch {
		case len(args) >= 3 && args[len(args)-3] == "ps":
			return []byte("container-id\n"), nil
		case len(args) > 0 && args[0] == "inspect":
			return []byte(`[{"Image":"image-id","Config":{"Image":"heself/vibeapi:latest"}}]`), nil
		case len(args) > 0 && args[0] == "image":
			return []byte(`[{"RepoDigests":["untrusted/example@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","heself/vibeapi@` + digest + `"]}]`), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}}
	engine := NewEngine(config, staticCatalog{}, runner)

	deployment, err := engine.currentDeployment(t.Context())

	require.NoError(t, err)
	assert.Equal(t, digest, deployment.digest)
}

func TestCurrentDeploymentResolvesExplicitBootstrapRepository(t *testing.T) {
	config := testConfig(t)
	config.BootstrapRepositories = []string{"calciumion/new-api"}
	digest := "sha256:" + strings.Repeat("c", 64)
	runner := &fakeRunner{outputs: func(args []string) ([]byte, error) {
		switch {
		case len(args) >= 3 && args[len(args)-3] == "ps":
			return []byte("container-id\n"), nil
		case len(args) > 0 && args[0] == "inspect":
			return []byte(`[{"Image":"image-id","Config":{"Image":"calciumion/new-api:latest"}}]`), nil
		case len(args) > 0 && args[0] == "image":
			return []byte(`[{"RepoDigests":["heself/vibeapi@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","calciumion/new-api@` + digest + `"]}]`), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}}
	engine := NewEngine(config, staticCatalog{}, runner)

	deployment, err := engine.currentDeployment(t.Context())

	require.NoError(t, err)
	assert.Equal(t, digest, deployment.digest)
	assert.Equal(t, "calciumion/new-api@"+digest, deployment.immutableRef)
}

func TestReadinessDecodesEnvelopeAndFallsBackForHistoricalRelease(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		dedicated bool
	}{
		{name: "dedicated endpoint", dedicated: true},
		{name: "legacy historical endpoint", dedicated: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := testConfig(t)
			engine := NewEngine(config, staticCatalog{}, &fakeRunner{})
			engine.httpClient = &http.Client{Transport: updaterRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if testCase.dedicated {
					assert.Equal(t, "/api/system-update/readiness", request.URL.Path)
				} else if request.URL.Path == "/api/system-update/readiness" {
					return jsonResponse(http.StatusNotFound, ""), nil
				} else {
					assert.Equal(t, "/api/status", request.URL.Path)
				}
				return jsonResponse(http.StatusOK, `{"success":true,"data":{"ready":true,"version":"v1.2.3"}}`), nil
			})}

			readiness, err := engine.readiness(t.Context())

			require.NoError(t, err)
			assert.True(t, readiness.Ready)
			assert.Equal(t, "v1.2.3", readiness.Version)
		})
	}
}

type failOnceJournal struct {
	delegate *Journal
	failOn   int
	saves    int
}

func (journal *failOnceJournal) Save(operation contract.Operation) error {
	journal.saves++
	if journal.saves == journal.failOn {
		return errors.New("injected journal failure")
	}
	return journal.delegate.Save(operation)
}

func (journal *failOnceJournal) Get(operationID string) (contract.Operation, error) {
	return journal.delegate.Get(operationID)
}

func (journal *failOnceJournal) List() ([]contract.Operation, error) {
	return journal.delegate.List()
}

type switchingRunner struct {
	mu              sync.Mutex
	repository      string
	digest          string
	version         string
	previousDigest  string
	targetDigest    string
	composeSwitches int
}

func (runner *switchingRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	switch {
	case len(args) >= 3 && args[len(args)-3] == "ps":
		return []byte("container-id\n"), nil
	case len(args) > 0 && args[0] == "inspect":
		return []byte(`[{"Image":"image-id","Config":{"Image":"` + runner.repository + `@` + runner.digest + `"}}]`), nil
	case len(args) > 0 && args[0] == "image" && args[1] == "inspect":
		imageRef := args[2]
		return []byte(`[{"RepoDigests":["` + imageRef + `"]}]`), nil
	case containsArgument(args, "pull"):
		return nil, nil
	case containsArgument(args, "up"):
		runner.composeSwitches++
		if runner.composeSwitches == 1 {
			runner.repository = "heself/vibeapi"
			runner.digest = runner.targetDigest
			runner.version = "v2.0.0"
		} else {
			runner.repository = "calciumion/new-api"
			runner.digest = runner.previousDigest
			runner.version = "v1.0.0"
		}
		return nil, nil
	default:
		return nil, errors.New("unexpected command")
	}
}

func containsArgument(arguments []string, expected string) bool {
	for _, argument := range arguments {
		if argument == expected {
			return true
		}
	}
	return false
}

func TestPostSwitchJournalFailureRollsBackAndPersistsTerminalState(t *testing.T) {
	config := testConfig(t)
	config.BootstrapRepositories = []string{"calciumion/new-api"}
	previousDigest := "sha256:" + strings.Repeat("1", 64)
	targetDigest := "sha256:" + strings.Repeat("2", 64)
	runner := &switchingRunner{
		repository: "calciumion/new-api", digest: previousDigest, version: "v1.0.0",
		previousDigest: previousDigest, targetDigest: targetDigest,
	}
	engine := NewEngine(config, staticCatalog{}, runner)
	engine.httpClient = &http.Client{Transport: updaterRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		runner.mu.Lock()
		version := runner.version
		runner.mu.Unlock()
		return jsonResponse(http.StatusOK, `{"success":true,"data":{"ready":true,"version":"`+version+`"}}`), nil
	})}
	journal := &failOnceJournal{delegate: NewJournal(config.StateDir), failOn: 3}
	engine.journal = journal
	virtualNow := time.Unix(1_000, 0)
	engine.now = func() time.Time { return virtualNow }
	engine.sleep = func(_ context.Context, duration time.Duration) error {
		virtualNow = virtualNow.Add(duration)
		return nil
	}
	release := contract.Release{ID: "v2.0.0", Version: "v2.0.0", Digest: targetDigest, Available: true}
	operation := contract.Operation{
		ID: "operation-123", IdempotencyKey: "request-123", ReleaseID: release.ID,
		Version: release.Version, Digest: release.Digest, CurrentVersion: "v1.0.0",
	}

	terminalPersisted := engine.execute(t.Context(), operation, release)

	assert.True(t, terminalPersisted)
	assert.Equal(t, 2, runner.composeSwitches)
	stored, err := journal.Get(operation.ID)
	require.NoError(t, err)
	assert.Equal(t, contract.OperationRolledBack, stored.Status)
	assert.True(t, stored.RollbackSucceeded)
	assert.Equal(t, previousDigest, stored.CurrentDigest)
}

func TestStartOperationRejectsChangedDigest(t *testing.T) {
	config := testConfig(t)
	release := contract.Release{
		ID: "v1.2.3", Version: "v1.2.3", ImageRef: "heself/vibeapi:v1.2.3",
		Digest: "sha256:" + strings.Repeat("d", 64), Available: true,
	}
	engine := NewEngine(config, staticCatalog{release: release}, &fakeRunner{})

	_, err := engine.Start(t.Context(), contract.StartOperationRequest{
		ReleaseID: release.ID, ExpectedDigest: "sha256:" + strings.Repeat("e", 64), IdempotencyKey: "update:20260901:abcdef",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected_digest")
}

func TestJournalPersistsAndReturnsNewestFirst(t *testing.T) {
	journal := NewJournal(t.TempDir())
	require.NoError(t, journal.Save(contract.Operation{ID: "first-op", CreatedAt: time.Now().Add(-time.Minute).Unix()}))
	require.NoError(t, journal.Save(contract.Operation{ID: "second-op", CreatedAt: time.Now().Unix()}))

	operations, err := journal.List()

	require.NoError(t, err)
	require.Len(t, operations, 2)
	assert.Equal(t, "second-op", operations[0].ID)
}
