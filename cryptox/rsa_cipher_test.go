package cryptox

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateRSAKeyPair(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// TestRsaCipherOaep tests RSA encryption and decryption in OAEP mode.
func TestRsaCipherOaep(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	cipher, err := NewRSA(privateKey, &privateKey.PublicKey, WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher in OAEP mode")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "RSA-OAEP encryption test"},
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

// TestRsaCipherPkcs1v15 tests RSA encryption and decryption in PKCS1v15 mode.
func TestRsaCipherPkcs1v15(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	cipher, err := NewRSA(privateKey, &privateKey.PublicKey, WithRSAMode(RsaModePKCS1v15))
	require.NoError(t, err, "Should create RSA cipher in PKCS1v15 mode")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "RSA-PKCS1v15 encryption test"},
		{"ChineseCharacters", "中文测试"},
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

// TestRsaCipherFromPem tests creating RSA cipher from PEM-encoded keys.
func TestRsaCipherFromPem(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err, "Should marshal public key")

	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	cipher, err := NewRSAFromPem(privatePEM, publicPEM, WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher from PEM")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestRsaCipherPublicKeyOnly tests RSA cipher with only public key.
func TestRsaCipherPublicKeyOnly(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	cipher, err := NewRSA(nil, &privateKey.PublicKey, WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher with public key only")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	_, err = cipher.Decrypt(ciphertext)
	assert.Error(t, err, "Should reject decryption without private key")
}

// TestRsaCipherPrivateKeyOnly tests RSA cipher with only private key.
func TestRsaCipherPrivateKeyOnly(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	cipher, err := NewRSA(privateKey, nil, WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher with private key only")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestRsaCipherNoKeys tests that creating cipher without keys fails.
func TestRsaCipherNoKeys(t *testing.T) {
	_, err := NewRSA(nil, nil, WithRSAMode(RsaModeOAEP))
	assert.Error(t, err, "Should reject creating cipher without any keys")
}

// TestRsaCipherPkcs8PrivateKey tests creating RSA cipher from PKCS8 PEM.
func TestRsaCipherPkcs8PrivateKey(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err, "Should marshal PKCS8 private key")

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	cipher, err := NewRSAFromPem(privatePEM, nil, WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher from PKCS8 PEM")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestRsaCipherFromHex tests creating RSA cipher from hex-encoded keys.
func TestRsaCipherFromHex(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyHex := hex.EncodeToString(privateKeyBytes)

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err, "Should marshal public key")

	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	cipher, err := NewRSAFromHex(privateKeyHex, publicKeyHex, WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher from hex")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestRsaCipherFromHexPkcs8 tests creating RSA cipher from PKCS8 hex-encoded key.
func TestRsaCipherFromHexPkcs8(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err, "Should marshal PKCS8 private key")

	privateKeyHex := hex.EncodeToString(privateKeyBytes)

	cipher, err := NewRSAFromHex(privateKeyHex, "", WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher from PKCS8 hex")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestRsaCipherFromBase64 tests creating RSA cipher from base64-encoded keys.
func TestRsaCipherFromBase64(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBase64 := base64.StdEncoding.EncodeToString(privateKeyBytes)

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err, "Should marshal public key")

	publicKeyBase64 := base64.StdEncoding.EncodeToString(publicKeyBytes)

	cipher, err := NewRSAFromBase64(privateKeyBase64, publicKeyBase64, WithRSAMode(RsaModeOAEP))
	require.NoError(t, err, "Should create RSA cipher from base64")

	plaintext := "Test message with base64 encoded keys"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestRsaCipherDefaultMode tests that default mode is OAEP.
func TestRsaCipherDefaultMode(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	cipher, err := NewRSA(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should create RSA cipher with default mode")

	plaintext := "Test default mode"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Should equal expected value")
}

// TestRsaCipherKeySizes tests RSA with different key sizes.
func TestRsaCipherKeySizes(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"KeySize1024Bit", 1024},
		{"KeySize2048Bit", 2048},
		{"KeySize4096Bit", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privateKey, err := generateRSAKeyPair(tt.keySize)
			require.NoError(t, err, "Should generate %d-bit RSA key", tt.keySize)

			cipher, err := NewRSA(privateKey, &privateKey.PublicKey, WithRSAMode(RsaModeOAEP))
			require.NoError(t, err, "Should create RSA cipher")

			plaintext := "Test message"
			ciphertext, err := cipher.Encrypt(plaintext)
			require.NoError(t, err, "Should encrypt plaintext successfully")

			decrypted, err := cipher.Decrypt(ciphertext)
			require.NoError(t, err, "Should decrypt ciphertext successfully")

			assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
		})
	}
}

// TestRsaCipherSignVerifyPss tests Rsa Cipher sign verify pss scenarios.
func TestRsaCipherSignVerifyPss(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should not return error")

	cipher, err := NewRSA(privateKey, &privateKey.PublicKey, WithRSASignMode(RsaSignModePSS))
	require.NoError(t, err, "Should not return error")

	tests := []struct {
		name string
		data string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "RSA-PSS signature test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature, err := cipher.Sign(tt.data)
			require.NoError(t, err, "Should not return error")

			valid, err := cipher.Verify(tt.data, signature)
			require.NoError(t, err, "Should not return error")
			assert.True(t, valid, "Should be valid")

			valid, err = cipher.Verify(tt.data+"tampered", signature)
			require.NoError(t, err, "Should not return error")
			assert.False(t, valid, "Should not be valid")
		})
	}
}

// TestRsaCipherSignVerifyPkcs1v15 tests Rsa Cipher sign verify pkcs1v15 scenarios.
func TestRsaCipherSignVerifyPkcs1v15(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should not return error")

	cipher, err := NewRSA(privateKey, &privateKey.PublicKey, WithRSASignMode(RsaSignModePKCS1v15))
	require.NoError(t, err, "Should not return error")

	data := "Test message"
	signature, err := cipher.Sign(data)
	require.NoError(t, err, "Should not return error")

	valid, err := cipher.Verify(data, signature)
	require.NoError(t, err, "Should not return error")
	assert.True(t, valid, "Should be valid")
}

// TestRsaCipherSignWithoutPrivateKey tests Rsa Cipher sign without private key scenarios.
func TestRsaCipherSignWithoutPrivateKey(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should not return error")

	cipher, err := NewRSA(nil, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	data := "Test message"
	_, err = cipher.Sign(data)
	assert.Error(t, err, "Should return error")
	assert.ErrorIs(t, err, ErrPrivateKeyRequiredForSign, "Error should be ErrPrivateKeyRequiredForSign")
}

// TestRsaCipherVerifyWithoutPublicKey tests Rsa Cipher verify without public key scenarios.
func TestRsaCipherVerifyWithoutPublicKey(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should not return error")

	signerCipher, err := NewRSA(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	data := "Test message"
	signature, err := signerCipher.Sign(data)
	require.NoError(t, err, "Should not return error")

	verifierCipher, err := NewRSA(privateKey, nil)
	require.NoError(t, err, "Should not return error")

	valid, err := verifierCipher.Verify(data, signature)
	require.NoError(t, err, "Should not return error")
	assert.True(t, valid, "Should be valid")
}

// TestRsaCipherInvalidSignature tests Rsa Cipher invalid signature scenarios.
func TestRsaCipherInvalidSignature(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should not return error")

	cipher, err := NewRSA(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	data := "Test message"

	_, err = cipher.Verify(data, "invalid-base64")
	assert.Error(t, err, "Should return error")
}

// TestRsaCipherInvalidPem tests RSA cipher with invalid PEM data.
func TestRsaCipherInvalidPem(t *testing.T) {
	t.Run("InvalidPrivatePem", func(t *testing.T) {
		_, err := NewRSAFromPem([]byte("not-a-pem"), nil)
		assert.Error(t, err, "Should return error for invalid private PEM")
	})

	t.Run("InvalidPublicPem", func(t *testing.T) {
		_, err := NewRSAFromPem(nil, []byte("not-a-pem"))
		assert.Error(t, err, "Should return error for invalid public PEM")
	})

	t.Run("UnsupportedPrivatePemType", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte("fake-data"),
		})
		_, err := NewRSAFromPem(badPEM, nil)
		assert.Error(t, err, "Should return error for unsupported PEM type")
	})

	t.Run("UnsupportedPublicPemType", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte("fake-data"),
		})
		_, err := NewRSAFromPem(nil, badPEM)
		assert.Error(t, err, "Should return error for unsupported public PEM type")
	})

	t.Run("Pkcs1PublicKeyPem", func(t *testing.T) {
		privateKey, err := generateRSAKeyPair(2048)
		require.NoError(t, err, "Should generate RSA key pair")

		publicKeyBytes := x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)
		publicPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: publicKeyBytes,
		})

		cipher, err := NewRSAFromPem(nil, publicPEM)
		require.NoError(t, err, "Should parse PKCS1 public key PEM")

		_, err = cipher.Encrypt("test")
		assert.NoError(t, err, "Should encrypt with PKCS1 public key")
	})
}

