package password

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"hash"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pbkdf2SaltLength = 16
	pbkdf2KeyLength  = 32
)

type pbkdf2Encoder struct {
	iterations   int
	hashFunction string
}

// Pbkdf2Option configures pbkdf2Encoder.
type Pbkdf2Option func(*pbkdf2Encoder)

// WithPbkdf2Iterations sets the number of iterations.
func WithPbkdf2Iterations(iterations int) Pbkdf2Option {
	return func(e *pbkdf2Encoder) {
		e.iterations = iterations
	}
}

// WithPbkdf2HashFunction sets the hash function ("sha256" or "sha512").
func WithPbkdf2HashFunction(hashFunction string) Pbkdf2Option {
	return func(e *pbkdf2Encoder) {
		e.hashFunction = hashFunction
	}
}

// NewPbkdf2Encoder creates a new PBKDF2-based password encoder.
// Defaults: 310,000 iterations with SHA-256 (OWASP 2023 recommendations).
func NewPbkdf2Encoder(opts ...Pbkdf2Option) Encoder {
	encoder := &pbkdf2Encoder{
		iterations:   310000,
		hashFunction: "sha256",
	}

	for _, opt := range opts {
		opt(encoder)
	}

	return encoder
}

func (e *pbkdf2Encoder) Encode(password string) (string, error) {
	if e.iterations < 1 {
		return "", ErrInvalidIterations
	}

	hashFunc := e.getHashFunc()
	if hashFunc == nil {
		return "", ErrInvalidHashFormat
	}

	salt := make([]byte, pbkdf2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := pbkdf2.Key([]byte(password), salt, e.iterations, pbkdf2KeyLength, hashFunc)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$pbkdf2-%s$i=%d$%s$%s",
		e.hashFunction, e.iterations, encodedSalt, encodedHash), nil
}

func (e *pbkdf2Encoder) Matches(password, encodedPassword string) bool {
	params, salt, hash, err := e.decodeHash(encodedPassword)
	if err != nil {
		return false
	}

	hashFunc := params.getHashFunc()
	if hashFunc == nil {
		return false
	}

	computedHash := pbkdf2.Key([]byte(password), salt, params.iterations, len(hash), hashFunc)

	return subtle.ConstantTimeCompare(hash, computedHash) == 1
}

func (e *pbkdf2Encoder) UpgradeEncoding(encodedPassword string) bool {
	params, _, _, err := e.decodeHash(encodedPassword)
	if err != nil {
		return false
	}

	return params.iterations < e.iterations || params.hashFunction != e.hashFunction
}

type pbkdf2Params struct {
	iterations   int
	hashFunction string
}

func (e *pbkdf2Encoder) getHashFunc() func() hash.Hash {
	return getHashFuncByName(e.hashFunction)
}

func (p *pbkdf2Params) getHashFunc() func() hash.Hash {
	return getHashFuncByName(p.hashFunction)
}

func getHashFuncByName(name string) func() hash.Hash {
	switch name {
	case "sha256":
		return sha256.New
	case "sha512":
		return sha512.New
	default:
		return nil
	}
}

func (*pbkdf2Encoder) decodeHash(encodedPassword string) (params *pbkdf2Params, salt, hash []byte, err error) {
	parts := strings.Split(encodedPassword, "$")
	if len(parts) != 5 {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	var hashFunction string
	if after, found := strings.CutPrefix(parts[1], "pbkdf2-"); found {
		hashFunction = after
	} else {
		return nil, nil, nil, ErrInvalidHashFormat
	}

	params = &pbkdf2Params{
		hashFunction: hashFunction,
	}

	if _, err = fmt.Sscanf(parts[2], "i=%d", &params.iterations); err != nil {
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
