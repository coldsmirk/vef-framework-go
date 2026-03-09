package cryptox

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
)

type RSAMode string

const (
	RsaModeOAEP     RSAMode = "OAEP"
	RsaModePKCS1v15 RSAMode = "PKCS1v15"
)

type RSASignMode string

const (
	RsaSignModePSS      RSASignMode = "PSS"
	RsaSignModePKCS1v15 RSASignMode = "PKCS1v15"
)

type rsaCipher struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	mode       RSAMode
	signMode   RSASignMode
}

type RSAOption func(*rsaCipher)

func WithRSAMode(mode RSAMode) RSAOption {
	return func(c *rsaCipher) {
		c.mode = mode
	}
}

func WithRSASignMode(signMode RSASignMode) RSAOption {
	return func(c *rsaCipher) {
		c.signMode = signMode
	}
}

func NewRSA(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, opts ...RSAOption) (CipherSigner, error) {
	if privateKey == nil && publicKey == nil {
		return nil, ErrAtLeastOneKeyRequired
	}

	cipher := &rsaCipher{
		privateKey: privateKey,
		publicKey:  publicKey,
		mode:       RsaModeOAEP,
		signMode:   RsaSignModePSS,
	}

	for _, opt := range opts {
		opt(cipher)
	}

	if publicKey == nil && privateKey != nil {
		cipher.publicKey = &privateKey.PublicKey
	}

	return cipher, nil
}

func parseRSAKeysFromBytes(privateKeyBytes, publicKeyBytes []byte) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	var (
		privateKey *rsa.PrivateKey
		publicKey  *rsa.PublicKey
		err        error
	)

	if len(privateKeyBytes) > 0 {
		privateKey, err = x509.ParsePKCS1PrivateKey(privateKeyBytes)
		if err != nil {
			key, pkcs8Err := x509.ParsePKCS8PrivateKey(privateKeyBytes)
			if pkcs8Err != nil {
				return nil, nil, fmt.Errorf("failed to parse private key (tried PKCS1 and PKCS8): %w", err)
			}

			var ok bool

			privateKey, ok = key.(*rsa.PrivateKey)
			if !ok {
				return nil, nil, ErrNotRsaPrivateKey
			}
		}
	}

	if len(publicKeyBytes) > 0 {
		key, err := x509.ParsePKIXPublicKey(publicKeyBytes)
		if err != nil {
			publicKey, err = x509.ParsePKCS1PublicKey(publicKeyBytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse public key (tried PKIX and PKCS1): %w", err)
			}
		} else {
			var ok bool

			publicKey, ok = key.(*rsa.PublicKey)
			if !ok {
				return nil, nil, ErrNotRsaPublicKey
			}
		}
	}

	return privateKey, publicKey, nil
}

func NewRSAFromPem(privatePem, publicPem []byte, opts ...RSAOption) (CipherSigner, error) {
	var (
		privateKey *rsa.PrivateKey
		publicKey  *rsa.PublicKey
		err        error
	)

	if privatePem != nil {
		if privateKey, err = parseRSAPrivateKeyFromPem(privatePem); err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	if publicPem != nil {
		if publicKey, err = parseRSAPublicKeyFromPem(publicPem); err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return NewRSA(privateKey, publicKey, opts...)
}

func NewRSAFromHex(privateKeyHex, publicKeyHex string, opts ...RSAOption) (CipherSigner, error) {
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

	privateKey, publicKey, err := parseRSAKeysFromBytes(privateBytes, publicBytes)
	if err != nil {
		return nil, err
	}

	return NewRSA(privateKey, publicKey, opts...)
}

func NewRSAFromBase64(privateKeyBase64, publicKeyBase64 string, opts ...RSAOption) (CipherSigner, error) {
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

	privateKey, publicKey, err := parseRSAKeysFromBytes(privateBytes, publicBytes)
	if err != nil {
		return nil, err
	}

	return NewRSA(privateKey, publicKey, opts...)
}

func (r *rsaCipher) Encrypt(plaintext string) (string, error) {
	if r.publicKey == nil {
		return "", ErrPublicKeyRequiredForEncrypt
	}

	var (
		ciphertext []byte
		err        error
	)

	if r.mode == RsaModeOAEP {
		hash := sha256.New()
		ciphertext, err = rsa.EncryptOAEP(hash, rand.Reader, r.publicKey, []byte(plaintext), nil)
	} else {
		ciphertext, err = rsa.EncryptPKCS1v15(rand.Reader, r.publicKey, []byte(plaintext)) //nolint:staticcheck // PKCS1v15 mode is user-configurable for legacy interop
	}

	if err != nil {
		return "", fmt.Errorf("failed to encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (r *rsaCipher) Decrypt(ciphertext string) (string, error) {
	if r.privateKey == nil {
		return "", ErrPrivateKeyRequiredForDecrypt
	}

	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	var plaintext []byte
	if r.mode == RsaModeOAEP {
		hash := sha256.New()
		plaintext, err = rsa.DecryptOAEP(hash, rand.Reader, r.privateKey, encryptedData, nil)
	} else {
		plaintext, err = rsa.DecryptPKCS1v15(rand.Reader, r.privateKey, encryptedData) //nolint:staticcheck // PKCS1v15 mode is user-configurable for legacy interop
	}

	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

func parseRSAPrivateKeyFromPem(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrFailedDecodePemBlock
	}

	if block.Type == "RSA PRIVATE KEY" {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}

	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, ErrNotRsaPrivateKey
		}

		return rsaKey, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrUnsupportedPemType, block.Type)
}

func parseRSAPublicKeyFromPem(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrFailedDecodePemBlock
	}

	if block.Type == "PUBLIC KEY" {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, ErrNotRsaPublicKey
		}

		return rsaKey, nil
	}

	if block.Type == "RSA PUBLIC KEY" {
		return x509.ParsePKCS1PublicKey(block.Bytes)
	}

	return nil, fmt.Errorf("%w: %s", ErrUnsupportedPemType, block.Type)
}

func (r *rsaCipher) Sign(data string) (string, error) {
	if r.privateKey == nil {
		return "", ErrPrivateKeyRequiredForSign
	}

	hash := sha256.New()
	_, _ = hash.Write([]byte(data))
	hashed := hash.Sum(nil)

	var (
		signature []byte
		err       error
	)

	if r.signMode == RsaSignModePSS {
		signature, err = rsa.SignPSS(rand.Reader, r.privateKey, crypto.SHA256, hashed, nil)
	} else {
		signature, err = rsa.SignPKCS1v15(rand.Reader, r.privateKey, crypto.SHA256, hashed)
	}

	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

func (r *rsaCipher) Verify(data, signature string) (bool, error) {
	if r.publicKey == nil {
		return false, ErrPublicKeyRequiredForVerify
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	hash := sha256.New()
	_, _ = hash.Write([]byte(data))
	hashed := hash.Sum(nil)

	if r.signMode == RsaSignModePSS {
		err = rsa.VerifyPSS(r.publicKey, crypto.SHA256, hashed, signatureBytes, nil)
	} else {
		err = rsa.VerifyPKCS1v15(r.publicKey, crypto.SHA256, hashed, signatureBytes)
	}

	return err == nil, nil
}
