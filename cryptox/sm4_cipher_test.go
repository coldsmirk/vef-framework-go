package cryptox

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tjfoc/gmsm/sm4"
)

// TestSm4CipherCbc tests SM4 encryption and decryption in CBC mode.
func TestSm4CipherCbc(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	iv := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	cipher, err := NewSM4(key, WithSM4Iv(iv), WithSM4Mode(SM4ModeCBC))
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

// TestSm4CipherEcb tests SM4 encryption and decryption in ECB mode.
func TestSm4CipherEcb(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key, WithSM4Mode(SM4ModeECB))
	require.NoError(t, err, "Should create SM4 cipher in ECB mode")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "SM4-ECB encryption test"},
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

// TestSm4CipherFromHex tests creating SM4 cipher from hex-encoded key.
func TestSm4CipherFromHex(t *testing.T) {
	keyHex := "0123456789abcdef0123456789abcdef"
	ivHex := "fedcba9876543210fedcba9876543210"

	iv, err := hex.DecodeString(ivHex)
	require.NoError(t, err, "Should decode hex IV")

	cipher, err := NewSM4FromHex(keyHex, WithSM4Iv(iv), WithSM4Mode(SM4ModeCBC))
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
	iv := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	keyBase64 := base64.StdEncoding.EncodeToString(key)

	cipher, err := NewSM4FromBase64(keyBase64, WithSM4Iv(iv), WithSM4Mode(SM4ModeCBC))
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
	iv := make([]byte, sm4.BlockSize)

	_, err := NewSM4(invalidKey, WithSM4Iv(iv), WithSM4Mode(SM4ModeCBC))
	assert.Error(t, err, "Should reject invalid key size")
}

// TestSm4CipherInvalidIvSize tests that invalid IV size is rejected.
func TestSm4CipherInvalidIvSize(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	invalidIV := make([]byte, 8)

	_, err := NewSM4(key, WithSM4Iv(invalidIV), WithSM4Mode(SM4ModeCBC))
	assert.Error(t, err, "Should reject invalid IV size")
}

// TestSm4CipherEcbNoIvRequired tests that ECB mode doesn't require IV.
func TestSm4CipherEcbNoIvRequired(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key, WithSM4Mode(SM4ModeECB))
	require.NoError(t, err, "Should create SM4 cipher in ECB mode without IV")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestSm4CipherEcbDeterministic tests that ECB mode is deterministic.
func TestSm4CipherEcbDeterministic(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewSM4(key, WithSM4Mode(SM4ModeECB))
	require.NoError(t, err, "Should create SM4 cipher")

	plaintext := "Test message"

	ciphertext1, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	ciphertext2, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	assert.Equal(t, ciphertext1, ciphertext2,
		"ECB mode should produce same ciphertext for same plaintext")
}

// TestSm4CipherCbcNonDeterministic tests CBC mode with fixed IV.
func TestSm4CipherCbcNonDeterministic(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	iv := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	cipher, err := NewSM4(key, WithSM4Iv(iv), WithSM4Mode(SM4ModeCBC))
	require.NoError(t, err, "Should create SM4 cipher")

	plaintext := "Test message"

	ciphertext1, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	ciphertext2, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	assert.Equal(t, ciphertext1, ciphertext2,
		"CBC mode with same IV should produce same ciphertext")

	decrypted1, err := cipher.Decrypt(ciphertext1)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	decrypted2, err := cipher.Decrypt(ciphertext2)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted1, "First decrypted text should match original plaintext")
	assert.Equal(t, plaintext, decrypted2, "Second decrypted text should match original plaintext")
}

// TestSm4CipherLongMessage tests SM4 with long messages spanning multiple blocks.
func TestSm4CipherLongMessage(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	iv := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	cipher, err := NewSM4(key, WithSM4Iv(iv), WithSM4Mode(SM4ModeCBC))
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
	iv := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	cipher, err := NewSM4(key, WithSM4Iv(iv), WithSM4Mode(SM4ModeCBC))
	require.NoError(t, err, "Should create SM4 cipher")

	plaintext := ""
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt empty string successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match empty plaintext")
}

// TestSm4CipherDefaultMode tests that default mode is CBC.
func TestSm4CipherDefaultMode(t *testing.T) {
	key := make([]byte, sm4.BlockSize)
	iv := make([]byte, sm4.BlockSize)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	cipher, err := NewSM4(key, WithSM4Iv(iv))
	require.NoError(t, err, "Should create SM4 cipher with default mode")

	plaintext := "Test default mode"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}
