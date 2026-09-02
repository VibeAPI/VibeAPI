package systemupdater

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type CommandRunner interface {
	Run(ctx context.Context, executable string, args ...string) ([]byte, error)
}

type ExecRunner struct {
	redactor *Redactor
}

func NewExecRunner(secrets ...string) *ExecRunner {
	return &ExecRunner{redactor: NewRedactor(secrets...)}
}

func (r *ExecRunner) Run(ctx context.Context, executable string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, executable, args...)
	output, err := command.CombinedOutput()
	if err == nil {
		return output, nil
	}
	message := strings.TrimSpace(string(output))
	if len(message) > 4096 {
		message = message[len(message)-4096:]
	}
	message = r.redactor.String(message)
	if message == "" {
		return nil, fmt.Errorf("command failed: %w", err)
	}
	return nil, fmt.Errorf("command failed: %w: %s", err, message)
}

type Redactor struct {
	secrets []string
}

func NewRedactor(secrets ...string) *Redactor {
	filtered := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if len(secret) >= 4 {
			filtered = append(filtered, secret)
		}
	}
	return &Redactor{secrets: filtered}
}

func (r *Redactor) String(value string) string {
	for _, secret := range r.secrets {
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
	}
	return value
}
