package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewJWT tests new JWT functionality.
func TestNewJWT(t *testing.T) {
	t.Run("ValidHexSecret", func(t *testing.T) {
		config := &JWTConfig{
			Secret:   DefaultJWTSecret,
			Audience: "test_app",
		}
		jwt, err := NewJWT(config)
		require.NoError(t, err, "Valid hex secret should create JWT without error")
		assert.NotNil(t, jwt, "Valid hex secret should return a JWT instance")
		assert.Equal(t, "test_app", jwt.config.Audience, "JWT audience should match configured audience")
	})

	t.Run("InvalidHexSecret", func(t *testing.T) {
		config := &JWTConfig{
			Secret: "invalid-hex",
		}
		jwt, err := NewJWT(config)
		assert.Error(t, err, "Invalid hex secret should return an error")
		assert.Nil(t, jwt, "Invalid hex secret should not create JWT")
		assert.Contains(t, err.Error(), "failed to decode jwt secret", "Invalid hex secret error should explain decode failure")
	})

	t.Run("EmptySecretUsesDefault", func(t *testing.T) {
		config := &JWTConfig{
			Secret: "",
		}
		jwt, err := NewJWT(config)
		require.NoError(t, err, "Empty secret should create JWT with default secret")
		assert.NotNil(t, jwt, "Empty secret should return a JWT instance")
		assert.Equal(t, 32, len(jwt.secret), "Default JWT secret should decode to 32 bytes") // Default secret is 64 hex chars = 32 bytes
	})

	t.Run("EmptyAudienceUsesDefault", func(t *testing.T) {
		config := &JWTConfig{
			Secret:   DefaultJWTSecret,
			Audience: "",
		}
		jwt, err := NewJWT(config)
		require.NoError(t, err, "Empty audience should create JWT with default audience")
		assert.Equal(t, DefaultJWTAudience, jwt.config.Audience, "Empty audience should use default audience")
	})
}

// TestJWTGenerate tests JWT generate functionality.
func TestJWTGenerate(t *testing.T) {
	config := &JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: "test_app",
	}
	jwt, err := NewJWT(config)
	require.NoError(t, err, "JWT generation tests should create JWT")

	t.Run("GenerateValidToken", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("user_id", "123").
			WithClaim("username", "testuser")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Valid claims should generate token")
		assert.NotEmpty(t, token, "Generated token should not be empty")

		// Verify token can be parsed
		claims, err := jwt.Parse(token)
		require.NoError(t, err, "Generated token should parse successfully")
		assert.Equal(t, "123", claims.Claim("user_id"), "Parsed token should preserve user_id claim")
		assert.Equal(t, "testuser", claims.Claim("username"), "Parsed token should preserve username claim")
	})

	t.Run("GenerateTokenWithNotBefore", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().WithClaim("test", "value")

		// Set nbf to 2 minutes in future (beyond the 1 minute leeway)
		token, err := jwt.Generate(builder, 1*time.Hour, 2*time.Minute)
		require.NoError(t, err, "Future not-before token should generate successfully")

		// Token should not be valid yet due to nbf
		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, ErrTokenNotValidYet, "Error should be ErrTokenNotValidYet")
	})

	t.Run("StandardClaimsAreSetCorrectly", func(t *testing.T) {
		builder := NewJWTClaimsBuilder()
		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Standard claims token should generate successfully")

		claims, err := jwt.Parse(token)
		require.NoError(t, err, "Standard claims token should parse successfully")

		assert.Equal(t, JWTIssuer, claims.Claim(claimIssuer), "Standard claims should include framework issuer")
		assert.Equal(t, "test_app", claims.Claim(claimAudience), "Standard claims should include configured audience")
		iat, ok := claims.Claim(claimIssuedAt).(float64)
		require.True(t, ok, "Issued-at claim should be numeric")
		exp, ok := claims.Claim(claimExpiresAt).(float64)
		require.True(t, ok, "Expires-at claim should be numeric")
		assert.Greater(t, int64(iat), int64(0), "Issued-at claim should be after Unix epoch")
		assert.Greater(t, int64(exp), int64(iat), "Expires-at claim should be after issued-at claim")
	})
}

