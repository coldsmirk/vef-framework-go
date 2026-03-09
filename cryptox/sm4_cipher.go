package cryptox

import (
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/tjfoc/gmsm/sm4"
)

type SM4Mode string

const (
	SM4ModeCBC SM4Mode = "CBC"
	SM4ModeECB SM4Mode = "ECB"
)

type sm4Cipher struct {
	key  []byte
	iv   []byte
	mode SM4Mode
}

type SM4Option func(*sm4Cipher)

func WithSM4Iv(iv []byte) SM4Option {
	return func(c *sm4Cipher) {
		c.iv = iv
	}
}

func WithSM4Mode(mode SM4Mode) SM4Option {
	return func(c *sm4Cipher) {
		c.mode = mode
	}
}

func NewSM4(key []byte, opts ...SM4Option) (Cipher, error) {
	if len(key) != sm4.BlockSize {
		return nil, fmt.Errorf("%w: %d bytes (must be %d)", ErrInvalidSm4KeySize, len(key), sm4.BlockSize)
	}

	cipher := &sm4Cipher{
		key:  key,
		mode: SM4ModeCBC,
	}

	for _, opt := range opts {
		opt(cipher)
	}

	if cipher.mode == SM4ModeCBC {
		if len(cipher.iv) != sm4.BlockSize {
			return nil, fmt.Errorf("%w: %d bytes (must be %d)", ErrInvalidIvSizeCbc, len(cipher.iv), sm4.BlockSize)
		}
	}

	return cipher, nil
}

func NewSM4FromHex(keyHex string, opts ...SM4Option) (Cipher, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key from hex: %w", err)
	}

	return NewSM4(key, opts...)
}

func NewSM4FromBase64(keyBase64 string, opts ...SM4Option) (Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key from base64: %w", err)
	}

	return NewSM4(key, opts...)
}

func (s *sm4Cipher) Encrypt(plaintext string) (string, error) {
	if s.mode == SM4ModeECB {
		return s.encryptECB(plaintext)
	}

	return s.encryptCBC(plaintext)
}

func (s *sm4Cipher) Decrypt(ciphertext string) (string, error) {
	if s.mode == SM4ModeECB {
		return s.decryptECB(ciphertext)
	}

	return s.decryptCBC(ciphertext)
}

func (s *sm4Cipher) encryptECB(plaintext string) (string, error) {
	paddedData := pkcs7Padding([]byte(plaintext), sm4.BlockSize)

	ciphertext, err := sm4.Sm4Ecb(s.key, paddedData, true)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *sm4Cipher) decryptECB(ciphertext string) (string, error) {
	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	plaintext, err := sm4.Sm4Ecb(s.key, encryptedData, false)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	unpaddedData, err := pkcs7Unpadding(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to remove padding: %w", err)
	}

	return string(unpaddedData), nil
}

func (s *sm4Cipher) encryptCBC(plaintext string) (string, error) {
	block, err := sm4.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create SM4 cipher: %w", err)
	}

	paddedData := pkcs7Padding([]byte(plaintext), sm4.BlockSize)

	ciphertext := make([]byte, len(paddedData))
	mode := cipher.NewCBCEncrypter(block, s.iv)
	mode.CryptBlocks(ciphertext, paddedData)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *sm4Cipher) decryptCBC(ciphertext string) (string, error) {
	block, err := sm4.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create SM4 cipher: %w", err)
	}

	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(encryptedData)%sm4.BlockSize != 0 {
		return "", ErrCiphertextNotMultipleOfBlock
	}

	plaintext := make([]byte, len(encryptedData))
	mode := cipher.NewCBCDecrypter(block, s.iv)
	mode.CryptBlocks(plaintext, encryptedData)

	unpaddedData, err := pkcs7Unpadding(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to remove padding: %w", err)
	}

	return string(unpaddedData), nil
}

var _ Cipher = (*sm4Cipher)(nil)
