package cryptox

import (
	"crypto/rand"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/x509"
)

func generateSM2KeyPair() (*sm2.PrivateKey, error) {
	return sm2.GenerateKey(rand.Reader)
}

// TestSm2CipherEncryptDecrypt tests SM2 encryption and decryption.
func TestSm2CipherEncryptDecrypt(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should generate SM2 key pair")

	cipher, err := NewSM2(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should create SM2 cipher")

	tests := []struct {
		name      string
		plaintext string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "SM2 encryption test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
		{"ChineseAlgorithm", "国密SM2加密算法"},
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

// TestSm2CipherFromPem tests creating SM2 cipher from PEM-encoded keys.
func TestSm2CipherFromPem(t *testing.T) {
	priv, err := generateSM2KeyPair()
	require.NoError(t, err, "Should generate SM2 key pair")

	type sm2Priv struct {
		Version       int
		PrivateKey    []byte
		NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
		PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
	}

	derPriv, err := asn1.Marshal(sm2Priv{Version: 1, PrivateKey: priv.D.Bytes()})
	require.NoError(t, err, "Should marshal SM2 private key")
	derPub, err := x509.MarshalSm2PublicKey(&priv.PublicKey)
	require.NoError(t, err, "Should marshal SM2 public key")

	pemPriv := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derPriv})
	pemPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: derPub})

	cipher, err := NewSM2FromPEM(pemPriv, pemPub)
	require.NoError(t, err, "Should create SM2 cipher from PEM")

	plaintext := "PEM roundtrip message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")
	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")
	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestSm2CipherFromHex tests creating SM2 cipher from hex-encoded keys.
func TestSm2CipherFromHex(t *testing.T) {
	priv, err := generateSM2KeyPair()
	require.NoError(t, err, "Should generate SM2 key pair")

	type sm2Priv struct {
		Version       int
		PrivateKey    []byte
		NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
		PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
	}

	derPriv, err := asn1.Marshal(sm2Priv{Version: 1, PrivateKey: priv.D.Bytes()})
	require.NoError(t, err, "Should marshal SM2 private key")
	derPub, err := x509.MarshalSm2PublicKey(&priv.PublicKey)
	require.NoError(t, err, "Should marshal SM2 public key")

	hexPriv := hex.EncodeToString(derPriv)
	hexPub := hex.EncodeToString(derPub)

	cipher, err := NewSM2FromHex(hexPriv, hexPub)
	require.NoError(t, err, "Should create SM2 cipher from hex")

	plaintext := "HEX roundtrip message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")
	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")
	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestSm2CipherPublicKeyOnly tests SM2 cipher with only public key.
func TestSm2CipherPublicKeyOnly(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should generate SM2 key pair")

	cipher, err := NewSM2(nil, &privateKey.PublicKey)
	require.NoError(t, err, "Should create SM2 cipher with public key only")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	_, err = cipher.Decrypt(ciphertext)
	assert.Error(t, err, "Should reject decryption without private key")
}

// TestSm2CipherPrivateKeyOnly tests SM2 cipher with only private key.
func TestSm2CipherPrivateKeyOnly(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should generate SM2 key pair")

	cipher, err := NewSM2(privateKey, nil)
	require.NoError(t, err, "Should create SM2 cipher with private key only")

	plaintext := "Test message"
	ciphertext, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	decrypted, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted, "Decrypted text should match original plaintext")
}

// TestSm2CipherNoKeys tests that creating cipher without keys fails.
func TestSm2CipherNoKeys(t *testing.T) {
	_, err := NewSM2(nil, nil)
	assert.Error(t, err, "Should reject creating cipher without any keys")
}

