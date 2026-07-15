package jscrypto

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/js"
)

// Name is the library identifier and the global binding installed into the
// runtime.
const Name = "crypto"

// lib exposes hashing and encoding helpers as the global "crypto" object —
// the toolbox scripts need to sign and decode external API traffic:
//
//	crypto.md5(data)                    // likewise sha1 / sha256 / sha512 / sm3
//	crypto.hmac('sha256', key, data)    // hex digest
//	crypto.base64Encode(data) / crypto.base64Decode(encoded)
//	crypto.hexEncode(data) / crypto.hexDecode(encoded)
//	crypto.uuid()
//
// Digests are lower-case hex strings; inputs are UTF-8 strings. The weak
// digests (md5, sha1) exist for legacy API signature interop — password
// storage belongs to the security module's encoders.
type lib struct{}

// New builds the crypto library. It is pure computation and needs no
// dependencies or policy.
func New() js.Lib {
	return new(lib)
}

func (*lib) Name() string {
	return Name
}

func (*lib) Install(rt *js.Runtime) error {
	return rt.Set(Name, map[string]any{
		"md5":    hashx.MD5,
		"sha1":   hashx.SHA1,
		"sha256": hashx.SHA256,
		"sha512": hashx.SHA512,
		"sm3":    hashx.SM3,
		"hmac":   hmac,
		"base64Encode": func(data string) string {
			return base64.StdEncoding.EncodeToString([]byte(data))
		},
		"base64Decode": func(encoded string) (string, error) {
			data, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return "", err
			}

			return string(data), nil
		},
		"hexEncode": func(data string) string {
			return hex.EncodeToString([]byte(data))
		},
		"hexDecode": func(encoded string) (string, error) {
			data, err := hex.DecodeString(encoded)
			if err != nil {
				return "", err
			}

			return string(data), nil
		},
		"uuid": uuid.NewString,
	})
}

// hmac computes the keyed digest for the given algorithm as lower-case hex.
func hmac(algorithm, key, data string) (string, error) {
	keyBytes, dataBytes := []byte(key), []byte(data)

	switch strings.ToLower(algorithm) {
	case "md5":
		return hashx.HmacMD5(keyBytes, dataBytes), nil
	case "sha1":
		return hashx.HmacSHA1(keyBytes, dataBytes), nil
	case "sha256":
		return hashx.HmacSHA256(keyBytes, dataBytes), nil
	case "sha512":
		return hashx.HmacSHA512(keyBytes, dataBytes), nil
	case "sm3":
		return hashx.HmacSM3(keyBytes, dataBytes), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedAlgorithm, algorithm)
	}
}
