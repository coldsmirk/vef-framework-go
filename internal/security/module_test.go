package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

func TestNewTokenAuthenticators(t *testing.T) {
	jwt, err := security.NewJWT(&security.JWTConfig{
		Secret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	})
	require.NoError(t, err, "building the test JWT signer should succeed")

	store := security.NewMemorySessionStore()

	supported := func(authenticators []security.Authenticator, authType string) bool {
		for _, authenticator := range authenticators {
			if authenticator.Supports(authType) {
				return true
			}
		}

		return false
	}

	t.Run("JWTMechanismRegistersAccessAndRefresh", func(t *testing.T) {
		cfg := &config.SecurityConfig{TokenType: config.TokenTypeJWT}

		authenticators := newTokenAuthenticators(cfg, jwt, nil, store, security.SessionPolicy{})
		require.Len(t, authenticators, 2, "the jwt mechanism should register exactly its access and refresh authenticators")
		assert.True(t, supported(authenticators, AuthTypeJWTToken), "jwt access tokens must be accepted under jwt_token")
		assert.True(t, supported(authenticators, AuthTypeRefresh), "refresh tokens must be accepted under jwt_token")
		assert.False(t, supported(authenticators, AuthTypeOpaqueToken), "opaque tokens must not be accepted under jwt_token")
	})

	t.Run("OpaqueMechanismRegistersOnlyOpaque", func(t *testing.T) {
		cfg := &config.SecurityConfig{TokenType: config.TokenTypeOpaque}

		authenticators := newTokenAuthenticators(cfg, jwt, nil, store, security.SessionPolicy{})
		require.Len(t, authenticators, 1, "the opaque mechanism should register exactly its session authenticator")
		assert.True(t, supported(authenticators, AuthTypeOpaqueToken), "opaque tokens must be accepted under opaque_token")
		assert.False(t, supported(authenticators, AuthTypeJWTToken), "a leftover jwt access token must not authenticate under opaque_token")
		assert.False(t, supported(authenticators, AuthTypeRefresh), "a leftover refresh token must not authenticate under opaque_token")
	})
}
