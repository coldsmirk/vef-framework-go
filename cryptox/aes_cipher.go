package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

type AESMode string

const (
	AesModeCbc AESMode = "CBC"
	AesModeGcm AESMode = "GCM"
)

type aesCipher struct {
	key  []byte
	iv   []byte
	mode AESMode
}

type AESOption func(*aesCipher)

func WithAESIv(iv []byte) AESOption {
	return func(c *aesCipher) {
		c.iv = iv
	}
}

func WithAESMode(mode AESMode) AESOption {
	return func(c *aesCipher) {
		c.mode = mode
	}
}

func NewAES(key []byte, opts ...AESOption) (Cipher, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("%w: %d bytes (must be 16, 24, or 32)", ErrInvalidAesKeySize, len(key))
	}

	cipher := &aesCipher{
		key:  key,
		mode: AesModeGcm,
	}

	for _, opt := range opts {
		opt(cipher)
	}

	if cipher.mode == AesModeCbc {
		if len(cipher.iv) != aes.BlockSize {
			return nil, fmt.Errorf("%w: %d bytes (must be %d)", ErrInvalidIvSizeCbc, len(cipher.iv), aes.BlockSize)
		}
	}

	return cipher, nil
}

func NewAESFromHex(keyHex string, opts ...AESOption) (Cipher, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key from hex: %w", err)
	}

	return NewAES(key, opts...)
}

func NewAESFromBase64(keyBase64 string, opts ...AESOption) (Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key from base64: %w", err)
	}

	return NewAES(key, opts...)
}

func (a *aesCipher) Encrypt(plaintext string) (string, error) {
	if a.mode == AesModeGcm {
		return a.encryptGCM(plaintext)
	}

	return a.encryptCBC(plaintext)
}

func (a *aesCipher) Decrypt(ciphertext string) (string, error) {
	if a.mode == AesModeGcm {
		return a.decryptGCM(ciphertext)
	}

	return a.decryptCBC(ciphertext)
}

func (a *aesCipher) encryptCBC(plaintext string) (string, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	paddedData := pkcs7Padding([]byte(plaintext), aes.BlockSize)

	ciphertext := make([]byte, len(paddedData))
	mode := cipher.NewCBCEncrypter(block, a.iv)
	mode.CryptBlocks(ciphertext, paddedData)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (a *aesCipher) decryptCBC(ciphertext string) (string, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(encryptedData)%aes.BlockSize != 0 {
		return "", ErrCiphertextNotMultipleOfBlock
	}

	plaintext := make([]byte, len(encryptedData))
	mode := cipher.NewCBCDecrypter(block, a.iv)
	mode.CryptBlocks(plaintext, encryptedData)

	unpaddedData, err := pkcs7Unpadding(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to remove padding: %w", err)
	}

	return string(unpaddedData), nil
}

func (a *aesCipher) encryptGCM(plaintext string) (string, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (a *aesCipher) decryptGCM(ciphertext string) (string, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return "", ErrCiphertextTooShort
	}

	nonce, ciphertextBytes := encryptedData[:nonceSize], encryptedData[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt and verify: %w", err)
	}

	return string(plaintext), nil
}

func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padByte := byte(padding)

	result := make([]byte, len(data)+padding)
	copy(result, data)

	for i := len(data); i < len(result); i++ {
		result[i] = padByte
	}

	return result
}

func pkcs7Unpadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, ErrDataEmpty
	}

	padding := int(data[length-1])
	if padding > length || padding > aes.BlockSize {
		return nil, ErrInvalidPadding
	}

	for i := range padding {
		if data[length-1-i] != byte(padding) {
			return nil, ErrInvalidPadding
		}
	}

	return data[:length-padding], nil
}
