package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// Default API configuration values, applied by the Effective* accessors.
const (
	DefaultAPIRateLimitMax    = 100
	DefaultAPIRateLimitPeriod = 5 * time.Minute
	// DefaultAPIBodyEncoding is used when protected body transport is enabled
	// without an explicit encoding.
	DefaultAPIBodyEncoding APIBodyEncoding = APIBodyEncodingAESGCMBase64
)

// APIBodyEncoding selects the bidirectional protected-body wire format.
type APIBodyEncoding string

const (
	// APIBodyEncodingAESGCMBase64 uses AES-GCM with a random 12-byte nonce and
	// transports Base64(nonce || ciphertext || tag).
	APIBodyEncodingAESGCMBase64 APIBodyEncoding = "aes-gcm+base64"
	// APIBodyEncodingSM4GCMBase64 uses SM4-GCM with the same wire layout.
	APIBodyEncodingSM4GCMBase64 APIBodyEncoding = "sm4-gcm+base64"
)

var (
	// ErrInvalidAPIBodyEncoding indicates an unsupported protected-body wire format.
	ErrInvalidAPIBodyEncoding = errors.New("invalid API body encoding")
	// ErrMissingAPIBodyEncodingKey indicates that protected-body transport was
	// enabled without a key.
	ErrMissingAPIBodyEncodingKey = errors.New("missing API body encoding key")
	// ErrInvalidAPIBodyEncodingKey indicates malformed base64 or an invalid key size.
	ErrInvalidAPIBodyEncodingKey = errors.New("invalid API body encoding key")
)

// APIConfig configures cross-cutting behavior of the API engine
// (`vef.api`). Zero values resolve to defaults through the Effective*
// accessors.
type APIConfig struct {
	// BodyEncoding protects JSON request and response bodies on the /api
	// surface. Multipart and binary bodies keep their native wire formats.
	BodyEncoding APIBodyEncodingConfig `config:"body_encoding"`
	// RateLimit is the default rate limit applied to every operation that does
	// not declare its own via api.OperationSpec.RateLimit.
	RateLimit APIRateLimitConfig `config:"rate_limit"`
}

// APIBodyEncodingConfig controls bidirectional protected JSON body transport.
// Key is standard base64 containing an AES key (16, 24, or 32 bytes) or an SM4
// key (16 bytes). HTTPS remains required: a browser-held key is extractable and
// this layer is intended to keep bodies non-plaintext on the wire, not replace
// transport security.
type APIBodyEncodingConfig struct {
	// Enabled requires protected JSON bodies on the /api surface.
	Enabled bool `config:"enabled"`
	// Encoding selects AES-GCM or SM4-GCM. Default: aes-gcm+base64.
	Encoding APIBodyEncoding `config:"encoding"`
	// Key is the standard-base64 symmetric key shared with the client.
	Key string `config:"key"`
}

// EffectiveEncoding returns Encoding or the AES-GCM default.
func (c *APIBodyEncodingConfig) EffectiveEncoding() APIBodyEncoding {
	if c.Encoding == "" {
		return DefaultAPIBodyEncoding
	}

	return c.Encoding
}

// Validate rejects a protected-body configuration that cannot be initialized.
func (c *APIBodyEncodingConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	encoding := c.EffectiveEncoding()
	if encoding != APIBodyEncodingAESGCMBase64 && encoding != APIBodyEncodingSM4GCMBase64 {
		return fmt.Errorf("%w %q (want %q or %q)", ErrInvalidAPIBodyEncoding,
			c.Encoding, APIBodyEncodingAESGCMBase64, APIBodyEncodingSM4GCMBase64)
	}

	if c.Key == "" {
		return ErrMissingAPIBodyEncodingKey
	}

	key, err := base64.StdEncoding.DecodeString(c.Key)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAPIBodyEncodingKey, err)
	}

	validSize := len(key) == 16
	if encoding == APIBodyEncodingAESGCMBase64 {
		validSize = validSize || len(key) == 24 || len(key) == 32
	}

	if !validSize {
		if encoding == APIBodyEncodingSM4GCMBase64 {
			return fmt.Errorf("%w: %s requires a 16-byte key", ErrInvalidAPIBodyEncodingKey, encoding)
		}

		return fmt.Errorf("%w: %s requires a 16-, 24-, or 32-byte key", ErrInvalidAPIBodyEncodingKey, encoding)
	}

	return nil
}

// Validate rejects invalid API configuration before middleware construction.
func (c *APIConfig) Validate() error {
	return c.BodyEncoding.Validate()
}

// APIRateLimitConfig bounds request throughput. The limiter counts per
// operation and client (resource:version:action + IP + principal) in a sliding
// window held in process memory, so in a multi-node deployment each node
// enforces the limit independently.
type APIRateLimitConfig struct {
	// Max is the number of requests admitted per window; 0 resolves to the
	// default. Default: 100.
	Max int `config:"max"`
	// Period is the sliding-window length; 0 resolves to the default.
	// Default: 5m.
	Period time.Duration `config:"period"`
}

// EffectiveMax returns Max or its default.
func (c *APIRateLimitConfig) EffectiveMax() int {
	return coalescePositive(c.Max, DefaultAPIRateLimitMax)
}

// EffectivePeriod returns Period or its default.
func (c *APIRateLimitConfig) EffectivePeriod() time.Duration {
	return coalescePositive(c.Period, DefaultAPIRateLimitPeriod)
}
