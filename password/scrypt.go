package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"
)

const (
	scryptSaltLength = 16
	scryptKeyLength  = 32
)

type scryptEncoder struct {
	n int
	r int
	p int
}

// ScryptOption configures scryptEncoder.
type ScryptOption func(*scryptEncoder)

// WithScryptN sets the CPU/memory cost parameter (must be a power of 2).
func WithScryptN(n int) ScryptOption {
	return func(e *scryptEncoder) {
		e.n = n
	}
}

// WithScryptR sets the block size parameter.
func WithScryptR(r int) ScryptOption {
	return func(e *scryptEncoder) {
		e.r = r
	}
}

// WithScryptP sets the parallelization parameter.
func WithScryptP(p int) ScryptOption {
	return func(e *scryptEncoder) {
		e.p = p
	}
}

// NewScryptEncoder creates a new scrypt-based password encoder.
// Defaults: N=32768 (2^15), r=8, p=1 (OWASP recommendations for interactive logins).
func NewScryptEncoder(opts ...ScryptOption) Encoder {
	encoder := &scryptEncoder{
		n: 32768,
		r: 8,
		p: 1,
	}

	for _, opt := range opts {
		opt(encoder)
	}

	return encoder
}

func (e *scryptEncoder) Encode(password string) (string, error) {
	if e.n < 2 {
		return "", ErrInvalidIterations
	}

	if e.r < 1 {
		return "", ErrInvalidIterations
	}

	if e.p < 1 {
		return "", ErrInvalidParallelism
	}

	salt := make([]byte, scryptSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash, err := scrypt.Key([]byte(password), salt, e.n, e.r, e.p, scryptKeyLength)
	if err != nil {
		return "", err
	}

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$scrypt$n=%d,r=%d,p=%d$%s$%s",
		e.n, e.r, e.p, encodedSalt, encodedHash), nil
}

func (e *scryptEncoder) Matches(password, encodedPassword string) bool {
	params, salt, hash, err := e.decodeHash(encodedPassword)
	if err != nil {
		return false
	}

	computedHash, err := scrypt.Key([]byte(password), salt, params.n, params.r, params.p, len(hash))
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(hash, computedHash) == 1
}

func (e *scryptEncoder) UpgradeEncoding(encodedPassword string) bool {
	params, _, _, err := e.decodeHash(encodedPassword)
	if err != nil {
		return false
	}

	return params.n < e.n || params.r < e.r || params.p < e.p
}

type scryptParams struct {
	n int
	r int
	p int
}

func (*scryptEncoder) decodeHash(encodedPassword string) (params *scryptParams, salt, hash []byte, err error) {
	parts := strings.Split(encodedPassword, "$")
	if len(parts) != 5 {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	if parts[1] != "scrypt" {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	params = new(scryptParams)
	if _, err = fmt.Sscanf(parts[2], "n=%d,r=%d,p=%d", &params.n, &params.r, &params.p); err != nil {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	return params, salt, hash, nil
}
