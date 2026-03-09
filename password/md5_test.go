package password

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMd5EncoderEncode tests the Encode method of MD5Encoder.
func TestMd5EncoderEncode(t *testing.T) {
	encoder := NewMd5Encoder()

	t.Run("BasicEncoding", func(t *testing.T) {
		password := "testpassword"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding should succeed")
		assert.NotEqual(t, password, encoded, "Encoded password should differ from plaintext")
		assert.Len(t, encoded, 32, "Encoded password should be 32 characters")
	})

	t.Run("DifferentPasswordsDifferentHashes", func(t *testing.T) {
		password1 := "password1"
		password2 := "password2"

		encoded1, err := encoder.Encode(password1)
		require.NoError(t, err, "First encoding should succeed")
		encoded2, err := encoder.Encode(password2)
		require.NoError(t, err, "Second encoding should succeed")

		assert.NotEqual(t, encoded1, encoded2, "Different passwords should produce different hashes")
	})

	t.Run("SamePasswordSameHash", func(t *testing.T) {
		password := "password"

		encoded1, err := encoder.Encode(password)
		require.NoError(t, err, "First encoding should succeed")
		encoded2, err := encoder.Encode(password)
		require.NoError(t, err, "Second encoding should succeed")

		assert.Equal(t, encoded1, encoded2, "Same password should produce same hash")
	})

	t.Run("EmptyPassword", func(t *testing.T) {
		encoded, err := encoder.Encode("")

		require.NoError(t, err, "Encoding empty password should succeed")
		assert.Len(t, encoded, 32, "Encoded empty password should be 32 characters")
		assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", encoded, "Empty password should match known MD5 hash")
	})

	t.Run("UnicodeCharacters", func(t *testing.T) {
		password := "密码🔒"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding unicode password should succeed")
		assert.Len(t, encoded, 32, "Encoded unicode password should be 32 characters")
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		password := "p@ssw0rd!#$%"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding special characters should succeed")
		assert.Len(t, encoded, 32, "Encoded password should be 32 characters")
	})

	t.Run("EncodesWithSalt", func(t *testing.T) {
		saltedEncoder := NewMd5Encoder(WithMd5Salt("mysalt"))
		password := "password"

		encoded, err := saltedEncoder.Encode(password)

		require.NoError(t, err, "Encoding with salt should succeed")
		assert.Contains(t, encoded, "{md5}$mysalt$", "Encoded password should contain salt format")
	})

	t.Run("SaltPositionPrefix", func(t *testing.T) {
		prefixEncoder := NewMd5Encoder(WithMd5Salt("salt"), WithMd5SaltPosition("prefix"))
		suffixEncoder := NewMd5Encoder(WithMd5Salt("salt"), WithMd5SaltPosition("suffix"))
		password := "password"

		prefixEncoded, err := prefixEncoder.Encode(password)
		require.NoError(t, err, "Prefix encoding should succeed")
		suffixEncoded, err := suffixEncoder.Encode(password)
		require.NoError(t, err, "Suffix encoding should succeed")

		assert.NotEqual(t, prefixEncoded, suffixEncoded, "Prefix and suffix salt positions should produce different hashes")
		assert.Contains(t, prefixEncoded, "{md5}$salt$", "Prefix encoded password should contain salt format")
		assert.Contains(t, suffixEncoded, "{md5}$salt$", "Suffix encoded password should contain salt format")
	})

	t.Run("SaltPositionSuffix", func(t *testing.T) {
		saltedEncoder := NewMd5Encoder(WithMd5Salt("mysalt"), WithMd5SaltPosition("suffix"))
		password := "password"

		encoded, err := saltedEncoder.Encode(password)

		require.NoError(t, err, "Encoding with suffix salt should succeed")
		assert.Contains(t, encoded, "{md5}$mysalt$", "Encoded password should contain salt format")
	})
}