// TestSm2CipherMultipleEncryptions tests that SM2 produces different ciphertexts.
func TestSm2CipherMultipleEncryptions(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should generate SM2 key pair")

	cipher, err := NewSM2(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should create SM2 cipher")

	plaintext := "Test message"

	ciphertext1, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	ciphertext2, err := cipher.Encrypt(plaintext)
	require.NoError(t, err, "Should encrypt plaintext successfully")

	assert.NotEqual(t, ciphertext1, ciphertext2,
		"SM2 should produce different ciphertexts due to random component")

	decrypted1, err := cipher.Decrypt(ciphertext1)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	decrypted2, err := cipher.Decrypt(ciphertext2)
	require.NoError(t, err, "Should decrypt ciphertext successfully")

	assert.Equal(t, plaintext, decrypted1, "First decrypted text should match original plaintext")
	assert.Equal(t, plaintext, decrypted2, "Second decrypted text should match original plaintext")
}

// TestSm2CipherSignVerify tests Sm2 Cipher sign verify scenarios.
func TestSm2CipherSignVerify(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should not return error")

	cipher, err := NewSM2(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	tests := []struct {
		name string
		data string
	}{
		{"EnglishText", "Hello, World!"},
		{"WithDescription", "SM2 signature test"},
		{"ChineseCharacters", "中文测试"},
		{"SpecialCharacters", "Special chars: !@#$%^&*()"},
		{"ChineseAlgorithm", "国密SM2签名算法"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature, err := cipher.Sign(tt.data)
			require.NoError(t, err, "Should sign data without error")

			valid, err := cipher.Verify(tt.data, signature)
			require.NoError(t, err, "Should verify signature without error")
			assert.True(t, valid, "Signature should be valid for original data")

			valid, err = cipher.Verify(tt.data+"tampered", signature)
			require.NoError(t, err, "Should verify tampered data without error")
			assert.False(t, valid, "Signature should be invalid for tampered data")
		})
	}
}

// TestSm2CipherSignWithoutPrivateKey tests Sm2 Cipher sign without private key scenarios.
func TestSm2CipherSignWithoutPrivateKey(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should not return error")

	cipher, err := NewSM2(nil, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	data := "Test message"
	_, err = cipher.Sign(data)
	assert.Error(t, err, "Should return error")
	assert.ErrorIs(t, err, ErrPrivateKeyRequiredForSign, "Error should be ErrPrivateKeyRequiredForSign")
}

// TestSm2CipherVerifyWithoutPublicKey tests Sm2 Cipher verify without public key scenarios.
func TestSm2CipherVerifyWithoutPublicKey(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should not return error")

	signerCipher, err := NewSM2(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	data := "Test message"
	signature, err := signerCipher.Sign(data)
	require.NoError(t, err, "Should not return error")

	verifierCipher, err := NewSM2(privateKey, nil)
	require.NoError(t, err, "Should not return error")

	valid, err := verifierCipher.Verify(data, signature)
	require.NoError(t, err, "Should verify signature without error")
	assert.True(t, valid, "Signature should be valid when verified using derived public key")
}

// TestSm2CipherInvalidSignature tests Sm2 Cipher invalid signature scenarios.
func TestSm2CipherInvalidSignature(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should not return error")

	cipher, err := NewSM2(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	data := "Test message"

	_, err = cipher.Verify(data, "invalid-base64")
	assert.Error(t, err, "Should return error")
}

// TestSm2CipherDifferentSignatures tests Sm2 Cipher different signatures scenarios.
func TestSm2CipherDifferentSignatures(t *testing.T) {
	privateKey, err := generateSM2KeyPair()
	require.NoError(t, err, "Should not return error")

	cipher, err := NewSM2(privateKey, &privateKey.PublicKey)
	require.NoError(t, err, "Should not return error")

	data := "Test message"

	signature1, err := cipher.Sign(data)
	require.NoError(t, err, "Should not return error")

	signature2, err := cipher.Sign(data)
	require.NoError(t, err, "Should not return error")

	assert.NotEqual(t, signature1, signature2,
		"SM2 should produce different signatures due to random component")

	valid1, err := cipher.Verify(data, signature1)
	require.NoError(t, err, "Should verify first signature without error")
	assert.True(t, valid1, "First signature should be valid")

	valid2, err := cipher.Verify(data, signature2)
	require.NoError(t, err, "Should verify second signature without error")
	assert.True(t, valid2, "Second signature should be valid")
}
