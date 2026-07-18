package cryptox

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/tjfoc/gmsm/sm4"
)

type SM4Mode string

const (
	Sm4ModeCbc SM4Mode = "CBC"
	Sm4ModeGcm SM4Mode = "GCM"
)

type sm4Cipher struct {
	key       []byte
	interopIV []byte
	mode      SM4Mode
}

type SM4Option func(*sm4Cipher)

// WithSM4Iv supplies a fixed IV for the CBC interop decrypt path only.
//
// It does NOT affect Encrypt: CBC encryption always generates a fresh random
// IV and prepends it to the ciphertext (layout: IV || ciphertext). The fixed
// IV configured here is consumed solely by DecryptWithFixedIV, which decrypts
// bare ciphertext produced by an external client that uses a constant IV.
func WithSM4Iv(iv []byte) SM4Option {
	return func(c *sm4Cipher) {
		c.interopIV = iv
	}
}

func WithSM4Mode(mode SM4Mode) SM4Option {
	return func(c *sm4Cipher) {
		c.mode = mode
	}
}

func NewSM4(key []byte, opts ...SM4Option) (Cipher, error) {
	if len(key) != sm4.BlockSize {
		return nil, fmt.Errorf("%w: %d bytes (must be %d)", ErrInvalidSM4KeySize, len(key), sm4.BlockSize)
	}

	cipher := &sm4Cipher{
		key:  key,
		mode: Sm4ModeGcm,
	}

	for _, opt := range opts {
		opt(cipher)
	}

	if len(cipher.interopIV) != 0 && len(cipher.interopIV) != sm4.BlockSize {
		return nil, fmt.Errorf("%w: %d bytes (must be %d)", ErrInvalidIVSizeCBC, len(cipher.interopIV), sm4.BlockSize)
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
	if s.mode == Sm4ModeGcm {
		return s.encryptGCM(plaintext)
	}

	return s.encryptCBC(plaintext)
}

func (s *sm4Cipher) Decrypt(ciphertext string) (string, error) {
	if s.mode == Sm4ModeGcm {
		return s.decryptGCM(ciphertext)
	}

	return s.decryptCBC(ciphertext)
}

func (s *sm4Cipher) encryptCBC(plaintext string) (string, error) {
	block, err := sm4.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create SM4 cipher: %w", err)
	}

	iv := make([]byte, sm4.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	paddedData := pkcs7Padding([]byte(plaintext), sm4.BlockSize)

	ciphertext := make([]byte, len(iv)+len(paddedData))
	copy(ciphertext, iv)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext[len(iv):], paddedData)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *sm4Cipher) decryptCBC(ciphertext string) (string, error) {
	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(encryptedData) < sm4.BlockSize {
		return "", ErrCiphertextTooShort
	}

	iv, payload := encryptedData[:sm4.BlockSize], encryptedData[sm4.BlockSize:]

	return s.cbcDecrypt(iv, payload)
}

// DecryptWithFixedIV decrypts bare CBC ciphertext (no prepended IV) using the
// fixed IV configured via WithSM4Iv. It exists solely for interop with an
// external client that encrypts with a constant IV; native VEF ciphertext from
// Encrypt must be read back with Decrypt, which derives the IV from the prefix.
func (s *sm4Cipher) DecryptWithFixedIV(ciphertext string) (string, error) {
	if len(s.interopIV) != sm4.BlockSize {
		return "", fmt.Errorf("%w: %d bytes (must be %d)", ErrInvalidIVSizeCBC, len(s.interopIV), sm4.BlockSize)
	}

	encryptedData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	return s.cbcDecrypt(s.interopIV, encryptedData)
}

func (s *sm4Cipher) cbcDecrypt(iv, payload []byte) (string, error) {
	if len(payload) == 0 || len(payload)%sm4.BlockSize != 0 {
		return "", ErrCiphertextNotMultipleOfBlock
	}

	block, err := sm4.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create SM4 cipher: %w", err)
	}

	plaintext := make([]byte, len(payload))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, payload)

	unpaddedData, err := pkcs7Unpadding(plaintext, sm4.BlockSize)
	if err != nil {
		return "", fmt.Errorf("failed to remove padding: %w", err)
	}

	return string(unpaddedData), nil
}

func (s *sm4Cipher) encryptGCM(plaintext string) (string, error) {
	block, err := sm4.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create SM4 cipher: %w", err)
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

func (s *sm4Cipher) decryptGCM(ciphertext string) (string, error) {
	block, err := sm4.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create SM4 cipher: %w", err)
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
