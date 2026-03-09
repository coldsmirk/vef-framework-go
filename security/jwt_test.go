package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/result"
)

// TestNewJWT tests new JWT functionality.
func TestNewJWT(t *testing.T) {
	t.Run("ValidHexSecret", func(t *testing.T) {
		config := &JWTConfig{
			Secret:   DefaultJWTSecret,
			Audience: "test_app",
		}
		jwt, err := NewJWT(config)
		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, jwt, "Should not be nil")
		assert.Equal(t, "test_app", jwt.config.Audience, "Should equal expected value")
	})

	t.Run("InvalidHexSecret", func(t *testing.T) {
		config := &JWTConfig{
			Secret: "invalid-hex",
		}
		jwt, err := NewJWT(config)
		assert.Error(t, err, "Should return error")
		assert.Nil(t, jwt, "Should be nil")
		assert.Contains(t, err.Error(), "failed to decode jwt secret", "Should contain expected value")
	})

	t.Run("EmptySecretUsesDefault", func(t *testing.T) {
		config := &JWTConfig{
			Secret: "",
		}
		jwt, err := NewJWT(config)
		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, jwt, "Should not be nil")
		assert.Equal(t, 32, len(jwt.secret), "Should equal expected value") // Default secret is 64 hex chars = 32 bytes
	})

	t.Run("EmptyAudienceUsesDefault", func(t *testing.T) {
		config := &JWTConfig{
			Secret:   DefaultJWTSecret,
			Audience: "",
		}
		jwt, err := NewJWT(config)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, DefaultJWTAudience, jwt.config.Audience, "Should equal expected value")
	})
}

// TestJWTGenerate tests JWT generate functionality.
func TestJWTGenerate(t *testing.T) {
	config := &JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: "test_app",
	}
	jwt, err := NewJWT(config)
	require.NoError(t, err, "Should not return error")

	t.Run("GenerateValidToken", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("user_id", "123").
			WithClaim("username", "testuser")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Should not return error")
		assert.NotEmpty(t, token, "Should not be empty")

		// Verify token can be parsed
		claims, err := jwt.Parse(token)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, "123", claims.Claim("user_id"), "Should equal expected value")
		assert.Equal(t, "testuser", claims.Claim("username"), "Should equal expected value")
	})

	t.Run("GenerateTokenWithNotBefore", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().WithClaim("test", "value")

		// Set nbf to 2 minutes in future (beyond the 1 minute leeway)
		token, err := jwt.Generate(builder, 1*time.Hour, 2*time.Minute)
		require.NoError(t, err, "Should not return error")

		// Token should not be valid yet due to nbf
		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, result.ErrTokenNotValidYet, "Error should be result.ErrTokenNotValidYet")
	})

	t.Run("StandardClaimsAreSetCorrectly", func(t *testing.T) {
		builder := NewJWTClaimsBuilder()
		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Should not return error")

		claims, err := jwt.Parse(token)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, JWTIssuer, claims.Claim(claimIssuer), "Should equal expected value")
		assert.Equal(t, "test_app", claims.Claim(claimAudience), "Should equal expected value")
		iat, ok := claims.Claim(claimIssuedAt).(float64)
		require.True(t, ok, "Should be ok")
		exp, ok := claims.Claim(claimExpiresAt).(float64)
		require.True(t, ok, "Should be ok")
		assert.Greater(t, int64(iat), int64(0), "Should be greater")
		assert.Greater(t, int64(exp), int64(iat), "Should be greater")
	})
}

// TestJWTParse tests JWT parse functionality.
func TestJWTParse(t *testing.T) {
	config := &JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: "test_app",
	}
	jwt, err := NewJWT(config)
	require.NoError(t, err, "Should not return error")

	t.Run("ParseValidToken", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("user_id", "456").
			WithClaim("role", "admin")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Should not return error")

		claims, err := jwt.Parse(token)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, "456", claims.Claim("user_id"), "Should equal expected value")
		assert.Equal(t, "admin", claims.Claim("role"), "Should equal expected value")
	})

	t.Run("ParseExpiredToken", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().WithClaim("test", "value")
		token, err := jwt.Generate(builder, -1*time.Hour, 0) // Already expired
		require.NoError(t, err, "Should not return error")

		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, result.ErrTokenExpired, "Error should be result.ErrTokenExpired")
	})

	t.Run("ParseTokenWithWrongAudience", func(t *testing.T) {
		wrongConfig := &JWTConfig{
			Secret:   DefaultJWTSecret,
			Audience: "wrong_app",
		}
		wrongJwt, err := NewJWT(wrongConfig)
		require.NoError(t, err, "Should not return error")

		builder := NewJWTClaimsBuilder().WithClaim("test", "value")
		token, err := wrongJwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Should not return error")

		// Try to parse with original JWT (different audience)
		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, result.ErrTokenInvalidAudience, "Error should be result.ErrTokenInvalidAudience")
	})

	t.Run("ParseTokenWithWrongSecret", func(t *testing.T) {
		wrongConfig := &JWTConfig{
			Secret:   "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			Audience: "test_app",
		}
		wrongJwt, err := NewJWT(wrongConfig)
		require.NoError(t, err, "Should not return error")

		builder := NewJWTClaimsBuilder().WithClaim("test", "value")
		token, err := wrongJwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Should not return error")

		// Try to parse with original JWT (different secret)
		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, result.ErrTokenInvalid, "Error should be result.ErrTokenInvalid")
	})

	t.Run("ParseMalformedToken", func(t *testing.T) {
		_, err := jwt.Parse("malformed.token.string")
		assert.ErrorIs(t, err, result.ErrTokenInvalid, "Error should be result.ErrTokenInvalid")
	})

	t.Run("ParseEmptyToken", func(t *testing.T) {
		_, err := jwt.Parse("")
		assert.ErrorIs(t, err, result.ErrTokenInvalid, "Error should be result.ErrTokenInvalid")
	})
}

