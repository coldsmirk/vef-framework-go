package cryptox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
)

type ECDSACurve string

const (
	EcdsaCurveP224 ECDSACurve = "P224"
	EcdsaCurveP256 ECDSACurve = "P256"
	EcdsaCurveP384 ECDSACurve = "P384"
	EcdsaCurveP521 ECDSACurve = "P521"
)

type ecdsaCipher struct {
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
}

type ECDSAOption func(*ecdsaCipher)

func NewECDSA(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey, opts ...ECDSAOption) (Signer, error) {
	if privateKey == nil && publicKey == nil {
		return nil, ErrAtLeastOneKeyRequired
	}

	cipher := &ecdsaCipher{
		privateKey: privateKey,
		publicKey:  publicKey,
	}

	for _, opt := range opts {
		opt(cipher)
	}

	if publicKey == nil && privateKey != nil {
		cipher.publicKey = &privateKey.PublicKey
	}

	return cipher, nil
}

func NewECDSAFromPem(privatePem, publicPem []byte, opts ...ECDSAOption) (Signer, error) {
	var (
		privateKey *ecdsa.PrivateKey
		publicKey  *ecdsa.PublicKey
		err        error
	)

	if privatePem != nil {
		if privateKey, err = parseECDSAPrivateKeyFromPem(privatePem); err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	if publicPem != nil {
		if publicKey, err = parseECDSAPublicKeyFromPem(publicPem); err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return NewECDSA(privateKey, publicKey, opts...)
}

func parseECDSAKeysFromBytes(privateKeyBytes, publicKeyBytes []byte) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	var (
		privateKey *ecdsa.PrivateKey
		publicKey  *ecdsa.PublicKey
		err        error
	)

	if len(privateKeyBytes) > 0 {
		privateKey, err = x509.ParseECPrivateKey(privateKeyBytes)
		if err != nil {
			key, pkcs8Err := x509.ParsePKCS8PrivateKey(privateKeyBytes)
			if pkcs8Err != nil {
				return nil, nil, fmt.Errorf("failed to parse private key (tried EC and PKCS8): %w", err)
			}

			var ok bool

			privateKey, ok = key.(*ecdsa.PrivateKey)
			if !ok {
				return nil, nil, ErrNotEcdsaPrivateKey
			}
		}
	}

	if len(publicKeyBytes) > 0 {
		key, err := x509.ParsePKIXPublicKey(publicKeyBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse public key: %w", err)
		}

		var ok bool

		publicKey, ok = key.(*ecdsa.PublicKey)
		if !ok {
			return nil, nil, ErrNotEcdsaPublicKey
		}
	}

	return privateKey, publicKey, nil
}

func NewECDSAFromHex(privateKeyHex, publicKeyHex string, opts ...ECDSAOption) (Signer, error) {
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

	privateKey, publicKey, err := parseECDSAKeysFromBytes(privateBytes, publicBytes)
	if err != nil {
		return nil, err
	}

	return NewECDSA(privateKey, publicKey, opts...)
}

func NewECDSAFromBase64(privateKeyBase64, publicKeyBase64 string, opts ...ECDSAOption) (Signer, error) {
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

	privateKey, publicKey, err := parseECDSAKeysFromBytes(privateBytes, publicBytes)
	if err != nil {
		return nil, err
	}

	return NewECDSA(privateKey, publicKey, opts...)
}

func GenerateECDSAKey(curve ECDSACurve) (*ecdsa.PrivateKey, error) {
	curves := map[ECDSACurve]elliptic.Curve{
		EcdsaCurveP224: elliptic.P224(),
		EcdsaCurveP256: elliptic.P256(),
		EcdsaCurveP384: elliptic.P384(),
		EcdsaCurveP521: elliptic.P521(),
	}

	ellipticCurve, ok := curves[curve]
	if !ok {
		ellipticCurve = elliptic.P256()
	}

	return ecdsa.GenerateKey(ellipticCurve, rand.Reader)
}

type ecdsaSignature struct {
	R, S *big.Int
}

func (e *ecdsaCipher) Sign(data string) (string, error) {
	if e.privateKey == nil {
		return "", ErrPrivateKeyRequiredForSign
	}

	hash := sha256.New()
	_, _ = hash.Write([]byte(data))
	hashed := hash.Sum(nil)

	r, s, err := ecdsa.Sign(rand.Reader, e.privateKey, hashed)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	signature, err := asn1.Marshal(ecdsaSignature{R: r, S: s})
	if err != nil {
		return "", fmt.Errorf("failed to marshal signature: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

func (e *ecdsaCipher) Verify(data, signature string) (bool, error) {
	if e.publicKey == nil {
		return false, ErrPublicKeyRequiredForVerify
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	var sig ecdsaSignature
	if _, err := asn1.Unmarshal(signatureBytes, &sig); err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	hash := sha256.New()
	_, _ = hash.Write([]byte(data))
	hashed := hash.Sum(nil)

	valid := ecdsa.Verify(e.publicKey, hashed, sig.R, sig.S)

	return valid, nil
}

func parseECDSAPrivateKeyFromPem(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrFailedDecodePemBlock
	}

	if block.Type == "EC PRIVATE KEY" {
		return x509.ParseECPrivateKey(block.Bytes)
	}

	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		ecdsaKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, ErrNotEcdsaPrivateKey
		}

		return ecdsaKey, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrUnsupportedPemType, block.Type)
}

func parseECDSAPublicKeyFromPem(pemData []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrFailedDecodePemBlock
	}

	if block.Type == "PUBLIC KEY" {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		ecdsaKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, ErrNotEcdsaPublicKey
		}

		return ecdsaKey, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrUnsupportedPemType, block.Type)
}

var _ Signer = (*ecdsaCipher)(nil)
