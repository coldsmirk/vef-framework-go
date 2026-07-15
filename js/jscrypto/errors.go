package jscrypto

import "errors"

// ErrUnsupportedAlgorithm is thrown into the script when crypto.hmac receives
// an algorithm outside md5 / sha1 / sha256 / sha512 / sm3.
var ErrUnsupportedAlgorithm = errors.New("jscrypto: unsupported algorithm")
