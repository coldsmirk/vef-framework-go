package password

import (
	"crypto/subtle"
	"fmt"
	"strings"
)

const saltPositionPrefix = "prefix"

// hashFunc represents a hash function that returns a hex-encoded hash string.
type hashFunc func(input []byte) string

// hashEncoder provides common functionality for simple hash-based password encoders.
type hashEncoder struct {
	salt         string
	saltPosition string
	algorithm    string
	hashFn       hashFunc
}

func (e *hashEncoder) prepareInput(password, salt string) string {
	if salt == "" {
		return password
	}

	if e.saltPosition == saltPositionPrefix {
		return salt + password
	}

	return password + salt
}

func (e *hashEncoder) Encode(password string) (string, error) {
	input := e.prepareInput(password, e.salt)
	hexHash := e.hashFn([]byte(input))

	if e.salt != "" {
		return fmt.Sprintf("{%s}$%s$%s", e.algorithm, e.salt, hexHash), nil
	}

	return hexHash, nil
}

func (e *hashEncoder) Matches(password, encodedPassword string) bool {
	prefix := "{" + e.algorithm + "}$"
	if rest, ok := strings.CutPrefix(encodedPassword, prefix); ok {
		// Stored form is {algo}$salt$hash. Split on the LAST '$' so a salt that
		// itself contains '$' cannot corrupt the parse (hex hashes never do).
		sep := strings.LastIndex(rest, "$")
		if sep < 0 {
			return false
		}

		salt, expectedHash := rest[:sep], rest[sep+1:]
		actualHash := e.hashFn([]byte(e.prepareInput(password, salt)))

		return constantTimeEqual(actualHash, expectedHash)
	}

	// No prefix: still apply the configured salt so a salted encoder never
	// accepts a bare unsalted hash of the password.
	actualHash := e.hashFn([]byte(e.prepareInput(password, e.salt)))

	return constantTimeEqual(actualHash, encodedPassword)
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (*hashEncoder) UpgradeEncoding(string) bool {
	return true
}
