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
	// Retention prunes invocation log rows older than this window; the sweep
	// runs hourly. Zero (the default) keeps rows forever — deletion of the
	// integration evidence trail is strictly opt-in.
	Retention time.Duration `config:"retention"`
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

// IntegrationSecretAlgorithm selects the symmetric cipher that seals
// sensitive integration values (auth parameters, data source passwords) at
// rest.
type IntegrationSecretAlgorithm string

const (
	// IntegrationSecretAlgorithmAES seals secrets with AES-GCM (16-, 24-, or
	// 32-byte key).
	IntegrationSecretAlgorithmAES IntegrationSecretAlgorithm = "aes"
	// IntegrationSecretAlgorithmSM4 seals secrets with SM4-GCM (the Chinese
	// national standard cipher, 16-byte key).
	IntegrationSecretAlgorithmSM4 IntegrationSecretAlgorithm = "sm4"
)

// ErrInvalidIntegrationSecretAlgorithm indicates an unsupported secret
// algorithm.
var ErrInvalidIntegrationSecretAlgorithm = errors.New("invalid integration secret algorithm")

// IntegrationConfig defines integration engine settings.
type IntegrationConfig struct {
	// AutoMigrate runs the integration DDL migration on application start.
	AutoMigrate bool `config:"auto_migrate"`

	// SecretKey is the base64-encoded key that encrypts sensitive auth
	// parameters at rest, sized for SecretAlgorithm (AES: 16, 24, or 32
	// bytes; SM4: 16 bytes). Unset stores them in plaintext and logs a
	// start-up warning.
	SecretKey string `config:"secret_key"`

	// SecretAlgorithm selects the cipher for SecretKey: "aes" (AES-GCM, the
	// default) or "sm4" (SM4-GCM). Values sealed with one algorithm are not
	// readable under the other — switching requires re-entering stored
	// secrets.
	SecretAlgorithm IntegrationSecretAlgorithm `config:"secret_algorithm"`

	// RunTimeout caps each adapter script execution, wire calls included.
	// Default: 30 seconds.
	RunTimeout time.Duration `config:"run_timeout"`

	// MaxResponseBody caps each HTTP response body read by adapter scripts,
	// in bytes. Default: 8 MiB.
	MaxResponseBody int64 `config:"max_response_body"`

	// Log controls invocation logging.
	Log IntegrationLogConfig `config:"log"`

	// Inbound configures the inbound gateways receiving calls initiated by
	// external systems.
	Inbound IntegrationInboundConfig `config:"inbound"`
}

// Default values for IntegrationInboundRateLimitConfig, applied by the
// Effective* accessors.
const (
	DefaultIntegrationInboundRateLimitMax    = 120
	DefaultIntegrationInboundRateLimitPeriod = time.Minute
)

// IntegrationInboundConfig configures the inbound gateways
// (vef.integration.inbound).
type IntegrationInboundConfig struct {
	// RateLimit bounds inbound deliveries on the HTTP gateway.
	RateLimit IntegrationInboundRateLimitConfig `config:"rate_limit"`
}

// IntegrationInboundRateLimitConfig bounds inbound delivery throughput. The
// limiter counts per (system, client IP) in a sliding window held in process
// memory, so in a multi-node deployment each node enforces the limit
// independently.
type IntegrationInboundRateLimitConfig struct {
	// Max is the number of deliveries admitted per window; 0 resolves to the
	// default. Default: 120.
	Max int `config:"max"`
	// Period is the sliding-window length; 0 resolves to the default.
	// Default: 1m.
	Period time.Duration `config:"period"`
}

// EffectiveMax returns Max or its default.
func (c *IntegrationInboundRateLimitConfig) EffectiveMax() int {
	return coalescePositive(c.Max, DefaultIntegrationInboundRateLimitMax)
}

// EffectivePeriod returns Period or its default.
func (c *IntegrationInboundRateLimitConfig) EffectivePeriod() time.Duration {
	return coalescePositive(c.Period, DefaultIntegrationInboundRateLimitPeriod)
}

// EffectiveSecretAlgorithm returns SecretAlgorithm or the AES default.
func (c *IntegrationConfig) EffectiveSecretAlgorithm() IntegrationSecretAlgorithm {
	if c.SecretAlgorithm == "" {
		return IntegrationSecretAlgorithmAES
	}

	return c.SecretAlgorithm
}

// EffectiveRunTimeout returns RunTimeout or its default.
func (c *IntegrationConfig) EffectiveRunTimeout() time.Duration {
	return coalescePositive(c.RunTimeout, 30*time.Second)
}

// EffectiveMaxResponseBody returns MaxResponseBody or its default.
func (c *IntegrationConfig) EffectiveMaxResponseBody() int64 {
	return coalescePositive(c.MaxResponseBody, 8<<20)
}

// ErrInvalidIntegrationLogRetention indicates a negative retention window.
var ErrInvalidIntegrationLogRetention = errors.New("invalid integration log retention")

// Validate rejects unsupported log modes, unsupported secret algorithms, and
// negative retention windows so configuration typos fail at startup instead
// of silently recording nothing or deleting everything.
func (c *IntegrationConfig) Validate() error {
	switch c.Log.EffectiveMode() {
	case IntegrationLogOff, IntegrationLogErrors, IntegrationLogAll:
	default:
		return fmt.Errorf("%w %q (want %q, %q, or %q)", ErrInvalidIntegrationLogMode,
			c.Log.Mode, IntegrationLogOff, IntegrationLogErrors, IntegrationLogAll)
	}

	switch c.EffectiveSecretAlgorithm() {
	case IntegrationSecretAlgorithmAES, IntegrationSecretAlgorithmSM4:
	default:
		return fmt.Errorf("%w %q (want %q or %q)", ErrInvalidIntegrationSecretAlgorithm,
			c.SecretAlgorithm, IntegrationSecretAlgorithmAES, IntegrationSecretAlgorithmSM4)
	}

	if c.Log.Retention < 0 {
		return fmt.Errorf("%w: must not be negative", ErrInvalidIntegrationLogRetention)
	}

	return nil
}
