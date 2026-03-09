package cryptox

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/x509"
)

type SM2Cipher struct {
	privateKey *sm2.PrivateKey
	publicKey  *sm2.PublicKey
}

func NewSM2(privateKey *sm2.PrivateKey, publicKey *sm2.PublicKey) (CipherSigner, error) {
	if privateKey == nil && publicKey == nil {
		return nil, ErrAtLeastOneKeyRequired
	}

	if publicKey == nil && privateKey != nil {
		publicKey = &privateKey.PublicKey
	}

	return &SM2Cipher{
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

func NewSM2FromPEM(privatePEM, publicPEM []byte) (CipherSigner, error) {
	var (
		privateKey *sm2.PrivateKey
		publicKey  *sm2.PublicKey
		err        error
	)

	if privatePEM != nil {
		privateKey, err = parseSM2PrivateKeyFromPEM(privatePEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	if publicPEM != nil {
		publicKey, err = parseSM2PublicKeyFromPEM(publicPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return NewSM2(privateKey, publicKey)
}

func NewSM2FromHex(privateKeyHex, publicKeyHex string) (CipherSigner, error) {
	var (
		privateKey *sm2.PrivateKey
		publicKey  *sm2.PublicKey
	)

	if privateKeyHex != "" {
		if keyBytes, err := hex.DecodeString(privateKeyHex); err != nil {
			return nil, fmt.Errorf("failed to decode private key from hex: %w", err)
		} else if privateKey, err = x509.ParseSm2PrivateKey(keyBytes); err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	if publicKeyHex != "" {
		if keyBytes, err := hex.DecodeString(publicKeyHex); err != nil {
			return nil, fmt.Errorf("failed to decode public key from hex: %w", err)
		} else if publicKey, err = x509.ParseSm2PublicKey(keyBytes); err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return NewSM2(privateKey, publicKey)
}

func NewSM2FromBase64(privateKeyBase64, publicKeyBase64 string) (CipherSigner, error) {
	var (
		privateKey *sm2.PrivateKey
		publicKey  *sm2.PublicKey
	)

	if privateKeyBase64 != "" {
		if keyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64); err != nil {
			return nil, fmt.Errorf("failed to decode private key from base64: %w", err)
		} else if privateKey, err = x509.ParseSm2PrivateKey(keyBytes); err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	if publicKeyBase64 != "" {
		if keyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64); err != nil {
			return nil, fmt.Errorf("failed to decode public key from base64: %w", err)
		} else if publicKey, err = x509.ParseSm2PublicKey(keyBytes); err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return NewSM2(privateKey, publicKey)
}

func (s *SM2Cipher) Encrypt(plaintext string) (string, error) {
	if s.publicKey == nil {
		return "", ErrPublicKeyRequiredForEncrypt
	}

	ciphertext, err := sm2.Encrypt(s.publicKey, []byte(plaintext), rand.Reader, sm2.C1C3C2)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *SM2Cipher) Decrypt(ciphertext string) (string, error) {
	if s.privateKey == nil {
		return "", ErrPrivateKeyRequiredForDecrypt
	}

	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	plaintext, err := sm2.Decrypt(s.privateKey, encryptedData, sm2.C1C3C2)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

func parseSM2PrivateKeyFromPEM(pemData []byte) (*sm2.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrFailedDecodePemBlock
	}

	return x509.ParseSm2PrivateKey(block.Bytes)
}

func parseSM2PublicKeyFromPEM(pemData []byte) (*sm2.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrFailedDecodePemBlock
	}

	return x509.ParseSm2PublicKey(block.Bytes)
}

func (s *SM2Cipher) Sign(data string) (string, error) {
	if s.privateKey == nil {
		return "", ErrPrivateKeyRequiredForSign
	}

	signature, err := s.privateKey.Sign(rand.Reader, []byte(data), nil)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

func (s *SM2Cipher) Verify(data, signature string) (bool, error) {
	if s.publicKey == nil {
		return false, ErrPublicKeyRequiredForVerify
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	valid := s.publicKey.Verify([]byte(data), signatureBytes)

	return valid, nil
}

var _ CipherSigner = (*SM2Cipher)(nil)
