package config

import (
	"errors"
	"fmt"
	"time"
)

// IntegrationLogMode selects which invocations are recorded to the
// itg_invocation_log table.
type IntegrationLogMode string

const (
	// IntegrationLogOff records nothing.
	IntegrationLogOff IntegrationLogMode = "off"
	// IntegrationLogErrors records failed invocations only.
	IntegrationLogErrors IntegrationLogMode = "errors"
	// IntegrationLogAll records every invocation.
	IntegrationLogAll IntegrationLogMode = "all"
)

// ErrInvalidIntegrationLogMode indicates an unsupported invocation log mode.
var ErrInvalidIntegrationLogMode = errors.New("invalid integration log mode")

// IntegrationLogConfig controls invocation logging.
type IntegrationLogConfig struct {
	// Mode selects which invocations are recorded. Default: errors.
	Mode IntegrationLogMode `config:"mode"`
	// CaptureLimit caps each captured payload (input, output, wire bodies)
	// in bytes; larger payloads are truncated. Default: 4096.
	CaptureLimit int `config:"capture_limit"`
	// MaskFields lists JSON field names (case-insensitive) whose values are
	// masked in captures, on top of the always-masked credential headers.
	MaskFields []string `config:"mask_fields"`
}

// EffectiveMode returns Mode or the errors-only default.
func (c *IntegrationLogConfig) EffectiveMode() IntegrationLogMode {
	if c.Mode == "" {
		return IntegrationLogErrors
	}

	return c.Mode
}

// EffectiveCaptureLimit returns CaptureLimit or its default.
func (c *IntegrationLogConfig) EffectiveCaptureLimit() int {
	return coalescePositive(c.CaptureLimit, 4096)
}

// IntegrationConfig defines integration engine settings.
type IntegrationConfig struct {
	// AutoMigrate runs the integration DDL migration on application start.
	AutoMigrate bool `config:"auto_migrate"`

	// SecretKey is the base64-encoded AES key (16, 24, or 32 bytes) that
	// encrypts sensitive auth parameters at rest. Unset stores them in
	// plaintext and logs a start-up warning.
	SecretKey string `config:"secret_key"`

	// RunTimeout caps each adapter script execution, wire calls included.
	// Default: 30 seconds.
	RunTimeout time.Duration `config:"run_timeout"`

	// MaxResponseBody caps each HTTP response body read by adapter scripts,
	// in bytes. Default: 8 MiB.
	MaxResponseBody int64 `config:"max_response_body"`

	// Log controls invocation logging.
	Log IntegrationLogConfig `config:"log"`
}

// EffectiveRunTimeout returns RunTimeout or its default.
func (c *IntegrationConfig) EffectiveRunTimeout() time.Duration {
	return coalescePositive(c.RunTimeout, 30*time.Second)
}

// EffectiveMaxResponseBody returns MaxResponseBody or its default.
func (c *IntegrationConfig) EffectiveMaxResponseBody() int64 {
	return coalescePositive(c.MaxResponseBody, 8<<20)
}

// Validate rejects unsupported log modes so configuration typos fail at
// startup instead of silently recording nothing.
func (c *IntegrationConfig) Validate() error {
	switch c.Log.EffectiveMode() {
	case IntegrationLogOff, IntegrationLogErrors, IntegrationLogAll:
		return nil
	default:
		return fmt.Errorf("%w %q (want %q, %q, or %q)", ErrInvalidIntegrationLogMode,
			c.Log.Mode, IntegrationLogOff, IntegrationLogErrors, IntegrationLogAll)
	}
}