// TestJWTErrorMapping tests JWT error mapping functionality.
func TestJWTErrorMapping(t *testing.T) {
	config := &JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: "test_app",
	}
	jwt, err := NewJWT(config)
	require.NoError(t, err, "Should not return error")

	testCases := []struct {
		name          string
		tokenGen      func() string
		expectedError error
	}{
		{
			name: "ExpiredToken",
			tokenGen: func() string {
				builder := NewJWTClaimsBuilder()
				token, _ := jwt.Generate(builder, -1*time.Hour, 0)

				return token
			},
			expectedError: result.ErrTokenExpired,
		},
		{
			name: "NotYetValidToken",
			tokenGen: func() string {
				builder := NewJWTClaimsBuilder()
				token, _ := jwt.Generate(builder, 1*time.Hour, 2*time.Minute)

				return token
			},
			expectedError: result.ErrTokenNotValidYet,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := tc.tokenGen()
			_, err := jwt.Parse(token)
			assert.ErrorIs(t, err, tc.expectedError, "Error should be tc.expectedError")
		})
	}
}

// TestJWTClaimsAccessor tests JWT claims accessor functionality.
func TestJWTClaimsAccessor(t *testing.T) {
	config := &JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: "test_app",
	}
	jwt, err := NewJWT(config)
	require.NoError(t, err, "Should not return error")

	t.Run("IDReturnsJWTID", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithID("my-jwt-id").
			WithSubject("user1")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Should not return error")

		accessor, err := jwt.Parse(token)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, "my-jwt-id", accessor.ID(), "Should return the JWT ID")
	})

	t.Run("IDReturnsEmptyWhenMissing", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithSubject("user1")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Should not return error")

		accessor, err := jwt.Parse(token)
		require.NoError(t, err, "Should not return error")

		assert.Empty(t, accessor.ID(), "Should return empty string when ID is missing")
	})
}

// TestJWTClaimsBuilder tests JWT claims builder functionality.
func TestJWTClaimsBuilder(t *testing.T) {
	t.Run("BuildClaimsWithVariousTypes", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("string_val", "test").
			WithClaim("int_val", 123).
			WithClaim("bool_val", true).
			WithClaim("float_val", 3.14).
			WithClaim("map_val", map[string]any{"key": "value"})

		claims := builder.build()
		assert.Equal(t, "test", claims["string_val"], "Should equal expected value")
		assert.Equal(t, 123, claims["int_val"], "Should equal expected value")
		assert.Equal(t, true, claims["bool_val"], "Should equal expected value")
		assert.Equal(t, 3.14, claims["float_val"], "Should equal expected value")
		assert.Equal(t, map[string]any{"key": "value"}, claims["map_val"], "Should equal expected value")
	})

	t.Run("OverwriteExistingClaim", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("key", "value1").
			WithClaim("key", "value2")

		claims := builder.build()
		assert.Equal(t, "value2", claims["key"], "Should equal expected value")
	})

	t.Run("UseSpecializedClaimMethods", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithID("jwt123").
			WithSubject("user456").
			WithType("access").
			WithRoles([]string{"admin", "user"}).
			WithDetails(map[string]any{"email": "test@example.com"})

		id, ok := builder.ID()
		assert.True(t, ok, "Should be ok")
		assert.Equal(t, "jwt123", id, "Should equal expected value")

		subject, ok := builder.Subject()
		assert.True(t, ok, "Should be ok")
		assert.Equal(t, "user456", subject, "Should equal expected value")

		typ, ok := builder.Type()
		assert.True(t, ok, "Should be ok")
		assert.Equal(t, "access", typ, "Should equal expected value")

		roles, ok := builder.Roles()
		assert.True(t, ok, "Should be ok")
		assert.Equal(t, []string{"admin", "user"}, roles, "Should equal expected value")

		details, ok := builder.Details()
		assert.True(t, ok, "Should be ok")
		assert.Equal(t, map[string]any{"email": "test@example.com"}, details, "Should equal expected value")
	})

	t.Run("ClaimGetterReturnsValueAndPresence", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("custom_key", "custom_value")

		val, ok := builder.Claim("custom_key")
		assert.True(t, ok, "Should return true for existing claim")
		assert.Equal(t, "custom_value", val, "Should return the claim value")

		val, ok = builder.Claim("missing_key")
		assert.False(t, ok, "Should return false for missing claim")
		assert.Nil(t, val, "Should return nil for missing claim")
	})
}
