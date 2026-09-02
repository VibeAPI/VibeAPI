package systemupdater

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var composeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

type Config struct {
	SocketPath            string   `json:"socket_path"`
	StateDir              string   `json:"state_dir"`
	ComposeFiles          []string `json:"compose_files"`
	ComposeProject        string   `json:"compose_project"`
	ComposeService        string   `json:"compose_service"`
	ImageRepository       string   `json:"image_repository"`
	BootstrapRepositories []string `json:"bootstrap_repositories"`
	ReleaseRepository     string   `json:"release_repository"`
	AppURL                string   `json:"app_url"`
	ReadinessPath         string   `json:"readiness_path"`
	LegacyStatusPath      string   `json:"legacy_status_path"`
	ReadinessToken        string   `json:"readiness_token"`
	APIToken              string   `json:"api_token"`
	DockerBinary          string   `json:"docker_binary"`
	HealthTimeout         string   `json:"health_timeout"`
	StableWindow          string   `json:"stable_window"`
	PollInterval          string   `json:"poll_interval"`
	CommandTimeout        string   `json:"command_timeout"`
	ShutdownWaitTimeout   string   `json:"shutdown_wait_timeout"`
	MinimumFreeBytes      uint64   `json:"minimum_free_bytes"`
}

type RuntimeConfig struct {
	Config
	HealthTimeoutDuration       time.Duration
	StableWindowDuration        time.Duration
	PollIntervalDuration        time.Duration
	CommandTimeoutDuration      time.Duration
	ShutdownWaitTimeoutDuration time.Duration
}

func LoadConfig(path string) (RuntimeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("read updater config: %w", err)
	}
	var cfg Config
	if err := common.Unmarshal(data, &cfg); err != nil {
		return RuntimeConfig{}, fmt.Errorf("decode updater config: %w", err)
	}
	return ValidateConfig(cfg)
}

