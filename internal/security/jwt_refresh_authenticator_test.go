package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type JWTRefreshAuthenticatorTestSuite struct {
	suite.Suite

	jwt *security.JWT
	gen security.TokenGenerator
}

func (s *JWTRefreshAuthenticatorTestSuite) SetupSuite() {
	s.jwt = newTestJWT(s.T())
	s.gen = newTestTokenGenerator(s.jwt)
}

// TestSupports verifies type matching.
func (s *JWTRefreshAuthenticatorTestSuite) TestSupports() {
	auth := NewJWTRefreshAuthenticator(s.jwt, nil)
	s.True(auth.Supports(AuthTypeRefresh), "Should support refresh type")
	s.False(auth.Supports("token"), "Should not support token type")
	s.False(auth.Supports(""), "Should not support empty type")
}

// TestAuthenticate verifies all authentication paths.
func (s *JWTRefreshAuthenticatorTestSuite) TestAuthenticate() {
	ctx := context.Background()

	s.Run("NilUserLoader", func() {
		auth := NewJWTRefreshAuthenticator(s.jwt, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: "some-token",
		})
		s.Require().Error(err, "Should return error when user loader is nil")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeNotImplemented, resErr.Code, "Should return not implemented code")
	})

	s.Run("EmptyToken", func() {
		loader := new(MockUserLoader)
		auth := NewJWTRefreshAuthenticator(s.jwt, loader)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: "",
		})
		s.Require().Error(err, "Should return error for empty token")
	})

	s.Run("InvalidJWT", func() {
		loader := new(MockUserLoader)
		auth := NewJWTRefreshAuthenticator(s.jwt, loader)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: "invalid.jwt.token",
		})
		s.Require().Error(err, "Should return error for invalid JWT")
	})

	s.Run("AccessTokenRejected", func() {
		principal := security.NewUser("user1", "Alice")
		tokens, err := s.gen.Generate(principal)
		s.Require().NoError(err, "Should generate tokens")

		loader := new(MockUserLoader)
		auth := NewJWTRefreshAuthenticator(s.jwt, loader)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: tokens.AccessToken,
		})
		s.Require().Error(err, "Should reject access token")
	})

	s.Run("UserNotFoundOnReload", func() {
		principal := security.NewUser("user1", "Alice")
		tokens, err := s.gen.Generate(principal)
		s.Require().NoError(err, "Should generate tokens")

		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(nil, nil)

		auth := NewJWTRefreshAuthenticator(s.jwt, loader)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: tokens.RefreshToken,
		})
		s.Require().Error(err, "Should return error when user not found on reload")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsError", func() {
		principal := security.NewUser("user1", "Alice")
		tokens, err := s.gen.Generate(principal)
		s.Require().NoError(err, "Should generate tokens")

		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(nil, errors.New("db error"))

		auth := NewJWTRefreshAuthenticator(s.jwt, loader)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: tokens.RefreshToken,
		})
		s.Require().Error(err, "Should propagate loader error")
		s.Equal("db error", err.Error(), "Should preserve error message")
		loader.AssertExpectations(s.T())
	})

	s.Run("SuccessfulRefresh", func() {
		original := security.NewUser("user1", "Alice", "admin")
		tokens, err := s.gen.Generate(original)
		s.Require().NoError(err, "Should generate tokens")

		// Reload returns updated principal with new roles
		reloaded := security.NewUser("user1", "Alice", "admin", "editor")
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(reloaded, nil)

		auth := NewJWTRefreshAuthenticator(s.jwt, loader)

		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: tokens.RefreshToken,
		})
		s.Require().NoError(err, "Should refresh successfully")
		s.Equal("user1", got.ID, "Should return reloaded principal ID")
		s.Equal([]string{"admin", "editor"}, got.Roles, "Should return updated roles from reload")
		loader.AssertExpectations(s.T())
	})

	s.Run("ExpiredRefreshToken", func() {
		claimsBuilder := security.NewJWTClaimsBuilder().
			WithSubject("user1@Alice").
			WithType(security.TokenTypeRefresh)
		token, err := s.jwt.Generate(claimsBuilder, -1*time.Hour, 0)
		s.Require().NoError(err, "Should generate expired token")

		loader := new(MockUserLoader)
		auth := NewJWTRefreshAuthenticator(s.jwt, loader)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: token,
		})
		s.Require().Error(err, "Should reject expired refresh token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenExpired, resErr.Code, "Should return token expired code")
	})
}

func TestJWTRefreshAuthenticator(t *testing.T) {
	suite.Run(t, new(JWTRefreshAuthenticatorTestSuite))
}
