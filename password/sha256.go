package password

import (
	"crypto/sha256"
	"encoding/hex"
)

// Sha256Option configures SHA-256 encoder.
type Sha256Option func(*hashEncoder)

// WithSha256Salt sets a static salt value.
// WARNING: Static salts provide minimal security. Use modern algorithms like Argon2 for new systems.
func WithSha256Salt(salt string) Sha256Option {
	return func(e *hashEncoder) {
		e.salt = salt
	}
}

// WithSha256SaltPosition sets where the salt is placed ("prefix" or "suffix").
func WithSha256SaltPosition(position string) Sha256Option {
	return func(e *hashEncoder) {
		e.saltPosition = position
	}
}

// NewSha256Encoder creates a new SHA-256-based password encoder.
// WARNING: Use only for legacy system compatibility.
func NewSha256Encoder(opts ...Sha256Option) Encoder {
	encoder := &hashEncoder{
		saltPosition: "suffix",
		algorithm:    "sha256",
		hashFn: func(input []byte) string {
			hash := sha256.Sum256(input)

			return hex.EncodeToString(hash[:])
		},
	}

	for _, opt := range opts {
		opt(encoder)
	}

	return encoder
}