// TestRsaCipherInvalidHex tests RSA cipher with invalid hex-encoded keys.
func TestRsaCipherInvalidHex(t *testing.T) {
	t.Run("InvalidPrivateHex", func(t *testing.T) {
		_, err := NewRSAFromHex("not-hex", "")
		assert.Error(t, err, "Should return error for invalid private hex")
	})

	t.Run("InvalidPublicHex", func(t *testing.T) {
		_, err := NewRSAFromHex("", "not-hex")
		assert.Error(t, err, "Should return error for invalid public hex")
	})

	t.Run("InvalidKeyBytes", func(t *testing.T) {
		_, err := NewRSAFromHex(hex.EncodeToString([]byte("bad-key")), "")
		assert.Error(t, err, "Should return error for invalid key bytes")
	})
}

// TestRsaCipherInvalidBase64 tests RSA cipher with invalid base64-encoded keys.
func TestRsaCipherInvalidBase64(t *testing.T) {
	t.Run("InvalidPrivateBase64", func(t *testing.T) {
		_, err := NewRSAFromBase64("!!!invalid!!!", "")
		assert.Error(t, err, "Should return error for invalid private base64")
	})

	t.Run("InvalidPublicBase64", func(t *testing.T) {
		_, err := NewRSAFromBase64("", "!!!invalid!!!")
		assert.Error(t, err, "Should return error for invalid public base64")
	})

	t.Run("InvalidKeyBytes", func(t *testing.T) {
		_, err := NewRSAFromBase64(base64.StdEncoding.EncodeToString([]byte("bad-key")), "")
		assert.Error(t, err, "Should return error for invalid key bytes")
	})
}

