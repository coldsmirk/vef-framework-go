package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

type ECIESCurve string

const (
	EciesCurveP256   ECIESCurve = "P256"
	EciesCurveP384   ECIESCurve = "P384"
	EciesCurveP521   ECIESCurve = "P521"
	EciesCurveX25519 ECIESCurve = "X25519"
)

type eciesCipher struct {
	privateKey *ecdh.PrivateKey
	publicKey  *ecdh.PublicKey
}

type ECIESOption func(*eciesCipher)

func NewECIES(privateKey *ecdh.PrivateKey, publicKey *ecdh.PublicKey, opts ...ECIESOption) (Cipher, error) {
	if privateKey == nil && publicKey == nil {
		return nil, ErrAtLeastOneKeyRequired
	}

	cipher := &eciesCipher{
		privateKey: privateKey,
		publicKey:  publicKey,
	}

	for _, opt := range opts {
		opt(cipher)
	}

	if publicKey == nil && privateKey != nil {
		cipher.publicKey = privateKey.PublicKey()
	}

	return cipher, nil
}

func NewECIESFromBytes(privateKeyBytes, publicKeyBytes []byte, curve ECIESCurve, opts ...ECIESOption) (Cipher, error) {
	var (
		privateKey *ecdh.PrivateKey
		publicKey  *ecdh.PublicKey
		err        error
	)

	ecdhCurve := getCurve(curve)

	if len(privateKeyBytes) > 0 {
		if privateKey, err = ecdhCurve.NewPrivateKey(privateKeyBytes); err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	if len(publicKeyBytes) > 0 {
		if publicKey, err = ecdhCurve.NewPublicKey(publicKeyBytes); err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return NewECIES(privateKey, publicKey, opts...)
}

func NewECIESFromHex(privateKeyHex, publicKeyHex string, curve ECIESCurve, opts ...ECIESOption) (Cipher, error) {
	var (
		privateBytes []byte
		publicBytes  []byte
		err          error
	)

	if privateKeyHex != "" {
		if privateBytes, err = hex.DecodeString(privateKeyHex); err != nil {
			return nil, fmt.Errorf("failed to decode private key from hex: %w", err)
		}
	}

	if publicKeyHex != "" {
		if publicBytes, err = hex.DecodeString(publicKeyHex); err != nil {
			return nil, fmt.Errorf("failed to decode public key from hex: %w", err)
		}
	}

	return NewECIESFromBytes(privateBytes, publicBytes, curve, opts...)
}

func NewECIESFromBase64(privateKeyBase64, publicKeyBase64 string, curve ECIESCurve, opts ...ECIESOption) (Cipher, error) {
	var (
		privateBytes []byte
		publicBytes  []byte
		err          error
	)

	if privateKeyBase64 != "" {
		if privateBytes, err = base64.StdEncoding.DecodeString(privateKeyBase64); err != nil {
			return nil, fmt.Errorf("failed to decode private key from base64: %w", err)
		}
	}

	if publicKeyBase64 != "" {
		if publicBytes, err = base64.StdEncoding.DecodeString(publicKeyBase64); err != nil {
			return nil, fmt.Errorf("failed to decode public key from base64: %w", err)
		}
	}

	return NewECIESFromBytes(privateBytes, publicBytes, curve, opts...)
}

func GenerateECIESKey(curve ECIESCurve) (*ecdh.PrivateKey, error) {
	ecdhCurve := getCurve(curve)

	return ecdhCurve.GenerateKey(rand.Reader)
}

func getCurve(curve ECIESCurve) ecdh.Curve {
	curves := map[ECIESCurve]ecdh.Curve{
		EciesCurveP256:   ecdh.P256(),
		EciesCurveP384:   ecdh.P384(),
		EciesCurveP521:   ecdh.P521(),
		EciesCurveX25519: ecdh.X25519(),
	}

	if c, ok := curves[curve]; ok {
		return c
	}

	return ecdh.P256()
}

func (e *eciesCipher) Encrypt(plaintext string) (string, error) {
	if e.publicKey == nil {
		return "", ErrPublicKeyRequiredForEncrypt
	}

	ephemeralKey, err := e.publicKey.Curve().GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	sharedSecret, err := ephemeralKey.ECDH(e.publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to derive shared secret: %w", err)
	}

	kdf := hkdf.New(sha256.New, sharedSecret, nil, nil)

	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(kdf, aesKey); err != nil {
		return "", fmt.Errorf("failed to derive AES key: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	ephemeralPublicKey := ephemeralKey.PublicKey().Bytes()
	result := make([]byte, 0, len(ephemeralPublicKey)+len(nonce)+len(ciphertext))
	result = append(result, ephemeralPublicKey...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

func (e *eciesCipher) Decrypt(ciphertext string) (string, error) {
	if e.privateKey == nil {
		return "", ErrPrivateKeyRequiredForDecrypt
	}

	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	publicKeySize := e.privateKey.PublicKey().Bytes()
	publicKeyLen := len(publicKeySize)

	if len(encryptedData) < publicKeyLen+12 {
		return "", ErrCiphertextTooShort
	}

	ephemeralPublicKeyBytes := encryptedData[:publicKeyLen]

	ephemeralPublicKey, err := e.privateKey.Curve().NewPublicKey(ephemeralPublicKeyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse ephemeral public key: %w", err)
	}

	sharedSecret, err := e.privateKey.ECDH(ephemeralPublicKey)
	if err != nil {
		return "", fmt.Errorf("failed to derive shared secret: %w", err)
	}

	kdf := hkdf.New(sha256.New, sharedSecret, nil, nil)

	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(kdf, aesKey); err != nil {
		return "", fmt.Errorf("failed to derive AES key: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < publicKeyLen+nonceSize {
		return "", ErrCiphertextTooShort
	}

	nonce := encryptedData[publicKeyLen : publicKeyLen+nonceSize]
	ciphertextData := encryptedData[publicKeyLen+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

var _ Cipher = (*eciesCipher)(nil)
