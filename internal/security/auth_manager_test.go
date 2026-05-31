package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// TestAuthenticate verifies AuthManager authentication dispatch and error handling.
func TestAuthManagerAuthenticate(t *testing.T) {
	ctx := context.Background()

	t.Run("MatchingAuthenticator", func(t *testing.T) {
		principal := security.NewUser("user1", "Alice", "admin")

		auth := new(MockAuthenticator)
		auth.On("Supports", "password").Return(true)
		auth.On("Authenticate", mock.Anything, mock.Anything).Return(principal, nil)

		manager := NewAuthManager([]security.Authenticator{auth})

		got, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		require.NoError(t, err, "Matching authenticator should authenticate without error")
		assert.Equal(t, "user1", got.ID, "Matching authenticator should return the loaded principal ID")
		auth.AssertExpectations(t)
	})

	t.Run("NoMatchingAuthenticator", func(t *testing.T) {
		auth := new(MockAuthenticator)
		auth.On("Supports", "oauth").Return(false)

		manager := NewAuthManager([]security.Authenticator{auth})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "oauth",
			Principal: "alice",
		})
		require.Error(t, err, "Unsupported auth type should return an error")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Unsupported auth type should return a result.Error")
		assert.Equal(t, security.ErrCodeUnsupportedAuthenticationType, resErr.Code, "Unsupported auth type should return unsupported authentication code")
	})

	t.Run("AuthenticatorReturnsResultError", func(t *testing.T) {
		authErr := security.ErrCredentialsInvalid("bad password")

		auth := new(MockAuthenticator)
		auth.On("Supports", "password").Return(true)
		auth.On("Authenticate", mock.Anything, mock.Anything).Return(nil, authErr)

		manager := NewAuthManager([]security.Authenticator{auth})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		require.Error(t, err, "Authenticator result error should be returned")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Authenticator result error should remain a result.Error")
		assert.Equal(t, security.ErrCodeCredentialsInvalid, resErr.Code, "Authenticator result error code should be preserved")
	})

	t.Run("AuthenticatorReturnsGenericError", func(t *testing.T) {
		auth := new(MockAuthenticator)
		auth.On("Supports", "password").Return(true)
		auth.On("Authenticate", mock.Anything, mock.Anything).Return(nil, errors.New("db connection failed"))

		manager := NewAuthManager([]security.Authenticator{auth})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		require.Error(t, err, "Authenticator generic error should be returned")
		assert.Equal(t, "db connection failed", err.Error(), "Authenticator generic error message should be preserved")
	})

	t.Run("MultipleAuthenticatorsSelectsCorrect", func(t *testing.T) {
		principal := security.NewUser("user1", "Alice")

		tokenAuth := new(MockAuthenticator)
		tokenAuth.On("Supports", "password").Return(false)

		passwordAuth := new(MockAuthenticator)
		passwordAuth.On("Supports", "password").Return(true)
		passwordAuth.On("Authenticate", mock.Anything, mock.Anything).Return(principal, nil)

		manager := NewAuthManager([]security.Authenticator{tokenAuth, passwordAuth})

		got, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		require.NoError(t, err, "Matching authenticator in list should authenticate without error")
		assert.Equal(t, "user1", got.ID, "Matching authenticator in list should return the loaded principal ID")
		tokenAuth.AssertNotCalled(t, "Authenticate")
		passwordAuth.AssertExpectations(t)
	})

	t.Run("EmptyAuthenticatorList", func(t *testing.T) {
		manager := NewAuthManager([]security.Authenticator{})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		require.Error(t, err, "Empty authenticator list should return an error")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Empty authenticator list should return a result.Error")
		assert.Equal(t, security.ErrCodeUnsupportedAuthenticationType, resErr.Code, "Empty authenticator list should return unsupported authentication code")
	})
}

// TestMaskPrincipal verifies principal masking for log safety.
func TestMaskPrincipal(t *testing.T) {
	tests := []struct {
		name      string
		principal string
		expected  string
	}{
		{"EmptyString", "", "<empty>"},
		{"TwoCharacters", "ab", "***"},
		{"ThreeCharacters", "abc", "***"},
		{"FiveCharacters", "alice", "ali***"},
		{"EmailAddress", "user@example.com", "use***"},
		{"SingleCharacter", "a", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, maskPrincipal(tt.principal), "Principal mask should match expected redaction")
		})
	}
}
