package cryptox

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEcdsaSignVerify tests ECDSA signing and verification.
func TestEcdsaSignVerify(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	cipher, err := NewECDSA(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should create ECDSA cipher")

	tests := []struct {
		name string
		data string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "ECDSA signature test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature, err := cipher.Sign(tt.data)
			require.NoError(t, err, "Should sign data successfully")

			valid, err := cipher.Verify(tt.data, signature)
			require.NoError(t, err, "Should verify signature successfully")
			assert.True(t, valid, "Signature should be valid")

			valid, err = cipher.Verify(tt.data+"tampered", signature)
			require.NoError(t, err, "Should verify tampered data successfully")
			assert.False(t, valid, "Signature should be invalid for tampered data")
		})
	}
}

// TestEcdsaFromPem tests creating ECDSA cipher from PEM-encoded keys.
func TestEcdsaFromPem(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err, "Should marshal EC private key")

	privatePem := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err, "Should marshal public key")

	publicPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	cipher, err := NewECDSAFromPem(privatePem, publicPem)
	require.NoError(t, err, "Should create ECDSA cipher from PEM")

	data := "Test message"
	signature, err := cipher.Sign(data)
	require.NoError(t, err, "Should sign data successfully")

	valid, err := cipher.Verify(data, signature)
	require.NoError(t, err, "Should verify signature successfully")
	assert.True(t, valid, "Signature should be valid")
}

// TestEcdsaFromHex tests creating ECDSA cipher from hex-encoded keys.
func TestEcdsaFromHex(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err, "Should marshal EC private key")

	privateKeyHex := hex.EncodeToString(privateKeyBytes)

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err, "Should marshal public key")

	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	cipher, err := NewECDSAFromHex(privateKeyHex, publicKeyHex)
	require.NoError(t, err, "Should create ECDSA cipher from hex")

	data := "Test message"
	signature, err := cipher.Sign(data)
	require.NoError(t, err, "Should sign data successfully")

	valid, err := cipher.Verify(data, signature)
	require.NoError(t, err, "Should verify signature successfully")
	assert.True(t, valid, "Signature should be valid")
}

// TestEcdsaPublicKeyOnly tests ECDSA cipher with only public key.
func TestEcdsaPublicKeyOnly(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	cipher, err := NewECDSA(nil, &privateKey.PublicKey)
	require.NoError(t, err, "Should create ECDSA cipher with public key only")

	data := "Test message"
	_, err = cipher.Sign(data)
	assert.Error(t, err, "Should reject signing without private key")
	assert.ErrorIs(t, err, ErrPrivateKeyRequiredForSign, "Should return correct error")
}

// TestEcdsaPrivateKeyOnly tests ECDSA cipher with only private key.
func TestEcdsaPrivateKeyOnly(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	cipher, err := NewECDSA(privateKey, nil)
	require.NoError(t, err, "Should create ECDSA cipher with private key only")

	data := "Test message"
	signature, err := cipher.Sign(data)
	require.NoError(t, err, "Should sign data successfully")

	valid, err := cipher.Verify(data, signature)
	require.NoError(t, err, "Should verify signature successfully")
	assert.True(t, valid, "Signature should be valid")
}

// TestEcdsaNoKeys tests that creating cipher without keys fails.
func TestEcdsaNoKeys(t *testing.T) {
	_, err := NewECDSA(nil, nil)
	assert.Error(t, err, "Should reject creating cipher without any keys")
	assert.ErrorIs(t, err, ErrAtLeastOneKeyRequired, "Should return correct error")
}

// TestEcdsaCurves tests ECDSA with different curves.
func TestEcdsaCurves(t *testing.T) {
	curves := []struct {
		name  string
		curve ECDSACurve
	}{
		{"P224", EcdsaCurveP224},
		{"P256", EcdsaCurveP256},
		{"P384", EcdsaCurveP384},
		{"P521", EcdsaCurveP521},
	}

	for _, tc := range curves {
		t.Run(tc.name, func(t *testing.T) {
			privateKey, err := GenerateECDSAKey(tc.curve)
			require.NoError(t, err, "Should generate ECDSA key pair")

			cipher, err := NewECDSA(privateKey, &privateKey.PublicKey)
			require.NoError(t, err, "Should create ECDSA cipher")

			data := "Test message"
			signature, err := cipher.Sign(data)
			require.NoError(t, err, "Should sign data successfully")

			valid, err := cipher.Verify(data, signature)
			require.NoError(t, err, "Should verify signature successfully")
			assert.True(t, valid, "Signature should be valid")
		})
	}
}

// TestEcdsaInvalidSignature tests verification with invalid signatures.
func TestEcdsaInvalidSignature(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	cipher, err := NewECDSA(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should create ECDSA cipher")

	data := "Test message"

	_, err = cipher.Verify(data, "invalid-base64")
	assert.Error(t, err, "Should reject invalid base64 signature")

	_, err = cipher.Verify(data, "YWJjZGVm")
	assert.Error(t, err, "Should reject malformed signature")
}

// TestEcdsaPkcs8PrivateKey tests creating ECDSA cipher from PKCS8 PEM.
func TestEcdsaPkcs8PrivateKey(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err, "Should marshal PKCS8 private key")

	privatePem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	cipher, err := NewECDSAFromPem(privatePem, nil)
	require.NoError(t, err, "Should create ECDSA cipher from PKCS8 PEM")

	data := "Test message"
	signature, err := cipher.Sign(data)
	require.NoError(t, err, "Should sign data successfully")

	valid, err := cipher.Verify(data, signature)
	require.NoError(t, err, "Should verify signature successfully")
	assert.True(t, valid, "Signature should be valid")
}

