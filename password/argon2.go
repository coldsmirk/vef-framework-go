package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2SaltLength = 16
	argon2KeyLength  = 32
)

type argon2Encoder struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

// Argon2Option configures argon2Encoder.
type Argon2Option func(*argon2Encoder)

// WithArgon2Memory sets the memory parameter in KiB.
func WithArgon2Memory(memory uint32) Argon2Option {
	return func(e *argon2Encoder) {
		e.memory = memory
	}
}

// WithArgon2Iterations sets the number of iterations.
func WithArgon2Iterations(iterations uint32) Argon2Option {
	return func(e *argon2Encoder) {
		e.iterations = iterations
	}
}

// WithArgon2Parallelism sets the parallelism factor.
func WithArgon2Parallelism(parallelism uint8) Argon2Option {
	return func(e *argon2Encoder) {
		e.parallelism = parallelism
	}
}

// NewArgon2Encoder creates a new Argon2id-based password encoder.
// Defaults: memory=64MB, iterations=3, parallelism=4 (OWASP recommendations for 2024).
func NewArgon2Encoder(opts ...Argon2Option) Encoder {
	encoder := &argon2Encoder{
		memory:      64 * 1024,
		iterations:  3,
		parallelism: 4,
	}

	for _, opt := range opts {
		opt(encoder)
	}

	return encoder
}

func (e *argon2Encoder) Encode(password string) (string, error) {
	if e.memory < 8 {
		return "", ErrInvalidMemory
	}

	if e.iterations < 1 {
		return "", ErrInvalidIterations
	}

	if e.parallelism < 1 {
		return "", ErrInvalidParallelism
	}

	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, e.iterations, e.memory, e.parallelism, argon2KeyLength)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, e.memory, e.iterations, e.parallelism, encodedSalt, encodedHash), nil
}

func (e *argon2Encoder) Matches(password, encodedPassword string) bool {
	params, salt, hash, err := e.decodeHash(encodedPassword)
	if err != nil {
		return false
	}

	computedHash := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, uint32(len(hash)))

	return subtle.ConstantTimeCompare(hash, computedHash) == 1
}

func (e *argon2Encoder) UpgradeEncoding(encodedPassword string) bool {
	params, _, _, err := e.decodeHash(encodedPassword)
	if err != nil {
		return false
	}

	return params.memory < e.memory || params.iterations < e.iterations || params.parallelism < e.parallelism
}

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func (*argon2Encoder) decodeHash(encodedPassword string) (params *argon2Params, salt, hash []byte, err error) {
	parts := strings.Split(encodedPassword, "$")
	if len(parts) != 6 {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	if parts[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	if version != argon2.Version {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	params = new(argon2Params)
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.iterations, &params.parallelism); err != nil {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	return params, salt, hash, nil
}
