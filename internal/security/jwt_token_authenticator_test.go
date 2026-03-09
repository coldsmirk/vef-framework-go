package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type JWTTokenAuthenticatorTestSuite struct {
	suite.Suite

	jwt  *security.JWT
	auth security.Authenticator
	gen  security.TokenGenerator
}

func (s *JWTTokenAuthenticatorTestSuite) SetupSuite() {
	s.jwt = newTestJWT(s.T())
	s.auth = NewJWTAuthenticator(s.jwt)
	s.gen = newTestTokenGenerator(s.jwt)
}

// TestSupports verifies type matching.
func (s *JWTTokenAuthenticatorTestSuite) TestSupports() {
	s.True(s.auth.Supports(AuthTypeToken), "Should support token type")
	s.False(s.auth.Supports("password"), "Should not support password type")
	s.False(s.auth.Supports(""), "Should not support empty type")
}

// TestAuthenticate verifies all authentication paths.
func (s *JWTTokenAuthenticatorTestSuite) TestAuthenticate() {
	ctx := context.Background()

	s.Run("EmptyToken", func() {
		_, err := s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: "",
		})
		s.Require().Error(err, "Should return error for empty token")
	})

	s.Run("InvalidJWT", func() {
		_, err := s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: "invalid.jwt.token",
		})
		s.Require().Error(err, "Should return error for invalid JWT")
	})

	s.Run("RefreshTokenRejected", func() {
		principal := security.NewUser("user1", "Alice", "admin")
		tokens, err := s.gen.Generate(principal)
		s.Require().NoError(err, "Should generate tokens")

		_, err = s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: tokens.RefreshToken,
		})
		s.Require().Error(err, "Should reject refresh token")
	})

	s.Run("ChallengeTokenRejected", func() {
		claimsBuilder := security.NewJWTClaimsBuilder().
			WithSubject("user1@Alice").
			WithType(security.TokenTypeChallenge)
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate challenge token")

		_, err = s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: token,
		})
		s.Require().Error(err, "Should reject challenge token")
	})

	s.Run("ValidAccessToken", func() {
		principal := security.NewUser("user1", "Alice", "admin", "editor")
		principal.Details = map[string]any{"department": "engineering"}

		tokens, err := s.gen.Generate(principal)
		s.Require().NoError(err, "Should generate tokens")

		got, err := s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: tokens.AccessToken,
		})
		s.Require().NoError(err, "Should authenticate with valid access token")
		s.Equal("user1", got.ID, "Should extract correct ID")
		s.Equal("Alice", got.Name, "Should extract correct name")
		s.Equal([]string{"admin", "editor"}, got.Roles, "Should extract roles")
		s.NotNil(got.Details, "Should extract details")
	})

	s.Run("ValidAccessTokenWithoutRoles", func() {
		principal := security.NewUser("user2", "Bob")
		tokens, err := s.gen.Generate(principal)
		s.Require().NoError(err, "Should generate tokens")

		got, err := s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: tokens.AccessToken,
		})
		s.Require().NoError(err, "Should authenticate without roles")
		s.Equal("user2", got.ID, "Should extract correct ID")
		s.Empty(got.Roles, "Should have empty roles")
		s.Nil(got.Details, "Should have nil details")
	})

	s.Run("SubjectWithAtSign", func() {
		principal := security.NewUser("user3", "user@example.com")
		tokens, err := s.gen.Generate(principal)
		s.Require().NoError(err, "Should generate tokens")

		got, err := s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: tokens.AccessToken,
		})
		s.Require().NoError(err, "Should authenticate with @ in name")
		s.Equal("user3", got.ID, "Should extract ID before first @")
		s.Equal("user@example.com", got.Name, "Should preserve name with @ sign")
	})

	s.Run("MalformedSubjectNoAtSign", func() {
		claimsBuilder := security.NewJWTClaimsBuilder().
			WithSubject("user1-no-at-sign").
			WithType(security.TokenTypeAccess)
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate token with malformed subject")

		_, err = s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: token,
		})
		s.Require().Error(err, "Should reject token with malformed subject")
	})

	s.Run("ExpiredToken", func() {
		claimsBuilder := security.NewJWTClaimsBuilder().
			WithSubject("user1@Alice").
			WithType(security.TokenTypeAccess)
		token, err := s.jwt.Generate(claimsBuilder, -1*time.Hour, 0)
		s.Require().NoError(err, "Should generate expired token")

		_, err = s.auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeToken,
			Principal: token,
		})
		s.Require().Error(err, "Should reject expired token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenExpired, resErr.Code, "Should return token expired code")
	})
}

func TestJWTTokenAuthenticator(t *testing.T) {
	suite.Run(t, new(JWTTokenAuthenticatorTestSuite))
}