func ValidateConfig(cfg Config) (RuntimeConfig, error) {
	if cfg.ReadinessPath == "" {
		cfg.ReadinessPath = "/api/system-update/readiness"
	}
	if cfg.LegacyStatusPath == "" {
		cfg.LegacyStatusPath = "/api/status"
	}
	if cfg.DockerBinary == "" {
		cfg.DockerBinary = "/usr/bin/docker"
	}
	if cfg.HealthTimeout == "" {
		cfg.HealthTimeout = "5m"
	}
	if cfg.StableWindow == "" {
		cfg.StableWindow = "60s"
	}
	if cfg.PollInterval == "" {
		cfg.PollInterval = "2s"
	}
	if cfg.CommandTimeout == "" {
		cfg.CommandTimeout = "10m"
	}
	if cfg.ShutdownWaitTimeout == "" {
		cfg.ShutdownWaitTimeout = "12m"
	}
	if cfg.MinimumFreeBytes == 0 {
		cfg.MinimumFreeBytes = 2 * 1024 * 1024 * 1024
	}

	requiredPaths := map[string]string{
		"socket_path": cfg.SocketPath,
		"state_dir":   cfg.StateDir,
	}
	for name, path := range requiredPaths {
		if path == "" || !filepath.IsAbs(path) {
			return RuntimeConfig{}, fmt.Errorf("%s must be an absolute path", name)
		}
	}
	if len(cfg.ComposeFiles) == 0 {
		return RuntimeConfig{}, fmt.Errorf("compose_files must contain at least one file")
	}
	for _, path := range cfg.ComposeFiles {
		if !filepath.IsAbs(path) {
			return RuntimeConfig{}, fmt.Errorf("compose file %q must be an absolute path", path)
		}
	}
	if !composeNamePattern.MatchString(cfg.ComposeProject) {
		return RuntimeConfig{}, fmt.Errorf("invalid compose_project")
	}
	if !composeNamePattern.MatchString(cfg.ComposeService) {
		return RuntimeConfig{}, fmt.Errorf("invalid compose_service")
	}
	if cfg.ImageRepository == "" || strings.ContainsAny(cfg.ImageRepository, "@ \t\r\n") {
		return RuntimeConfig{}, fmt.Errorf("image_repository must be a repository without tag or digest")
	}
	if repositoryFromImageRef(cfg.ImageRepository) != cfg.ImageRepository || strings.Count(strings.Trim(cfg.ImageRepository, "/"), "/") != 1 {
		return RuntimeConfig{}, fmt.Errorf("image_repository must not include a tag")
	}
	seenRepositories := map[string]struct{}{cfg.ImageRepository: {}}
	for _, repository := range cfg.BootstrapRepositories {
		if repository == "" || strings.ContainsAny(repository, "@ \t\r\n") ||
			repositoryFromImageRef(repository) != repository ||
			strings.Count(strings.Trim(repository, "/"), "/") != 1 {
			return RuntimeConfig{}, fmt.Errorf("bootstrap repository %q must be a repository without tag or digest", repository)
		}
		if _, exists := seenRepositories[repository]; exists {
			return RuntimeConfig{}, fmt.Errorf("duplicate trusted image repository %q", repository)
		}
		seenRepositories[repository] = struct{}{}
	}
	if strings.Count(strings.Trim(cfg.ReleaseRepository, "/"), "/") != 1 {
		return RuntimeConfig{}, fmt.Errorf("release_repository must be owner/name")
	}
	appURL, err := url.Parse(cfg.AppURL)
	if err != nil || (appURL.Scheme != "http" && appURL.Scheme != "https") || appURL.Host == "" || appURL.User != nil {
		return RuntimeConfig{}, fmt.Errorf("app_url must be an HTTP(S) URL without credentials")
	}
	hostname := appURL.Hostname()
	address := net.ParseIP(hostname)
	if hostname != "localhost" && (address == nil || !address.IsLoopback()) {
		return RuntimeConfig{}, fmt.Errorf("app_url must use a loopback host")
	}
	if !strings.HasPrefix(cfg.ReadinessPath, "/") || strings.ContainsAny(cfg.ReadinessPath, "?#") {
		return RuntimeConfig{}, fmt.Errorf("readiness_path must be an absolute URL path")
	}
	if !strings.HasPrefix(cfg.LegacyStatusPath, "/") || strings.ContainsAny(cfg.LegacyStatusPath, "?#") {
		return RuntimeConfig{}, fmt.Errorf("legacy_status_path must be an absolute URL path")
	}
	if len(cfg.APIToken) < 32 {
		return RuntimeConfig{}, fmt.Errorf("api_token must contain at least 32 characters")
	}
	if len(cfg.ReadinessToken) < 32 {
		return RuntimeConfig{}, fmt.Errorf("readiness_token must contain at least 32 characters")
	}
	if !filepath.IsAbs(cfg.DockerBinary) {
		return RuntimeConfig{}, fmt.Errorf("docker_binary must be an absolute path")
	}

	healthTimeout, err := positiveDuration("health_timeout", cfg.HealthTimeout)
	if err != nil {
		return RuntimeConfig{}, err
	}
	stableWindow, err := positiveDuration("stable_window", cfg.StableWindow)
	if err != nil {
		return RuntimeConfig{}, err
	}
	pollInterval, err := positiveDuration("poll_interval", cfg.PollInterval)
	if err != nil {
		return RuntimeConfig{}, err
	}
	commandTimeout, err := positiveDuration("command_timeout", cfg.CommandTimeout)
	if err != nil {
		return RuntimeConfig{}, err
	}
	shutdownWait, err := positiveDuration("shutdown_wait_timeout", cfg.ShutdownWaitTimeout)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if stableWindow > healthTimeout {
		return RuntimeConfig{}, fmt.Errorf("stable_window cannot exceed health_timeout")
	}

	return RuntimeConfig{
		Config:                      cfg,
		HealthTimeoutDuration:       healthTimeout,
		StableWindowDuration:        stableWindow,
		PollIntervalDuration:        pollInterval,
		CommandTimeoutDuration:      commandTimeout,
		ShutdownWaitTimeoutDuration: shutdownWait,
	}, nil
}

func positiveDuration(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}
