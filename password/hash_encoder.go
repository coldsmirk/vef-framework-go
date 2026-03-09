package password

import (
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
	if strings.HasPrefix(encodedPassword, prefix) {
		parts := strings.Split(encodedPassword, "$")
		if len(parts) != 3 {
			return false
		}

		salt := parts[1]
		expectedHash := parts[2]
		input := e.prepareInput(password, salt)
		actualHash := e.hashFn([]byte(input))

		return actualHash == expectedHash
	}

	actualHash := e.hashFn([]byte(password))

	return actualHash == encodedPassword
}

func (*hashEncoder) UpgradeEncoding(_ string) bool {
	return true
}