// TestMd5EncoderMatches tests the Matches method of MD5Encoder.
func TestMd5EncoderMatches(t *testing.T) {
	encoder := NewMd5Encoder()

	t.Run("MatchesCorrectPassword", func(t *testing.T) {
		password := "password"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match correct password")
	})

	t.Run("RejectsIncorrectPassword", func(t *testing.T) {
		password := "password"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches("wrongpassword", encoded)

		assert.False(t, result, "Should reject incorrect password")
	})

	t.Run("MatchesEmptyPassword", func(t *testing.T) {
		encoded, err := encoder.Encode("")
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches("", encoded)

		assert.True(t, result, "Should match empty password")
	})

	t.Run("RejectsEmptyPasswordAgainstNonEmpty", func(t *testing.T) {
		password := "password"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches("", encoded)

		assert.False(t, result, "Should reject empty password against non-empty hash")
	})

	t.Run("MatchesUnicodeCharacters", func(t *testing.T) {
		password := "密码🔒"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match unicode password")
	})

	t.Run("CaseSensitive", func(t *testing.T) {
		password := "Password"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		assert.True(t, encoder.Matches("Password", encoded), "Should match original case")
		assert.False(t, encoder.Matches("password", encoded), "Should reject lowercase password")
		assert.False(t, encoder.Matches("PASSWORD", encoded), "Should reject uppercase password")
	})

	t.Run("MatchesSaltedHash", func(t *testing.T) {
		saltedEncoder := NewMd5Encoder(WithMd5Salt("mysalt"))
		password := "password"
		encoded, err := saltedEncoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := saltedEncoder.Matches(password, encoded)

		assert.True(t, result, "Should match salted hash")
	})

	t.Run("RejectsSaltedHashWithWrongPassword", func(t *testing.T) {
		saltedEncoder := NewMd5Encoder(WithMd5Salt("mysalt"))
		password := "password"
		encoded, err := saltedEncoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := saltedEncoder.Matches("wrongpassword", encoded)

		assert.False(t, result, "Should reject wrong password for salted hash")
	})

	t.Run("MatchesPrefixSaltedHash", func(t *testing.T) {
		prefixEncoder := NewMd5Encoder(WithMd5Salt("salt"), WithMd5SaltPosition("prefix"))
		password := "password"
		encoded, err := prefixEncoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := prefixEncoder.Matches(password, encoded)

		assert.True(t, result, "Should match prefix salted hash")
	})

	t.Run("MatchesSuffixSaltedHash", func(t *testing.T) {
		suffixEncoder := NewMd5Encoder(WithMd5Salt("salt"), WithMd5SaltPosition("suffix"))
		password := "password"
		encoded, err := suffixEncoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := suffixEncoder.Matches(password, encoded)

		assert.True(t, result, "Should match suffix salted hash")
	})

	t.Run("RejectsInvalidHashFormat", func(t *testing.T) {
		invalidHashes := []string{
			"{md5}$salt",
			"{md5}$$hash",
			"invalid",
			"",
		}

		for _, hash := range invalidHashes {
			result := encoder.Matches("password", hash)
			assert.False(t, result, "Should reject invalid hash: %s", hash)
		}
	})

	t.Run("MatchesRawMD5Hash", func(t *testing.T) {
		password := "password"
		encoded, err := encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := encoder.Matches(password, encoded)

		assert.True(t, result, "Should match raw MD5 hash")
	})
}

// TestMd5EncoderUpgradeEncoding tests the UpgradeEncoding method of MD5Encoder.
func TestMd5EncoderUpgradeEncoding(t *testing.T) {
	encoder := NewMd5Encoder()

	t.Run("AlwaysNeedsUpgrade", func(t *testing.T) {
		testCases := []string{
			"5f4dcc3b5aa765d61d8327deb882cf99",
			"{md5}$salt$hash",
			"anyhash",
			"",
		}

		for _, tc := range testCases {
			needsUpgrade := encoder.UpgradeEncoding(tc)
			assert.True(t, needsUpgrade, "MD5 should always need upgrade for: %s", tc)
		}
	})
}
