package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScryptEncoderEncode tests the Encode method of ScryptEncoder.
func TestScryptEncoderEncode(t *testing.T) {
	encoder := NewScryptEncoder()

	t.Run("BasicEncoding", func(t *testing.T) {
		password := "testpassword123"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding should succeed")
		assert.NotEmpty(t, encoded, "Encoded password should not be empty")
		assert.NotEqual(t, password, encoded, "Encoded password should differ from plaintext")
		assert.True(t, strings.HasPrefix(encoded, "$scrypt$"), "Encoded password should start with $scrypt$")
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

	t.Run("UnicodeCharacters", func(t *testing.T) {
		password := "密码测试123🔒"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding unicode password should succeed")
		assert.NotEmpty(t, encoded, "Encoded unicode password should not be empty")
	})

	t.Run("CustomParameters", func(t *testing.T) {
		customEncoder := NewScryptEncoder(
			WithScryptN(65536),
			WithScryptR(16),
			WithScryptP(2),
		)

		encoded, err := customEncoder.Encode("password")

		require.NoError(t, err, "Encoding with custom parameters should succeed")
		assert.Contains(t, encoded, "n=65536,r=16,p=2", "Encoded password should contain custom parameters")
	})

	t.Run("InvalidN", func(t *testing.T) {
		invalidEncoder := NewScryptEncoder(WithScryptN(1))

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrInvalidIterations, "Should return ErrInvalidIterations for invalid N")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("InvalidR", func(t *testing.T) {
		invalidEncoder := NewScryptEncoder(WithScryptR(0))

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrInvalidIterations, "Should return ErrInvalidIterations for invalid R")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("InvalidP", func(t *testing.T) {
		invalidEncoder := NewScryptEncoder(WithScryptP(0))

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrInvalidParallelism, "Should return ErrInvalidParallelism for invalid P")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})
}

// TestScryptEncoderMatches tests the Matches method of ScryptEncoder.
func TestScryptEncoderMatches(t *testing.T) {
	encoder := NewScryptEncoder()

	t.Run("MatchesCorrectPassword", func(t *testing.T) {
		password := "testpassword123"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match correct password")
	})

	t.Run("RejectsIncorrectPassword", func(t *testing.T) {
		encoded, err := encoder.Encode("correctpassword")
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches("incorrectpassword", encoded)

		assert.False(t, result, "Should reject incorrect password")
	})

	t.Run("MatchesEmptyPassword", func(t *testing.T) {
		encoded, err := encoder.Encode("")
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches("", encoded)

		assert.True(t, result, "Should match empty password")
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
}

// TestScryptEncoderUpgradeEncoding tests the UpgradeEncoding method of ScryptEncoder.
func TestScryptEncoderUpgradeEncoding(t *testing.T) {
	t.Run("NeedsUpgradeWhenParametersIncreased", func(t *testing.T) {
		lowEncoder := NewScryptEncoder(
			WithScryptN(16384),
			WithScryptR(4),
			WithScryptP(1),
		)
		highEncoder := NewScryptEncoder(
			WithScryptN(32768),
			WithScryptR(8),
			WithScryptP(1),
		)

		lowHash, err := lowEncoder.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := highEncoder.UpgradeEncoding(lowHash)

		assert.True(t, needsUpgrade, "Should need upgrade when parameters increased")
	})

	t.Run("NoUpgradeWhenParametersSame", func(t *testing.T) {
		encoder := NewScryptEncoder()

		hash, err := encoder.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := encoder.UpgradeEncoding(hash)

		assert.False(t, needsUpgrade, "Should not need upgrade when parameters are same")
	})

	t.Run("InvalidHashFormat", func(t *testing.T) {
		encoder := NewScryptEncoder()

		needsUpgrade := encoder.UpgradeEncoding("invalid_hash")

		assert.False(t, needsUpgrade, "Should return false for invalid hash format")
	})
}
