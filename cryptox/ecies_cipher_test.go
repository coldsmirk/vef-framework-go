package cryptox

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestECIESEncryptDecrypt tests ECIES encryption and decryption.
func TestECIESEncryptDecrypt(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	cipher, err := NewECIES(privateKey, privateKey.PublicKey())
	require.NoError(t, err, "Should create ECIES cipher")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "ECIES encryption test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
		{"LongText", "This is a longer text to test ECIES encryption with more data"},
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

// TestEciesCurves tests ECIES with different curves.
func TestEciesCurves(t *testing.T) {
	curves := []struct {
		name  string
		curve ECIESCurve
	}{
		{"P256", EciesCurveP256},
		{"P384", EciesCurveP384},
		{"P521", EciesCurveP521},
		{"X25519", EciesCurveX25519},
	}

	for _, tc := range curves {
		t.Run(tc.name, func(t *testing.T) {
			privateKey, err := GenerateECIESKey(tc.curve)
			require.NoError(t, err, "Should generate ECIES key pair")

			cipher, err := NewECIES(privateKey, privateKey.PublicKey())
			require.NoError(t, err, "Should create ECIES cipher")

			plaintext := "Test message for " + tc.name
			ciphertext, err := cipher.Encrypt(plaintext)
			require.NoError(t, err, "Should encrypt plaintext successfully")

			decrypted, err := cipher.Decrypt(ciphertext)
			require.NoError(t, err, "Should decrypt ciphertext successfully")

			assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
		})
	}
}

// TestEciesFromBytes tests creating ECIES cipher from byte-encoded keys.
func TestEciesFromBytes(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	privateKeyBytes := privateKey.Bytes()
	publicKeyBytes := privateKey.PublicKey().Bytes()

	cipher, err := NewECIESFromBytes(privateKeyBytes, publicKeyBytes, EciesCurveP256)
	require.NoError(t, err, "Should create ECIES cipher from bytes")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestEciesFromHex tests creating ECIES cipher from hex-encoded keys.
func TestEciesFromHex(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	privateKeyHex := hex.EncodeToString(privateKey.Bytes())
	publicKeyHex := hex.EncodeToString(privateKey.PublicKey().Bytes())

	cipher, err := NewECIESFromHex(privateKeyHex, publicKeyHex, EciesCurveP256)
	require.NoError(t, err, "Should create ECIES cipher from hex")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestEciesPublicKeyOnly tests ECIES cipher with only public key.
func TestEciesPublicKeyOnly(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	cipher, err := NewECIES(nil, privateKey.PublicKey())
	require.NoError(t, err, "Should create ECIES cipher with public key only")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	_, err = cipher.Decrypt(ciphertext)
	assert.Error(t, err, "Should reject decryption without private key")
	assert.ErrorIs(t, err, ErrPrivateKeyRequiredForDecrypt, "Should return correct error")
}

// TestEciesPrivateKeyOnly tests ECIES cipher with only private key.
func TestEciesPrivateKeyOnly(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	cipher, err := NewECIES(privateKey, nil)
	require.NoError(t, err, "Should create ECIES cipher with private key only")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestEciesNoKeys tests that creating cipher without keys fails.
func TestEciesNoKeys(t *testing.T) {
	_, err := NewECIES(nil, nil)
	assert.Error(t, err, "Should reject creating cipher without any keys")
	assert.ErrorIs(t, err, ErrAtLeastOneKeyRequired, "Should return correct error")
}

// TestEciesDifferentCiphertexts tests that ECIES produces different ciphertexts.
func TestEciesDifferentCiphertexts(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	cipher, err := NewECIES(privateKey, privateKey.PublicKey())
	require.NoError(t, err, "Should create ECIES cipher")

	plaintext := "Test message"

	ciphertext1, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	ciphertext2, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	assert.NotEqual(t, ciphertext1, ciphertext2,
		"ECIES should produce different ciphertexts due to random ephemeral key and nonce")

	decrypted1, err := cipher.Decrypt(ciphertext1)
	require.NoError(t, err, "Should decrypt first ciphertext successfully")
	assert.Equal(t, plaintext, decrypted1, "First decrypted text should match original plaintext")

	decrypted2, err := cipher.Decrypt(ciphertext2)
	require.NoError(t, err, "Should decrypt second ciphertext successfully")
	assert.Equal(t, plaintext, decrypted2, "Second decrypted text should match original plaintext")
}

// TestEciesCrossKeyDecryption tests encryption with one key and decryption with another.
func TestEciesCrossKeyDecryption(t *testing.T) {
	senderKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate sender key pair")

	receiverKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate receiver key pair")

	encryptCipher, err := NewECIES(nil, receiverKey.PublicKey())
	require.NoError(t, err, "Should create ECIES cipher for encryption")

	decryptCipher, err := NewECIES(receiverKey, nil)
	require.NoError(t, err, "Should create ECIES cipher for decryption")

	plaintext := "Message from sender to receiver"
	ciphertext, err := encryptCipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := decryptCipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")

	wrongDecryptCipher, err := NewECIES(senderKey, nil)
	require.NoError(t, err, "Should create ECIES cipher with wrong key")

	_, err = wrongDecryptCipher.Decrypt(ciphertext)
	assert.Error(t, err, "Should reject decryption with wrong private key")
}

// TestEciesInvalidCiphertext tests decryption with invalid ciphertext.
func TestEciesInvalidCiphertext(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	cipher, err := NewECIES(privateKey, privateKey.PublicKey())
	require.NoError(t, err, "Should create ECIES cipher")

	_, err = cipher.Decrypt("invalid-base64")
	assert.Error(t, err, "Should reject invalid base64 ciphertext")

	_, err = cipher.Decrypt("YWJjZGVm")
	assert.Error(t, err, "Should reject malformed ciphertext")
}

// TestEciesEmptyString tests ECIES with empty string input.
func TestEciesEmptyString(t *testing.T) {
	privateKey, err := GenerateECIESKey(EciesCurveP256)
	require.NoError(t, err, "Should generate ECIES key pair")

	cipher, err := NewECIES(privateKey, privateKey.PublicKey())
	require.NoError(t, err, "Should create ECIES cipher")

	plaintext := ""
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt empty string successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match empty plaintext")
}