// TestJWTParse tests JWT parse functionality.
func TestJWTParse(t *testing.T) {
	config := &JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: "test_app",
	}
	jwt, err := NewJWT(config)
	require.NoError(t, err, "JWT parse tests should create JWT")

	t.Run("ParseValidToken", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("user_id", "456").
			WithClaim("role", "admin")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Valid parse token should generate successfully")

		claims, err := jwt.Parse(token)
		require.NoError(t, err, "Valid parse token should parse successfully")
		assert.Equal(t, "456", claims.Claim("user_id"), "Parsed token should preserve user_id claim")
		assert.Equal(t, "admin", claims.Claim("role"), "Parsed token should preserve role claim")
	})

	t.Run("ParseExpiredToken", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().WithClaim("test", "value")
		token, err := jwt.Generate(builder, -1*time.Hour, 0) // Already expired
		require.NoError(t, err, "Expired token fixture should generate successfully")

		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, ErrTokenExpired, "Error should be ErrTokenExpired")
	})

	t.Run("ParseTokenWithWrongAudience", func(t *testing.T) {
		wrongConfig := &JWTConfig{
			Secret:   DefaultJWTSecret,
			Audience: "wrong_app",
		}
		wrongJwt, err := NewJWT(wrongConfig)
		require.NoError(t, err, "Wrong-audience JWT fixture should create JWT")

		builder := NewJWTClaimsBuilder().WithClaim("test", "value")
		token, err := wrongJwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Wrong-audience token fixture should generate successfully")

		// Try to parse with original JWT (different audience)
		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, ErrTokenInvalidAudience, "Error should be ErrTokenInvalidAudience")
	})

	t.Run("ParseTokenWithWrongSecret", func(t *testing.T) {
		wrongConfig := &JWTConfig{
			Secret:   "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			Audience: "test_app",
		}
		wrongJwt, err := NewJWT(wrongConfig)
		require.NoError(t, err, "Wrong-secret JWT fixture should create JWT")

		builder := NewJWTClaimsBuilder().WithClaim("test", "value")
		token, err := wrongJwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Wrong-secret token fixture should generate successfully")

		// Try to parse with original JWT (different secret)
		_, err = jwt.Parse(token)
		assert.ErrorIs(t, err, ErrTokenInvalid, "Error should be ErrTokenInvalid")
	})

	t.Run("ParseMalformedToken", func(t *testing.T) {
		_, err := jwt.Parse("malformed.token.string")
		assert.ErrorIs(t, err, ErrTokenInvalid, "Error should be ErrTokenInvalid")
	})

	t.Run("ParseEmptyToken", func(t *testing.T) {
		_, err := jwt.Parse("")
		assert.ErrorIs(t, err, ErrTokenInvalid, "Error should be ErrTokenInvalid")
	})
}

// TestJWTErrorMapping tests JWT error mapping functionality.
func TestJWTErrorMapping(t *testing.T) {
	config := &JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: "test_app",
	}
	jwt, err := NewJWT(config)
	require.NoError(t, err, "JWT error mapping tests should create JWT")

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
			expectedError: ErrTokenExpired,
		},
		{
			name: "NotYetValidToken",
			tokenGen: func() string {
				builder := NewJWTClaimsBuilder()
				token, _ := jwt.Generate(builder, 1*time.Hour, 2*time.Minute)

				return token
			},
			expectedError: ErrTokenNotValidYet,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := tc.tokenGen()
			_, err := jwt.Parse(token)
			assert.ErrorIs(t, err, tc.expectedError, "JWT parse error should match expected token validity error")
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
	require.NoError(t, err, "JWT claims accessor tests should create JWT")

	t.Run("IDReturnsJWTID", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithID("my-jwt-id").
			WithSubject("user1")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Token with ID claim should generate successfully")

		accessor, err := jwt.Parse(token)
		require.NoError(t, err, "Token with ID claim should parse successfully")

		assert.Equal(t, "my-jwt-id", accessor.ID(), "Should return the JWT ID")
	})

	t.Run("IDReturnsEmptyWhenMissing", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithSubject("user1")

		token, err := jwt.Generate(builder, 1*time.Hour, 0)
		require.NoError(t, err, "Token without ID claim should generate successfully")

		accessor, err := jwt.Parse(token)
		require.NoError(t, err, "Token without ID claim should parse successfully")

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
		assert.Equal(t, "test", claims["string_val"], "Builder should preserve string claim")
		assert.Equal(t, 123, claims["int_val"], "Builder should preserve int claim")
		assert.Equal(t, true, claims["bool_val"], "Builder should preserve bool claim")
		assert.Equal(t, 3.14, claims["float_val"], "Builder should preserve float claim")
		assert.Equal(t, map[string]any{"key": "value"}, claims["map_val"], "Builder should preserve map claim")
	})

	t.Run("OverwriteExistingClaim", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithClaim("key", "value1").
			WithClaim("key", "value2")

		claims := builder.build()
		assert.Equal(t, "value2", claims["key"], "Later claim value should overwrite earlier value")
	})

	t.Run("UseSpecializedClaimMethods", func(t *testing.T) {
		builder := NewJWTClaimsBuilder().
			WithID("jwt123").
			WithSubject("user456").
			WithType("access").
			WithRoles([]string{"admin", "user"}).
			WithDetails(map[string]any{"email": "test@example.com"})

		id, ok := builder.ID()
		assert.True(t, ok, "Builder should report ID claim as present")
		assert.Equal(t, "jwt123", id, "Builder should return ID claim value")

		subject, ok := builder.Subject()
		assert.True(t, ok, "Builder should report subject claim as present")
		assert.Equal(t, "user456", subject, "Builder should return subject claim value")

		typ, ok := builder.Type()
		assert.True(t, ok, "Builder should report token type claim as present")
		assert.Equal(t, "access", typ, "Builder should return token type claim value")

		roles, ok := builder.Roles()
		assert.True(t, ok, "Builder should report roles claim as present")
		assert.Equal(t, []string{"admin", "user"}, roles, "Builder should return roles claim value")

		details, ok := builder.Details()
		assert.True(t, ok, "Builder should report details claim as present")
		assert.Equal(t, map[string]any{"email": "test@example.com"}, details, "Builder should return details claim value")
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
