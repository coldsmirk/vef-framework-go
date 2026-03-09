package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// TestBcryptEncoderEncode tests the Encode method of BcryptEncoder.
func TestBcryptEncoderEncode(t *testing.T) {
	encoder := NewBcryptEncoder()

	t.Run("BasicEncoding", func(t *testing.T) {
		password := "testpassword123"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding should succeed")
		assert.NotEmpty(t, encoded, "Encoded password should not be empty")
		assert.NotEqual(t, password, encoded, "Encoded password should differ from plaintext")
		assert.True(t, strings.HasPrefix(encoded, "$2"), "Encoded password should start with $2")
		assert.Len(t, encoded, 60, "Encoded password should be 60 characters")
	})

	t.Run("DifferentPasswordsDifferentHashes", func(t *testing.T) {
		hash1, err1 := encoder.Encode("password123")
		hash2, err2 := encoder.Encode("password456")

		require.NoError(t, err1, "First encoding should succeed")
		require.NoError(t, err2, "Second encoding should succeed")
		assert.NotEqual(t, hash1, hash2, "Different passwords should produce different hashes")
	})

	t.Run("SamePasswordDifferentHashesDueToSalt", func(t *testing.T) {
		password := "samepassword"

		hash1, err1 := encoder.Encode(password)
		hash2, err2 := encoder.Encode(password)

		require.NoError(t, err1, "First encoding should succeed")
		require.NoError(t, err2, "Second encoding should succeed")
		assert.NotEqual(t, hash1, hash2, "Same password should produce different hashes due to random salt")
	})

	t.Run("EmptyPassword", func(t *testing.T) {
		encoded, err := encoder.Encode("")

		require.NoError(t, err, "Encoding empty password should succeed")
		assert.NotEmpty(t, encoded, "Encoded empty password should not be empty")
	})

	t.Run("LongPassword", func(t *testing.T) {
		password := strings.Repeat("a", 1000)

		encoded, err := encoder.Encode(password)

		assert.Error(t, err, "Should return error for password exceeding bcrypt limit")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		password := "!@#$%^&*()_+-=[]{}|;:,.<>?"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding special characters should succeed")
		assert.NotEmpty(t, encoded, "Encoded password should not be empty")
		assert.NotEqual(t, password, encoded, "Encoded password should differ from plaintext")
	})

	t.Run("UnicodeCharacters", func(t *testing.T) {
		password := "密码测试123🔒"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding unicode password should succeed")
		assert.NotEmpty(t, encoded, "Encoded unicode password should not be empty")
		assert.NotEqual(t, password, encoded, "Encoded password should differ from plaintext")
	})

	t.Run("CustomCost", func(t *testing.T) {
		customEncoder := NewBcryptEncoder(WithBcryptCost(12))

		encoded, err := customEncoder.Encode("password")

		require.NoError(t, err, "Encoding with custom cost should succeed")
		cost, err := bcrypt.Cost([]byte(encoded))
		require.NoError(t, err, "Extracting cost from hash should succeed")
		assert.Equal(t, 12, cost, "Cost should match custom value")
	})

	t.Run("InvalidCostTooLow", func(t *testing.T) {
		invalidEncoder := NewBcryptEncoder(WithBcryptCost(3))

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrInvalidCost, "Should return ErrInvalidCost for cost < 4")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("InvalidCostTooHigh", func(t *testing.T) {
		invalidEncoder := NewBcryptEncoder(WithBcryptCost(32))

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrInvalidCost, "Should return ErrInvalidCost for cost > 31")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})
}

