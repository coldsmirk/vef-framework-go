package cryptox

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tjfoc/gmsm/sm4"
)

// TestSm4CipherCbc tests SM4 encryption and decryption round-trip in CBC mode.
func TestSm4CipherCbc(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key)
	require.NoError(t, err, "Should create SM4 cipher in CBC mode")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "SM4-CBC encryption test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
		{"ChineseAlgorithm", "国密SM4加密算法"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := cipher.Encrypt(tt.plaintext)
			require.NoError(t, err, "Should encrypt plaintext successfully")

			decrypted, err := cipher.Decrypt(ciphertext)
			require.NoError(t, err, "Should decrypt ciphertext successfully")

			assert.Equal(t, tt.plaintext, decrypted, "Decrypted text should match original plaintext")
		})
	}
}

// TestSm4CipherCbcRandomIv asserts CBC encryption uses a fresh random IV per
// call: encrypting the same plaintext twice must yield different ciphertext,
// and both must still decrypt back to the original.
func TestSm4CipherCbcRandomIv(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key)
	require.NoError(t, err, "Should create SM4 cipher")

	plaintext := "Test message"

	ciphertext1, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	ciphertext2, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	assert.NotEqual(t, ciphertext1, ciphertext2,
		"CBC encryption of the same plaintext must differ due to a fresh random IV")

	decrypted1, err := cipher.Decrypt(ciphertext1)
	require.NoError(t, err, "Should decrypt first ciphertext successfully")
	decrypted2, err := cipher.Decrypt(ciphertext2)
	require.NoError(t, err, "Should decrypt second ciphertext successfully")

	assert.Equal(t, plaintext, decrypted1, "First decrypted text should match original plaintext")
	assert.Equal(t, plaintext, decrypted2, "Second decrypted text should match original plaintext")
}

// TestSm4CipherDecryptWithFixedIv covers the interop decrypt path: data that an
// external client produced with a constant IV (no prepended IV) is decrypted
// using the fixed IV configured via WithSM4Iv.
func TestSm4CipherDecryptWithFixedIv(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	iv := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	plaintext := "client-encrypted password"
	external := sm4FixedIvCiphertext(t, key, iv, plaintext)

	c, err := NewSM4(key, WithSM4Iv(iv))
	require.NoError(t, err, "Should create SM4 cipher with fixed interop IV")

	decrypter, ok := c.(FixedIVDecrypter)
	require.True(t, ok, "SM4 cipher should implement FixedIVDecrypter")

	decrypted, err := decrypter.DecryptWithFixedIV(external)
	require.NoError(t, err, "Should decrypt fixed-IV interop ciphertext")
	assert.Equal(t, plaintext, decrypted, "Decrypted text should match client plaintext")
}

// TestSm4CipherDecryptWithFixedIvRequiresIv verifies the interop path fails
// cleanly when no fixed IV was configured.
func TestSm4CipherDecryptWithFixedIvRequiresIv(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	c, err := NewSM4(key)
	require.NoError(t, err, "Should create SM4 cipher")

	decrypter, ok := c.(FixedIVDecrypter)
	require.True(t, ok, "SM4 cipher should implement FixedIVDecrypter")

	_, err = decrypter.DecryptWithFixedIV(base64.StdEncoding.EncodeToString(make([]byte, sm4.BlockSize)))
	require.ErrorIs(t, err, ErrInvalidIVSizeCBC, "Should reject interop decrypt without a configured IV")
}

// TestSm4CipherFromHex tests creating SM4 cipher from hex-encoded key.
func TestSm4CipherFromHex(t *testing.T) {
	keyHex := "0123456789abcdef0123456789abcdef"

	cipher, err := NewSM4FromHex(keyHex)
	require.NoError(t, err, "Should create SM4 cipher from hex")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestSm4CipherFromBase64 tests creating SM4 cipher from base64-encoded key.
func TestSm4CipherFromBase64(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	keyBase64 := base64.StdEncoding.EncodeToString(key)

	cipher, err := NewSM4FromBase64(keyBase64)
	require.NoError(t, err, "Should create SM4 cipher from base64")

	plaintext := "Test message with base64 encoded key"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestSm4CipherInvalidKeySize tests that invalid key size is rejected.
func TestSm4CipherInvalidKeySize(t *testing.T) {
	invalidKey := make([]byte, 8)

	_, err := NewSM4(invalidKey)
	assert.Error(t, err, "Should reject invalid key size")
}

// TestSm4CipherInvalidIvSize tests that an invalid fixed interop IV is rejected.
func TestSm4CipherInvalidIvSize(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	invalidIV := make([]byte, 8)

	_, err := NewSM4(key, WithSM4Iv(invalidIV))
	assert.Error(t, err, "Should reject invalid fixed IV size")
}

// TestSm4CipherDecryptShortCiphertext verifies short/invalid ciphertext is
// rejected cleanly rather than panicking.
func TestSm4CipherDecryptShortCiphertext(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key)
	require.NoError(t, err, "Should create SM4 cipher")

	tests := []struct {
		name       string
		ciphertext string
	}{
		{"NotBase64", "not valid base64 !!!"},
		{"ShorterThanIv", base64.StdEncoding.EncodeToString(make([]byte, sm4.BlockSize-1))},
		{"IvOnlyNoPayload", base64.StdEncoding.EncodeToString(make([]byte, sm4.BlockSize))},
		{"PayloadNotBlockMultiple", base64.StdEncoding.EncodeToString(make([]byte, sm4.BlockSize+1))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cipher.Decrypt(tt.ciphertext)
			require.Error(t, err, "Should reject malformed ciphertext")
		})
	}
}

// TestSm4CipherLongMessage tests SM4 with long messages spanning multiple blocks.
func TestSm4CipherLongMessage(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key)
	require.NoError(t, err, "Should create SM4 cipher")

	plaintext := strings.Repeat("This is a test message. ", 100)

	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestSm4CipherEmptyString tests SM4 with empty string input.
func TestSm4CipherEmptyString(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key)
	require.NoError(t, err, "Should create SM4 cipher")

	plaintext := ""
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt empty string successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match empty plaintext")
}

// TestSm4CipherDefaultMode tests that the default (and only) mode is CBC with a
// prepended random IV.
func TestSm4CipherDefaultMode(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key)
	require.NoError(t, err, "Should create SM4 cipher with default mode")

	plaintext := "Test default mode"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// sm4FixedIvCiphertext emulates an external client encrypting with a constant
// IV and no prepended IV, mirroring the interop input DecryptWithFixedIV reads.
func sm4FixedIvCiphertext(t *testing.T, key, iv []byte, plaintext string) string {
	t.Helper()

	block, err := sm4.NewCipher(key)
	require.NoError(t, err, "Should create SM4 block cipher")

	padded := pkcs7Padding([]byte(plaintext), sm4.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)

	return base64.StdEncoding.EncodeToString(out)
}
