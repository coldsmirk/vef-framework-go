package cryptox

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAesCipherCbc tests AES encryption and decryption in CBC mode.
func TestAesCipherCbc(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 16)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	cipher, err := NewAES(key, WithAESIv(iv), WithAESMode(AesModeCbc))
	require.NoError(t, err, "Should create AES cipher in CBC mode")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"LongerMessage", "This is a test message"},
		{"WithDescription", "AES-256-CBC encryption test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
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

// TestAesCipherGcm tests AES encryption and decryption in GCM mode.
func TestAesCipherGcm(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewAES(key, WithAESMode(AesModeGcm))
	require.NoError(t, err, "Should create AES cipher in GCM mode")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"LongerMessage", "This is a test message"},
		{"WithDescription", "AES-256-GCM encryption test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
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

// TestAesCipherFromHex tests creating AES cipher from hex-encoded key.
func TestAesCipherFromHex(t *testing.T) {
	keyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	iv := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}

	cipher, err := NewAESFromHex(keyHex, WithAESIv(iv), WithAESMode(AesModeCbc))
	require.NoError(t, err, "Should create AES cipher from hex")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestAesCipherFromBase64 tests creating AES cipher from base64-encoded key.
func TestAesCipherFromBase64(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 16)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")
	_, err = rand.Read(iv)
	require.NoError(t, err, "Should generate random IV")

	keyBase64 := base64.StdEncoding.EncodeToString(key)

	cipher, err := NewAESFromBase64(keyBase64, WithAESIv(iv), WithAESMode(AesModeCbc))
	require.NoError(t, err, "Should create AES cipher from base64")

	plaintext := "Test message with base64 encoded key"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestAesCipherInvalidKeySize tests that invalid key size is rejected.
func TestAesCipherInvalidKeySize(t *testing.T) {
	invalidKey := make([]byte, 15)
	iv := make([]byte, 16)

	_, err := NewAES(invalidKey, WithAESIv(iv), WithAESMode(AesModeCbc))
	assert.Error(t, err, "Should reject invalid key size")
}

// TestAesCipherInvalidIvSize tests that invalid IV size is rejected.
func TestAesCipherInvalidIvSize(t *testing.T) {
	key := make([]byte, 32)
	invalidIV := make([]byte, 8)

	_, err := NewAES(key, WithAESIv(invalidIV), WithAESMode(AesModeCbc))
	assert.Error(t, err, "Should reject invalid IV size")
}

// TestAesCipherGcmAuthentication tests GCM mode authentication tag verification.
func TestAesCipherGcmAuthentication(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewAES(key, WithAESMode(AesModeGcm))
	require.NoError(t, err, "Should create AES cipher in GCM mode")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	tamperedCiphertext := ciphertext[:len(ciphertext)-2] + "X" + ciphertext[len(ciphertext)-1:]

	_, err = cipher.Decrypt(tamperedCiphertext)
	assert.Error(t, err, "Should reject tampered ciphertext")
}

// TestAesCipherKeySizes tests AES with different key sizes.
func TestAesCipherKeySizes(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"Aes128", 16},
		{"Aes192", 24},
		{"Aes256", 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			iv := make([]byte, 16)
			_, err := rand.Read(key)
			require.NoError(t, err, "Should generate random key")
			_, err = rand.Read(iv)
			require.NoError(t, err, "Should generate random IV")

			cipher, err := NewAES(key, WithAESIv(iv), WithAESMode(AesModeCbc))
			require.NoError(t, err, "Should create AES cipher")

			plaintext := "Test message"
			ciphertext, err := cipher.Encrypt(plaintext)
			require.NoError(t, err, "Should encrypt plaintext successfully")

			decrypted, err := cipher.Decrypt(ciphertext)
			require.NoError(t, err, "Should decrypt ciphertext successfully")

			assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
		})
	}
}

// TestAesCipherDefaultMode tests that default mode is GCM.
func TestAesCipherDefaultMode(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err, "Should generate random key")

	cipher, err := NewAES(key)
	require.NoError(t, err, "Should create AES cipher with default mode")

	plaintext := "Test default mode"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestPkcs7Padding tests PKCS7 padding and unpadding.
func TestPkcs7Padding(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		blockSize int
	}{
		{"ShortString", "Hello", 16},
		{"EightByteBlock", "Test", 8},
		{"ExactBlock", "1234567890123456", 16},
		{"EmptyString", "", 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := pkcs7Padding([]byte(tt.input), tt.blockSize)

			assert.Equal(t, 0, len(padded)%tt.blockSize, "Padded length should be multiple of block size")

			unpadded, err := pkcs7Unpadding(padded)
			require.NoError(t, err, "Should unpad successfully")

			assert.Equal(t, tt.input, string(unpadded), "Unpadded text should match original input")
		})
	}
}
