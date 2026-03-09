package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPbkdf2EncoderEncode tests the Encode method of Pbkdf2Encoder.
func TestPbkdf2EncoderEncode(t *testing.T) {
	encoder := NewPbkdf2Encoder()

	t.Run("BasicEncoding", func(t *testing.T) {
		password := "testpassword123"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding should succeed")
		assert.NotEmpty(t, encoded, "Encoded password should not be empty")
		assert.NotEqual(t, password, encoded, "Encoded password should differ from plaintext")
		assert.True(t, strings.HasPrefix(encoded, "$pbkdf2-sha256$"), "Encoded password should start with $pbkdf2-sha256$")
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

	t.Run("CustomIterations", func(t *testing.T) {
		customEncoder := NewPbkdf2Encoder(WithPbkdf2Iterations(600000))

		encoded, err := customEncoder.Encode("password")

		require.NoError(t, err, "Encoding with custom iterations should succeed")
		assert.Contains(t, encoded, "i=600000", "Encoded password should contain custom iteration count")
	})

	t.Run("SHA512HashFunction", func(t *testing.T) {
		sha512Encoder := NewPbkdf2Encoder(WithPbkdf2HashFunction("sha512"))

		encoded, err := sha512Encoder.Encode("password")

		require.NoError(t, err, "Encoding with SHA512 should succeed")
		assert.Contains(t, encoded, "$pbkdf2-sha512$", "Encoded password should contain sha512 identifier")
	})

	t.Run("InvalidIterations", func(t *testing.T) {
		invalidEncoder := NewPbkdf2Encoder(WithPbkdf2Iterations(0))

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrInvalidIterations, "Should return ErrInvalidIterations for iterations < 1")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})

	t.Run("InvalidHashFunction", func(t *testing.T) {
		invalidEncoder := NewPbkdf2Encoder(WithPbkdf2HashFunction("md5"))

		encoded, err := invalidEncoder.Encode("password")

		assert.ErrorIs(t, err, ErrInvalidHashFormat, "Should return ErrInvalidHashFormat for unsupported hash function")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})
}

// TestPbkdf2EncoderMatches tests the Matches method of Pbkdf2Encoder.
func TestPbkdf2EncoderMatches(t *testing.T) {
	encoder := NewPbkdf2Encoder()

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

	t.Run("MatchesAcrossDifferentHashFunctions", func(t *testing.T) {
		sha256Encoder := NewPbkdf2Encoder(WithPbkdf2HashFunction("sha256"))
		sha512Encoder := NewPbkdf2Encoder(WithPbkdf2HashFunction("sha512"))

		password := "password"
		sha256Hash, err := sha256Encoder.Encode(password)
		require.NoError(t, err, "SHA256 encoding should succeed")
		sha512Hash, err := sha512Encoder.Encode(password)
		require.NoError(t, err, "SHA512 encoding should succeed")

		assert.True(t, sha256Encoder.Matches(password, sha256Hash), "Should match SHA256 hash")
		assert.True(t, sha512Encoder.Matches(password, sha512Hash), "Should match SHA512 hash")
	})
}

// TestPbkdf2EncoderUpgradeEncoding tests the UpgradeEncoding method of Pbkdf2Encoder.
func TestPbkdf2EncoderUpgradeEncoding(t *testing.T) {
	t.Run("NeedsUpgradeWhenIterationsIncreased", func(t *testing.T) {
		lowEncoder := NewPbkdf2Encoder(WithPbkdf2Iterations(100000))
		highEncoder := NewPbkdf2Encoder(WithPbkdf2Iterations(310000))

		lowHash, err := lowEncoder.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := highEncoder.UpgradeEncoding(lowHash)

		assert.True(t, needsUpgrade, "Should need upgrade when iterations increased")
	})

	t.Run("NeedsUpgradeWhenHashFunctionChanged", func(t *testing.T) {
		sha256Encoder := NewPbkdf2Encoder(WithPbkdf2HashFunction("sha256"))
		sha512Encoder := NewPbkdf2Encoder(WithPbkdf2HashFunction("sha512"))

		sha256Hash, err := sha256Encoder.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := sha512Encoder.UpgradeEncoding(sha256Hash)

		assert.True(t, needsUpgrade, "Should need upgrade when hash function changed")
	})

	t.Run("NoUpgradeWhenParametersSame", func(t *testing.T) {
		encoder := NewPbkdf2Encoder()

		hash, err := encoder.Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := encoder.UpgradeEncoding(hash)

		assert.False(t, needsUpgrade, "Should not need upgrade when parameters are same")
	})

	t.Run("InvalidHashFormat", func(t *testing.T) {
		encoder := NewPbkdf2Encoder()

		needsUpgrade := encoder.UpgradeEncoding("invalid_hash")

		assert.False(t, needsUpgrade, "Should return false for invalid hash format")
	})
}