// TestBcryptEncoderMatches tests the Matches method of BcryptEncoder.
func TestBcryptEncoderMatches(t *testing.T) {
	encoder := NewBcryptEncoder()

	t.Run("MatchesCorrectPassword", func(t *testing.T) {
		password := "testpassword123"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match correct password")
	})

	t.Run("RejectsIncorrectPassword", func(t *testing.T) {
		correctPassword := "correctpassword"
		incorrectPassword := "incorrectpassword"
		encoded, err := encoder.Encode(correctPassword)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(incorrectPassword, encoded)

		assert.False(t, result, "Should reject incorrect password")
	})

	t.Run("MatchesEmptyPassword", func(t *testing.T) {
		password := ""
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match empty password")
	})

	t.Run("RejectsEmptyPasswordAgainstNonEmptyHash", func(t *testing.T) {
		encoded, err := encoder.Encode("nonemptypassword")
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches("", encoded)

		assert.False(t, result, "Should reject empty password against non-empty hash")
	})

	t.Run("MatchesSpecialCharacters", func(t *testing.T) {
		password := "!@#$%^&*()_+-=[]{}|;:,.<>?"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match special characters password")
	})

	t.Run("MatchesUnicodeCharacters", func(t *testing.T) {
		password := "密码测试123🔒"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match unicode password")
	})

	t.Run("RejectsInvalidHashFormat", func(t *testing.T) {
		result := encoder.Matches("password", "invalid_hash_format")

		assert.False(t, result, "Should reject invalid hash format")
	})

	t.Run("CaseSensitive", func(t *testing.T) {
		password := "TestPassword"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		assert.False(t, encoder.Matches("TESTPASSWORD", encoded), "Should reject uppercase password")
		assert.False(t, encoder.Matches("testpassword", encoded), "Should reject lowercase password")
		assert.True(t, encoder.Matches("TestPassword", encoded), "Should match original case")
	})

	t.Run("CrossVerification", func(t *testing.T) {
		password1 := "password1"
		password2 := "password2"

		hash1, err1 := encoder.Encode(password1)
		hash2, err2 := encoder.Encode(password2)

		require.NoError(t, err1, "First encoding should succeed")
		require.NoError(t, err2, "Second encoding should succeed")

		assert.True(t, encoder.Matches(password1, hash1), "Should match password1 with hash1")
		assert.True(t, encoder.Matches(password2, hash2), "Should match password2 with hash2")
		assert.False(t, encoder.Matches(password1, hash2), "Should reject password1 with hash2")
		assert.False(t, encoder.Matches(password2, hash1), "Should reject password2 with hash1")
	})

	t.Run("MatchesAcrossDifferentCostEncoders", func(t *testing.T) {
		encoder10 := NewBcryptEncoder(WithBcryptCost(10))
		encoder12 := NewBcryptEncoder(WithBcryptCost(12))

		password := "password"
		hash10, err := encoder10.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		assert.True(t, encoder12.Matches(password, hash10), "Should match password across different cost encoders")
	})
}

// TestBcryptEncoderUpgradeEncoding tests the UpgradeEncoding method of BcryptEncoder.
func TestBcryptEncoderUpgradeEncoding(t *testing.T) {
	t.Run("NeedsUpgradeWhenCostIncreased", func(t *testing.T) {
		encoder10 := NewBcryptEncoder(WithBcryptCost(10))
		encoder12 := NewBcryptEncoder(WithBcryptCost(12))

		hash10, err := encoder10.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := encoder12.UpgradeEncoding(hash10)

		assert.True(t, needsUpgrade, "Should need upgrade when cost increased")
	})

	t.Run("NoUpgradeWhenCostSame", func(t *testing.T) {
		encoder := NewBcryptEncoder(WithBcryptCost(10))

		hash, err := encoder.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := encoder.UpgradeEncoding(hash)

		assert.False(t, needsUpgrade, "Should not need upgrade when cost is same")
	})

	t.Run("NoUpgradeWhenCostDecreased", func(t *testing.T) {
		encoder12 := NewBcryptEncoder(WithBcryptCost(12))
		encoder10 := NewBcryptEncoder(WithBcryptCost(10))

		hash12, err := encoder12.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := encoder10.UpgradeEncoding(hash12)

		assert.False(t, needsUpgrade, "Should not need upgrade when cost decreased")
	})

	t.Run("InvalidHashFormat", func(t *testing.T) {
		encoder := NewBcryptEncoder()

		needsUpgrade := encoder.UpgradeEncoding("invalid_hash")

		assert.False(t, needsUpgrade, "Should return false for invalid hash format")
	})
}