// TestEcdsaInvalidPem tests ECDSA cipher with invalid PEM data.
func TestEcdsaInvalidPem(t *testing.T) {
	t.Run("InvalidPrivatePem", func(t *testing.T) {
		_, err := NewECDSAFromPem([]byte("not-a-pem"), nil)
		assert.Error(t, err, "Should return error for invalid private PEM")
	})

	t.Run("InvalidPublicPem", func(t *testing.T) {
		_, err := NewECDSAFromPem(nil, []byte("not-a-pem"))
		assert.Error(t, err, "Should return error for invalid public PEM")
	})

	t.Run("UnsupportedPrivatePemType", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte("fake-data"),
		})
		_, err := NewECDSAFromPem(badPEM, nil)
		assert.Error(t, err, "Should return error for unsupported PEM type")
	})

	t.Run("UnsupportedPublicPemType", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte("fake-data"),
		})
		_, err := NewECDSAFromPem(nil, badPEM)
		assert.Error(t, err, "Should return error for unsupported public PEM type")
	})
}

// TestEcdsaInvalidHex tests ECDSA cipher with invalid hex-encoded keys.
func TestEcdsaInvalidHex(t *testing.T) {
	t.Run("InvalidPrivateHex", func(t *testing.T) {
		_, err := NewECDSAFromHex("not-hex", "")
		assert.Error(t, err, "Should return error for invalid private hex")
	})

	t.Run("InvalidPublicHex", func(t *testing.T) {
		_, err := NewECDSAFromHex("", "not-hex")
		assert.Error(t, err, "Should return error for invalid public hex")
	})

	t.Run("InvalidKeyBytes", func(t *testing.T) {
		_, err := NewECDSAFromHex(hex.EncodeToString([]byte("bad-key")), "")
		assert.Error(t, err, "Should return error for invalid key bytes")
	})
}

// TestEcdsaFromBase64 tests creating ECDSA cipher from base64-encoded keys.
func TestEcdsaFromBase64(t *testing.T) {
	t.Run("ValidKeys", func(t *testing.T) {
		privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
		require.NoError(t, err, "Should generate ECDSA key pair")

		privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
		require.NoError(t, err, "Should marshal EC private key")

		publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		require.NoError(t, err, "Should marshal public key")

		cipher, err := NewECDSAFromBase64(
			base64.StdEncoding.EncodeToString(privateKeyBytes),
			base64.StdEncoding.EncodeToString(publicKeyBytes),
		)
		require.NoError(t, err, "Should create ECDSA cipher from base64")

		data := "Test message"
		signature, err := cipher.Sign(data)
		require.NoError(t, err, "Should sign data")

		valid, err := cipher.Verify(data, signature)
		require.NoError(t, err, "Should verify")
		assert.True(t, valid, "Signature should be valid")
	})

	t.Run("InvalidPrivateBase64", func(t *testing.T) {
		_, err := NewECDSAFromBase64("!!!invalid!!!", "")
		assert.Error(t, err, "Should return error for invalid private base64")
	})

	t.Run("InvalidPublicBase64", func(t *testing.T) {
		_, err := NewECDSAFromBase64("", "!!!invalid!!!")
		assert.Error(t, err, "Should return error for invalid public base64")
	})

	t.Run("InvalidKeyBytes", func(t *testing.T) {
		_, err := NewECDSAFromBase64(base64.StdEncoding.EncodeToString([]byte("bad-key")), "")
		assert.Error(t, err, "Should return error for invalid key bytes")
	})
}

// TestEcdsaVerifyWithoutPublicKey tests ECDSA verify without public key.
func TestEcdsaVerifyWithoutPublicKey(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	// Verify with derived public key (from private key)
	cipher, err := NewECDSA(privateKey, nil)
	require.NoError(t, err, "Should create ECDSA cipher")

	data := "Test message"
	signature, err := cipher.Sign(data)
	require.NoError(t, err, "Should sign")

	valid, err := cipher.Verify(data, signature)
	require.NoError(t, err, "Should verify")
	assert.True(t, valid, "Should be valid")
}

// TestEcdsaDifferentSignatures tests that ECDSA produces different signatures.
func TestEcdsaDifferentSignatures(t *testing.T) {
	privateKey, err := GenerateECDSAKey(EcdsaCurveP256)
	require.NoError(t, err, "Should generate ECDSA key pair")

	cipher, err := NewECDSA(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should create ECDSA cipher")

	data := "Test message"

	signature1, err := cipher.Sign(data)
	require.NoError(t, err, "Should sign data successfully")

	signature2, err := cipher.Sign(data)
	require.NoError(t, err, "Should sign data successfully")

	assert.NotEqual(t, signature1, signature2,
		"ECDSA should produce different signatures due to random component")

	valid1, err := cipher.Verify(data, signature1)
	require.NoError(t, err, "Should verify first signature successfully")
	assert.True(t, valid1, "First signature should be valid")

	valid2, err := cipher.Verify(data, signature2)
	require.NoError(t, err, "Should verify second signature successfully")
	assert.True(t, valid2, "Second signature should be valid")
}
