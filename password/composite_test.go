package password

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompositeEncoderEncode tests the Encode method of CompositeEncoder.
func TestCompositeEncoderEncode(t *testing.T) {
	encoders := map[EncoderID]Encoder{
		EncoderBcrypt:    NewBcryptEncoder(),
		EncoderArgon2:    NewArgon2Encoder(),
		EncoderMd5:       NewMd5Encoder(),
		EncoderPlaintext: NewPlaintextEncoder(),
	}
	composite := NewCompositeEncoder(EncoderBcrypt, encoders)

	t.Run("EncodesWithDefaultEncoder", func(t *testing.T) {
		password := "testpassword"

		encoded, err := composite.Encode(password)

		require.NoError(t, err, "Encoding should succeed")
		assert.Contains(t, encoded, "{bcrypt}", "Encoded password should contain encoder prefix")
	})

	t.Run("ErrorWhenDefaultEncoderNotRegistered", func(t *testing.T) {
		invalidComposite := NewCompositeEncoder(EncoderID("nonexistent"), encoders)

		encoded, err := invalidComposite.Encode("password")

		assert.Error(t, err, "Should return error when default encoder not found")
		assert.ErrorIs(t, err, ErrDefaultEncoderNotFound, "Should return ErrDefaultEncoderNotFound")
		assert.Empty(t, encoded, "Encoded password should be empty on error")
	})
}

// TestCompositeEncoderMatches tests the Matches method of CompositeEncoder.
func TestCompositeEncoderMatches(t *testing.T) {
	encoders := map[EncoderID]Encoder{
		EncoderBcrypt:    NewBcryptEncoder(),
		EncoderArgon2:    NewArgon2Encoder(),
		EncoderMd5:       NewMd5Encoder(),
		EncoderPlaintext: NewPlaintextEncoder(),
	}
	composite := NewCompositeEncoder(EncoderBcrypt, encoders)

	t.Run("MatchesWithCorrectEncoder", func(t *testing.T) {
		password := "testpassword"
		encoded, err := composite.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := composite.Matches(password, encoded)

		assert.True(t, result, "Should match correct password")
	})

	t.Run("MatchesLegacyMD5Format", func(t *testing.T) {
		md5Encoder := NewMd5Encoder()
		password := "legacypassword"
		md5Hash, err := md5Encoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		legacyFormat := "{md5}" + md5Hash

		result := composite.Matches(password, legacyFormat)

		assert.True(t, result, "Should match legacy MD5 format")
	})

	t.Run("MatchesWithoutPrefix", func(t *testing.T) {
		bcryptEncoder := NewBcryptEncoder()
		password := "oldformat"
		hash, err := bcryptEncoder.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		result := composite.Matches(password, hash)

		assert.True(t, result, "Should match hash without encoder prefix")
	})

	t.Run("RejectsUnknownEncoderId", func(t *testing.T) {
		password := "password"
		invalidFormat := "{unknown}somehash"

		result := composite.Matches(password, invalidFormat)

		assert.False(t, result, "Should reject unknown encoder ID")
	})

	t.Run("CrossEncoderCompatibility", func(t *testing.T) {
		password := "password"

		bcryptHash, err := encoders[EncoderBcrypt].Encode(password)
		require.NoError(t, err, "Bcrypt encoding should succeed")
		argon2Hash, err := encoders[EncoderArgon2].Encode(password)
		require.NoError(t, err, "Argon2 encoding should succeed")

		bcryptFormat := "{bcrypt}" + bcryptHash
		argon2Format := "{argon2}" + argon2Hash

		assert.True(t, composite.Matches(password, bcryptFormat), "Should match bcrypt format")
		assert.True(t, composite.Matches(password, argon2Format), "Should match argon2 format")
		assert.False(t, composite.Matches("wrongpassword", bcryptFormat), "Should reject wrong password with bcrypt")
		assert.False(t, composite.Matches("wrongpassword", argon2Format), "Should reject wrong password with argon2")
	})
}

// TestCompositeEncoderUpgradeEncoding tests the UpgradeEncoding method of CompositeEncoder.
func TestCompositeEncoderUpgradeEncoding(t *testing.T) {
	encoders := map[EncoderID]Encoder{
		EncoderBcrypt:    NewBcryptEncoder(),
		EncoderArgon2:    NewArgon2Encoder(),
		EncoderMd5:       NewMd5Encoder(),
		EncoderPlaintext: NewPlaintextEncoder(),
	}
	composite := NewCompositeEncoder(EncoderBcrypt, encoders)

	t.Run("UpgradeDetectsNonDefaultEncoder", func(t *testing.T) {
		md5Hash, err := encoders[EncoderMd5].Encode("password")
		require.NoError(t, err, "Encoding should succeed")

		legacyFormat := "{md5}" + md5Hash

		needsUpgrade := composite.UpgradeEncoding(legacyFormat)

		assert.True(t, needsUpgrade, "Should need upgrade for non-default encoder")
	})

	t.Run("NoUpgradeForDefaultEncoder", func(t *testing.T) {
		password := "password"
		encoded, err := composite.Encode(password)
		require.NoError(t, err, "Encoding should succeed")

		needsUpgrade := composite.UpgradeEncoding(encoded)

		assert.False(t, needsUpgrade, "Should not need upgrade for default encoder")
	})
}