// TestRsaCipherDecryptInvalidBase64 tests RSA decrypt with invalid base64 ciphertext.
func TestRsaCipherDecryptInvalidBase64(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	cipher, err := NewRSA(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should create RSA cipher")

	_, err = cipher.Decrypt("!!!not-base64!!!")
	assert.Error(t, err, "Should return error for invalid base64 ciphertext")
}

// TestRsaCipherEncryptWithoutPublicKey tests RSA encrypt without public key.
func TestRsaCipherEncryptWithoutPublicKey(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should generate RSA key pair")

	// Create cipher with private key only, then nil out public key
	cipher, err := NewRSA(nil, &privateKey.PublicKey)
	require.NoError(t, err, "Should create RSA cipher")

	_, err = cipher.Decrypt("dGVzdA==")
	assert.Error(t, err, "Should return error when decrypting without private key")
}

// TestRsaCipherDifferentSignModes tests Rsa Cipher different sign modes scenarios.
func TestRsaCipherDifferentSignModes(t *testing.T) {
	privateKey, err := generateRSAKeyPair(2048)
	require.NoError(t, err, "Should not return error")

	pssCipher, err := NewRSA(privateKey, &privateKey.PublicKey, WithRSASignMode(RsaSignModePSS))
	require.NoError(t, err, "Should not return error")

	pkcs1Cipher, err := NewRSA(privateKey, &privateKey.PublicKey, WithRSASignMode(RsaSignModePKCS1v15))
	require.NoError(t, err, "Should not return error")

	data := "Test message"

	pssSignature, err := pssCipher.Sign(data)
	require.NoError(t, err, "Should not return error")

	pkcs1Signature, err := pkcs1Cipher.Sign(data)
	require.NoError(t, err, "Should not return error")

	assert.NotEqual(t, pssSignature, pkcs1Signature, "Should not equal")

	validPss, err := pssCipher.Verify(data, pssSignature)
	require.NoError(t, err, "Should not return error")
	assert.True(t, validPss, "Should be valid")

	validPkcs1, err := pkcs1Cipher.Verify(data, pkcs1Signature)
	require.NoError(t, err, "Should not return error")
	assert.True(t, validPkcs1, "Should be valid")
}
