package systemupdate

// Capability describes whether this deployment can browse releases and ask a
// separately privileged updater to replace the running container.
type Capability struct {
	ReleaseCatalogAvailable bool       `json:"release_catalog_available"`
	UpdaterConfigured       bool       `json:"updater_configured"`
	UpdaterReachable        bool       `json:"updater_reachable"`
	CurrentVersion          string     `json:"current_version"`
	ImageRepository         string     `json:"image_repository"`
	ReleaseRepository       string     `json:"release_repository"`
	DeploymentKind          string     `json:"deployment_kind,omitempty"`
	Owner                   string     `json:"owner,omitempty"`
	ComposeProject          string     `json:"compose_project,omitempty"`
	ComposeService          string     `json:"compose_service,omitempty"`
	Architecture            string     `json:"architecture,omitempty"`
	CurrentImageRef         string     `json:"current_image_ref,omitempty"`
	CurrentDigest           string     `json:"current_digest,omitempty"`
	RollbackAvailable       bool       `json:"rollback_available"`
	ActiveOperation         *Operation `json:"active_operation,omitempty"`
	Reason                  string     `json:"reason,omitempty"`
}

type Release struct {
	ID                string   `json:"id"`
	Version           string   `json:"version"`
	Name              string   `json:"name,omitempty"`
	PublishedAt       string   `json:"published_at,omitempty"`
	ReleaseNotes      string   `json:"release_notes,omitempty"`
	ReleaseURL        string   `json:"release_url,omitempty"`
	ImageRef          string   `json:"image_ref"`
	Digest            string   `json:"digest,omitempty"`
	Architectures     []string `json:"architectures,omitempty"`
	Current           bool     `json:"current"`
	Available         bool     `json:"available"`
	UnavailableReason string   `json:"unavailable_reason,omitempty"`
}

type ReleaseList struct {
	Releases []Release `json:"releases"`
	Cached   bool      `json:"cached"`
}

type PreflightRequest struct {
	ReleaseID      string `json:"release_id"`
	ExpectedDigest string `json:"expected_digest"`
}

type PreflightResult struct {
	ReleaseID                  string   `json:"release_id"`
	Version                    string   `json:"version"`
	ImageRef                   string   `json:"image_ref"`
	Digest                     string   `json:"digest"`
	CurrentVersion             string   `json:"current_version"`
	CurrentDigest              string   `json:"current_digest,omitempty"`
	Action                     string   `json:"action"`
	ExpectedDisconnect         bool     `json:"expected_disconnect"`
	AutomaticRollbackAvailable bool     `json:"automatic_rollback_available"`
	Ready                      bool     `json:"ready"`
	Warnings                   []string `json:"warnings,omitempty"`
	BlockingReasons            []string `json:"blocking_reasons,omitempty"`
}

type StartOperationRequest struct {
	ReleaseID      string `json:"release_id"`
	ExpectedDigest string `json:"expected_digest"`
	IdempotencyKey string `json:"idempotency_key"`
}

type OperationStatus string

const (
	OperationPending    OperationStatus = "pending"
	OperationRunning    OperationStatus = "running"
	OperationSucceeded  OperationStatus = "succeeded"
	OperationFailed     OperationStatus = "failed"
	OperationRolledBack OperationStatus = "rolled_back"
)

type Operation struct {
	ID                string          `json:"id"`
	IdempotencyKey    string          `json:"idempotency_key"`
	ReleaseID         string          `json:"release_id"`
	Version           string          `json:"version"`
	ImageRef          string          `json:"image_ref"`
	Digest            string          `json:"digest"`
	CurrentVersion    string          `json:"current_version,omitempty"`
	CurrentImageRef   string          `json:"current_image_ref,omitempty"`
	CurrentDigest     string          `json:"current_digest,omitempty"`
	PreviousImageRef  string          `json:"previous_image_ref,omitempty"`
	PreviousDigest    string          `json:"previous_digest,omitempty"`
	Status            OperationStatus `json:"status"`
	Phase             string          `json:"phase,omitempty"`
	Progress          int             `json:"progress,omitempty"`
	Message           string          `json:"message,omitempty"`
	Error             string          `json:"error,omitempty"`
	RollbackAvailable bool            `json:"rollback_available"`
	AutomaticRollback bool            `json:"automatic_rollback"`
	RollbackAttempted bool            `json:"rollback_attempted"`
	RollbackSucceeded bool            `json:"rollback_succeeded"`
	ErrorCode         string          `json:"error_code,omitempty"`
	CreatedAt         int64           `json:"created_at"`
	UpdatedAt         int64           `json:"updated_at"`
}

type Readiness struct {
	Ready   bool   `json:"ready"`
	Version string `json:"version"`
}
