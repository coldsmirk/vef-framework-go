package password

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlaintextEncoderEncode tests the Encode method of PlaintextEncoder.
func TestPlaintextEncoderEncode(t *testing.T) {
	encoder := NewPlaintextEncoder()

	t.Run("EncodesAsPlaintext", func(t *testing.T) {
		password := "testpassword"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding should succeed")
		assert.Equal(t, password, encoded, "Encoded password should equal plaintext")
	})

	t.Run("EmptyPassword", func(t *testing.T) {
		encoded, err := encoder.Encode("")

		require.NoError(t, err, "Encoding empty password should succeed")
		assert.Equal(t, "", encoded, "Encoded empty password should be empty")
	})

	t.Run("UnicodeCharacters", func(t *testing.T) {
		password := "密码测试🔒"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding unicode password should succeed")
		assert.Equal(t, password, encoded, "Encoded unicode password should equal plaintext")
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		password := "p@ssw0rd!#$%"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding special characters should succeed")
		assert.Equal(t, password, encoded, "Encoded password should equal plaintext")
	})

	t.Run("LongPassword", func(t *testing.T) {
		password := "this is a very long password with many characters to test plaintext encoding"

		encoded, err := encoder.Encode(password)

		require.NoError(t, err, "Encoding long password should succeed")
		assert.Equal(t, password, encoded, "Encoded long password should equal plaintext")
	})
}

// TestPlaintextEncoderMatches tests the Matches method of PlaintextEncoder.
func TestPlaintextEncoderMatches(t *testing.T) {
	encoder := NewPlaintextEncoder()

	t.Run("MatchesCorrectPassword", func(t *testing.T) {
		password := "password"

		result := encoder.Matches(password, password)

		assert.True(t, result, "Should match correct password")
	})

	t.Run("RejectsIncorrectPassword", func(t *testing.T) {
		result := encoder.Matches("password", "wrongpassword")

		assert.False(t, result, "Should reject incorrect password")
	})

	t.Run("MatchesEmptyPassword", func(t *testing.T) {
		result := encoder.Matches("", "")

		assert.True(t, result, "Should match empty password")
	})

	t.Run("RejectsEmptyPasswordAgainstNonEmpty", func(t *testing.T) {
		result := encoder.Matches("", "password")

		assert.False(t, result, "Should reject empty password against non-empty")
	})

	t.Run("MatchesUnicodeCharacters", func(t *testing.T) {
		password := "密码🔒"

		result := encoder.Matches(password, password)

		assert.True(t, result, "Should match unicode password")
	})

	t.Run("CaseSensitive", func(t *testing.T) {
		password := "TestPassword"

		assert.True(t, encoder.Matches("TestPassword", password), "Should match original case")
		assert.False(t, encoder.Matches("testpassword", password), "Should reject lowercase password")
		assert.False(t, encoder.Matches("TESTPASSWORD", password), "Should reject uppercase password")
	})

	t.Run("MatchesSpecialCharacters", func(t *testing.T) {
		password := "p@ssw0rd!#$%"

		result := encoder.Matches(password, password)

		assert.True(t, result, "Should match special characters password")
	})
}

// TestPlaintextEncoderUpgradeEncoding tests the UpgradeEncoding method of PlaintextEncoder.
func TestPlaintextEncoderUpgradeEncoding(t *testing.T) {
	encoder := NewPlaintextEncoder()

	t.Run("AlwaysNeedsUpgrade", func(t *testing.T) {
		testCases := []string{
			"anypassword",
			"",
			"密码",
			"$2a$10$somehash",
			"{bcrypt}hash",
		}

		for _, tc := range testCases {
			needsUpgrade := encoder.UpgradeEncoding(tc)
			assert.True(t, needsUpgrade, "Plaintext should always need upgrade for: %s", tc)
		}
	})
}
