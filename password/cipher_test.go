package password

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCipher is a simple mock cipher for testing.
type MockCipher struct {
	encryptFunc func(string) (string, error)
	decryptFunc func(string) (string, error)
}

func (m *MockCipher) Encrypt(plaintext string) (string, error) {
	if m.encryptFunc != nil {
		return m.encryptFunc(plaintext)
	}

	return base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (m *MockCipher) Decrypt(ciphertext string) (string, error) {
	if m.decryptFunc != nil {
		return m.decryptFunc(ciphertext)
	}

	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

// TestCipherEncoderEncode tests the Encode method of CipherEncoder.
func TestCipherEncoderEncode(t *testing.T) {
	cipher := &MockCipher{}
	bcryptEncoder := NewBcryptEncoder()
	encoder := NewCipherEncoder(cipher, bcryptEncoder)

	t.Run("EncodesAfterDecryption", func(t *testing.T) {
		password := "testpassword"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Cipher encryption should succeed")

		encoded, err := encoder.Encode(encryptedPassword)

		require.NoError(t, err, "Encoding should succeed")
		assert.NotEmpty(t, encoded, "Encoded password should not be empty")
		assert.True(t, encoder.Matches(encryptedPassword, encoded), "Should match encrypted password")
	})

	t.Run("ErrorWhenCipherMissing", func(t *testing.T) {
		invalidEncoder := NewCipherEncoder(nil, bcryptEncoder)

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrCipherRequired, "Should return ErrCipherRequired")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("ErrorWhenEncoderMissing", func(t *testing.T) {
		invalidEncoder := NewCipherEncoder(cipher, nil)

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrEncoderRequired, "Should return ErrEncoderRequired")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("ErrorWhenDecryptionFails", func(t *testing.T) {
		failingCipher := &MockCipher{
			decryptFunc: func(_ string) (string, error) {
				return "", errors.New("decryption failed")
			},
		}
		failingEncoder := NewCipherEncoder(failingCipher, bcryptEncoder)

		encoded, err := failingEncoder.Encode("encrypted")

		assert.Error(t, err, "Should return decryption error")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("WorksWithDifferentEncoders", func(t *testing.T) {
		password := "testpassword"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Cipher encryption should succeed")

		argon2Encoder := NewCipherEncoder(cipher, NewArgon2Encoder())
		pbkdf2Encoder := NewCipherEncoder(cipher, NewPbkdf2Encoder())

		argon2Hash, err := argon2Encoder.Encode(encryptedPassword)
		require.NoError(t, err, "Argon2 encoding should succeed")
		pbkdf2Hash, err := pbkdf2Encoder.Encode(encryptedPassword)
		require.NoError(t, err, "PBKDF2 encoding should succeed")

		assert.NotEqual(t, argon2Hash, pbkdf2Hash, "Different encoders should produce different hashes")
	})

	t.Run("EmptyPassword", func(t *testing.T) {
		emptyEncrypted, err := cipher.Encrypt("")
		require.NoError(t, err, "Encrypting empty password should succeed")

		encoded, err := encoder.Encode(emptyEncrypted)

		require.NoError(t, err, "Encoding empty password should succeed")
		assert.NotEmpty(t, encoded, "Encoded empty password should not be empty")
	})

	t.Run("UnicodePassword", func(t *testing.T) {
		password := "密码🔒"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Encrypting unicode password should succeed")

		encoded, err := encoder.Encode(encryptedPassword)

		require.NoError(t, err, "Encoding unicode password should succeed")
		assert.NotEmpty(t, encoded, "Encoded unicode password should not be empty")
	})
}

// TestCipherEncoderMatches tests the Matches method of CipherEncoder.
func TestCipherEncoderMatches(t *testing.T) {
	cipher := &MockCipher{}
	bcryptEncoder := NewBcryptEncoder()
	encoder := NewCipherEncoder(cipher, bcryptEncoder)

	t.Run("MatchesCorrectPassword", func(t *testing.T) {
		password := "testpassword"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Cipher encryption should succeed")

		encoded, err := encoder.Encode(encryptedPassword)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(encryptedPassword, encoded)

		assert.True(t, result, "Should match correct encrypted password")
	})

	t.Run("RejectsIncorrectPassword", func(t *testing.T) {
		password := "correctpassword"
		wrongPassword := "wrongpassword"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Cipher encryption should succeed")
		encryptedWrong, err := cipher.Encrypt(wrongPassword)
		require.NoError(t, err, "Cipher encryption should succeed")

		encoded, err := encoder.Encode(encryptedPassword)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(encryptedWrong, encoded)

		assert.False(t, result, "Should reject incorrect password")
	})

	t.Run("ReturnsFalseWhenCipherMissing", func(t *testing.T) {
		invalidEncoder := NewCipherEncoder(nil, bcryptEncoder)

		result := invalidEncoder.Matches("password", "hash")

		assert.False(t, result, "Should return false when cipher missing")
	})

	t.Run("ReturnsFalseWhenEncoderMissing", func(t *testing.T) {
		invalidEncoder := NewCipherEncoder(cipher, nil)

		result := invalidEncoder.Matches("password", "hash")

		assert.False(t, result, "Should return false when encoder missing")
	})

	t.Run("ReturnsFalseWhenDecryptionFails", func(t *testing.T) {
		failingCipher := &MockCipher{
			decryptFunc: func(_ string) (string, error) {
				return "", errors.New("decryption failed")
			},
		}
		failingEncoder := NewCipherEncoder(failingCipher, bcryptEncoder)

		result := failingEncoder.Matches("encrypted", "somehash")

		assert.False(t, result, "Should return false when decryption fails")
	})

	t.Run("MatchesEmptyPassword", func(t *testing.T) {
		emptyEncrypted, err := cipher.Encrypt("")
		require.NoError(t, err, "Encrypting empty password should succeed")

		encoded, err := encoder.Encode(emptyEncrypted)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(emptyEncrypted, encoded)

		assert.True(t, result, "Should match empty password")
	})

	t.Run("MatchesUnicodePassword", func(t *testing.T) {
		password := "密码🔒"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Encrypting unicode password should succeed")

		encoded, err := encoder.Encode(encryptedPassword)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(encryptedPassword, encoded)

		assert.True(t, result, "Should match unicode password")
	})

	t.Run("CaseSensitive", func(t *testing.T) {
		password := "TestPassword"
		wrongCase := "testpassword"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Encrypting password should succeed")
		encryptedWrong, err := cipher.Encrypt(wrongCase)
		require.NoError(t, err, "Encrypting wrong case should succeed")

		encoded, err := encoder.Encode(encryptedPassword)
		require.NoError(t, err, "Encoding should succeed")

		assert.True(t, encoder.Matches(encryptedPassword, encoded), "Should match original case")
		assert.False(t, encoder.Matches(encryptedWrong, encoded), "Should reject different case")
	})
}

// TestCipherEncoderUpgradeEncoding tests the UpgradeEncoding method of CipherEncoder.
func TestCipherEncoderUpgradeEncoding(t *testing.T) {
	cipher := &MockCipher{}

	t.Run("DelegatesUpgradeToEncoder", func(t *testing.T) {
		lowCostEncoder := NewBcryptEncoder(WithBcryptCost(4))
		highCostEncoder := NewBcryptEncoder(WithBcryptCost(12))
		cipherEncoder := NewCipherEncoder(cipher, highCostEncoder)

		password := "testpassword"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Cipher encryption should succeed")

		lowHash, err := lowCostEncoder.Encode(password)
		require.NoError(t, err, "Low cost encoding should succeed")

		highHash, err := cipherEncoder.Encode(encryptedPassword)
		require.NoError(t, err, "High cost encoding should succeed")

		assert.True(t, cipherEncoder.UpgradeEncoding(lowHash), "Should need upgrade for low cost hash")
		assert.False(t, cipherEncoder.UpgradeEncoding(highHash), "Should not need upgrade for high cost hash")
	})

	t.Run("ReturnsFalseWhenEncoderMissing", func(t *testing.T) {
		invalidEncoder := NewCipherEncoder(cipher, nil)

		result := invalidEncoder.UpgradeEncoding("somehash")

		assert.False(t, result, "Should return false when encoder missing")
	})

	t.Run("WorksWithDifferentEncoderTypes", func(t *testing.T) {
		md5Encoder := NewCipherEncoder(cipher, NewMd5Encoder())
		bcryptEncoder := NewCipherEncoder(cipher, NewBcryptEncoder())

		md5Hash := "5f4dcc3b5aa765d61d8327deb882cf99"
		password := "testpassword"
		encryptedPassword, err := cipher.Encrypt(password)
		require.NoError(t, err, "Cipher encryption should succeed")

		bcryptHash, err := bcryptEncoder.Encode(encryptedPassword)
		require.NoError(t, err, "Bcrypt encoding should succeed")

		assert.True(t, md5Encoder.UpgradeEncoding(md5Hash), "MD5 encoder should always need upgrade")
		assert.False(t, bcryptEncoder.UpgradeEncoding(md5Hash), "Bcrypt encoder returns false for non-bcrypt hash")
		assert.False(t, bcryptEncoder.UpgradeEncoding(bcryptHash), "Bcrypt encoder should not need upgrade for same cost")
	})
}
