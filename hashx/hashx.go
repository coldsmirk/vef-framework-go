package hashx

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"

	"github.com/tjfoc/gmsm/sm3"
)

// MD5 computes the MD5 hash of the input string and returns the hex-encoded result.
func MD5(data string) string {
	return MD5Bytes([]byte(data))
}

// MD5Bytes computes the MD5 hash of the input bytes and returns the hex-encoded result.
func MD5Bytes(data []byte) string {
	sum := md5.Sum(data)

	return hex.EncodeToString(sum[:])
}

// SHA1 computes the SHA-1 hash of the input string and returns the hex-encoded result.
func SHA1(data string) string {
	return SHA1Bytes([]byte(data))
}

// SHA1Bytes computes the SHA-1 hash of the input bytes and returns the hex-encoded result.
func SHA1Bytes(data []byte) string {
	sum := sha1.Sum(data)

	return hex.EncodeToString(sum[:])
}

// SHA256 computes the SHA-256 hash of the input string and returns the hex-encoded result.
func SHA256(data string) string {
	return SHA256Bytes([]byte(data))
}

// SHA256Bytes computes the SHA-256 hash of the input bytes and returns the hex-encoded result.
func SHA256Bytes(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// SHA512 computes the SHA-512 hash of the input string and returns the hex-encoded result.
func SHA512(data string) string {
	return SHA512Bytes([]byte(data))
}

// SHA512Bytes computes the SHA-512 hash of the input bytes and returns the hex-encoded result.
func SHA512Bytes(data []byte) string {
	sum := sha512.Sum512(data)

	return hex.EncodeToString(sum[:])
}

// SM3 computes the SM3 hash (Chinese National Standard) of the input string
// and returns the hex-encoded result.
func SM3(data string) string {
	return SM3Bytes([]byte(data))
}

// SM3Bytes computes the SM3 hash (Chinese National Standard) of the input bytes
// and returns the hex-encoded result.
func SM3Bytes(data []byte) string {
	sum := sm3.Sm3Sum(data)

	return hex.EncodeToString(sum)
}

// HmacMD5 computes the HMAC-MD5 of the data using the provided key.
func HmacMD5(key, data []byte) string {
	mac := hmac.New(md5.New, key)
	_, _ = mac.Write(data)

	return hex.EncodeToString(mac.Sum(nil))
}

// HmacSHA1 computes the HMAC-SHA1 of the data using the provided key.
func HmacSHA1(key, data []byte) string {
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(data)

	return hex.EncodeToString(mac.Sum(nil))
}

// HmacSHA256 computes the HMAC-SHA256 of the data using the provided key.
func HmacSHA256(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)

	return hex.EncodeToString(mac.Sum(nil))
}

// HmacSHA512 computes the HMAC-SHA512 of the data using the provided key.
func HmacSHA512(key, data []byte) string {
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write(data)

	return hex.EncodeToString(mac.Sum(nil))
}

// HmacSM3 computes the HMAC-SM3 (Chinese National Standard) of the data using the provided key.
func HmacSM3(key, data []byte) string {
	mac := hmac.New(sm3.New, key)
	_, _ = mac.Write(data)

	return hex.EncodeToString(mac.Sum(nil))
}
