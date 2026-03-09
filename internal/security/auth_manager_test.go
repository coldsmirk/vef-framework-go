package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type AuthManagerTestSuite struct {
	suite.Suite
}

// TestAuthenticate verifies AuthManager authentication dispatch and error handling.
func (s *AuthManagerTestSuite) TestAuthenticate() {
	ctx := context.Background()

	s.Run("MatchingAuthenticator", func() {
		principal := security.NewUser("user1", "Alice", "admin")

		auth := new(MockAuthenticator)
		auth.On("Supports", "password").Return(true)
		auth.On("Authenticate", mock.Anything, mock.Anything).Return(principal, nil)

		manager := NewAuthManager([]security.Authenticator{auth})

		got, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		s.Require().NoError(err, "Should authenticate without error")
		s.Equal("user1", got.ID, "Should return expected principal")
		auth.AssertExpectations(s.T())
	})

	s.Run("NoMatchingAuthenticator", func() {
		auth := new(MockAuthenticator)
		auth.On("Supports", "oauth").Return(false)

		manager := NewAuthManager([]security.Authenticator{auth})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "oauth",
			Principal: "alice",
		})
		s.Require().Error(err, "Should return error for unsupported type")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeUnsupportedAuthenticationType, resErr.Code, "Should return unsupported auth type code")
	})

	s.Run("AuthenticatorReturnsResultError", func() {
		authErr := result.ErrCredentialsInvalid("bad password")

		auth := new(MockAuthenticator)
		auth.On("Supports", "password").Return(true)
		auth.On("Authenticate", mock.Anything, mock.Anything).Return(nil, authErr)

		manager := NewAuthManager([]security.Authenticator{auth})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		s.Require().Error(err, "Should propagate authenticator error")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should preserve error code")
	})

	s.Run("AuthenticatorReturnsGenericError", func() {
		auth := new(MockAuthenticator)
		auth.On("Supports", "password").Return(true)
		auth.On("Authenticate", mock.Anything, mock.Anything).Return(nil, errors.New("db connection failed"))

		manager := NewAuthManager([]security.Authenticator{auth})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		s.Require().Error(err, "Should propagate generic error")
		s.Equal("db connection failed", err.Error(), "Should preserve error message")
	})

	s.Run("MultipleAuthenticatorsSelectsCorrect", func() {
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
		s.Require().NoError(err, "Should authenticate with matching authenticator")
		s.Equal("user1", got.ID, "Should return expected principal")
		tokenAuth.AssertNotCalled(s.T(), "Authenticate")
		passwordAuth.AssertExpectations(s.T())
	})

	s.Run("EmptyAuthenticatorList", func() {
		manager := NewAuthManager([]security.Authenticator{})

		_, err := manager.Authenticate(ctx, security.Authentication{
			Type:      "password",
			Principal: "alice",
		})
		s.Require().Error(err, "Should return error with no authenticators")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeUnsupportedAuthenticationType, resErr.Code, "Should return unsupported auth type code")
	})
}

// TestMaskPrincipal verifies principal masking for log safety.
func (s *AuthManagerTestSuite) TestMaskPrincipal() {
	s.Run("EmptyString", func() {
		s.Equal("<empty>", maskPrincipal(""), "Should return <empty> for empty string")
	})

	s.Run("ShortString", func() {
		s.Equal("***", maskPrincipal("ab"), "Should fully mask short strings")
		s.Equal("***", maskPrincipal("abc"), "Should fully mask 3-char strings")
	})

	s.Run("LongString", func() {
		s.Equal("ali***", maskPrincipal("alice"), "Should show first 3 chars and mask rest")
		s.Equal("use***", maskPrincipal("user@example.com"), "Should show first 3 chars and mask rest")
	})

	s.Run("SingleChar", func() {
		s.Equal("***", maskPrincipal("a"), "Should fully mask single char")
	})
}

func TestAuthManager(t *testing.T) {
	suite.Run(t, new(AuthManagerTestSuite))
}
