package password

import (
	"crypto/md5"
	"encoding/hex"
)

// Md5Option configures MD5 encoder.
type Md5Option func(*hashEncoder)

// WithMd5Salt sets a static salt value.
// WARNING: Static salts provide minimal security. Use modern algorithms like Argon2 for new systems.
func WithMd5Salt(salt string) Md5Option {
	return func(e *hashEncoder) {
		e.salt = salt
	}
}

// WithMd5SaltPosition sets where the salt is placed ("prefix" or "suffix").
func WithMd5SaltPosition(position string) Md5Option {
	return func(e *hashEncoder) {
		e.saltPosition = position
	}
}

// NewMd5Encoder creates a new MD5-based password encoder.
// WARNING: Use only for legacy system compatibility.
func NewMd5Encoder(opts ...Md5Option) Encoder {
	encoder := &hashEncoder{
		saltPosition: "suffix",
		algorithm:    "md5",
		hashFn: func(input []byte) string {
			hash := md5.Sum(input)

			return hex.EncodeToString(hash[:])
		},
	}

	for _, opt := range opts {
		opt(encoder)
	}

	return encoder
}
