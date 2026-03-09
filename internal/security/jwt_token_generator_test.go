package security

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/security"
)

type JWTTokenGeneratorTestSuite struct {
	suite.Suite

	jwt       *security.JWT
	generator *JWTTokenGenerator
}

func (s *JWTTokenGeneratorTestSuite) SetupSuite() {
	s.jwt = newTestJWT(s.T())
	s.generator = newTestTokenGenerator(s.jwt).(*JWTTokenGenerator)
}

// TestGenerate verifies token pair generation.
func (s *JWTTokenGeneratorTestSuite) TestGenerate() {
	s.Run("WithRolesAndDetails", func() {
		principal := security.NewUser("user1", "Alice", "admin", "editor")
		principal.Details = map[string]any{"department": "engineering"}

		tokens, err := s.generator.Generate(principal)
		s.Require().NoError(err, "Should generate tokens without error")
		s.Require().NotNil(tokens, "Should return non-nil tokens")
		s.NotEmpty(tokens.AccessToken, "Should have non-empty access token")
		s.NotEmpty(tokens.RefreshToken, "Should have non-empty refresh token")
	})

	s.Run("WithNoRoles", func() {
		principal := security.NewUser("user2", "Bob")

		tokens, err := s.generator.Generate(principal)
		s.Require().NoError(err, "Should generate tokens without error")
		s.Require().NotNil(tokens, "Should return non-nil tokens")
		s.NotEmpty(tokens.AccessToken, "Should have non-empty access token")
		s.NotEmpty(tokens.RefreshToken, "Should have non-empty refresh token")
	})

	s.Run("WithNoDetails", func() {
		principal := security.NewUser("user3", "Charlie", "viewer")

		tokens, err := s.generator.Generate(principal)
		s.Require().NoError(err, "Should generate tokens without error")
		s.Require().NotNil(tokens, "Should return non-nil tokens")
	})

	s.Run("ExternalAppPrincipal", func() {
		principal := security.NewExternalApp("app1", "MyApp", "api_access")

		tokens, err := s.generator.Generate(principal)
		s.Require().NoError(err, "Should generate tokens for external app")
		s.Require().NotNil(tokens, "Should return non-nil tokens")
		s.NotEmpty(tokens.AccessToken, "Should have non-empty access token")
		s.NotEmpty(tokens.RefreshToken, "Should have non-empty refresh token")
	})

	s.Run("FailsWithNonSerializableDetails", func() {
		principal := security.NewUser("user4", "Diana")
		principal.Details = make(chan int) // channels cannot be JSON-marshaled

		tokens, err := s.generator.Generate(principal)
		s.Error(err, "Should fail when details cannot be serialized")
		s.Nil(tokens, "Should return nil tokens on error")
	})
}

// TestAccessTokenClaims verifies claims encoded in the access token.
func (s *JWTTokenGeneratorTestSuite) TestAccessTokenClaims() {
	s.Run("SubjectAndType", func() {
		principal := security.NewUser("user1", "Alice", "admin", "editor")
		principal.Details = map[string]any{"department": "engineering"}

		tokens, err := s.generator.Generate(principal)
		s.Require().NoError(err, "Should generate tokens without error")

		claims, err := s.jwt.Parse(tokens.AccessToken)
		s.Require().NoError(err, "Should parse access token without error")

		s.Equal("user1@Alice", claims.Subject(), "Should encode id@name in subject")
		s.Equal(security.TokenTypeAccess, claims.Type(), "Should have access token type")
		s.NotEmpty(claims.ID(), "Should have JWT ID")
	})

	s.Run("IncludesRolesAndDetails", func() {
		principal := security.NewUser("user2", "Bob", "admin", "editor")
		principal.Details = map[string]any{"level": 5}

		tokens, err := s.generator.Generate(principal)
		s.Require().NoError(err, "Should generate tokens without error")

		claims, err := s.jwt.Parse(tokens.AccessToken)
		s.Require().NoError(err, "Should parse access token without error")

		s.Equal([]string{"admin", "editor"}, claims.Roles(), "Should include roles")
		s.NotNil(claims.Details(), "Should include details")
	})

	s.Run("EmptyRolesAndNilDetails", func() {
		principal := security.NewUser("user3", "Charlie")

		tokens, err := s.generator.Generate(principal)
		s.Require().NoError(err, "Should generate tokens without error")

		claims, err := s.jwt.Parse(tokens.AccessToken)
		s.Require().NoError(err, "Should parse access token without error")

		s.Empty(claims.Roles(), "Should have empty roles")
		s.Nil(claims.Details(), "Should have nil details")
	})
}

// TestRefreshTokenClaims verifies refresh token carries only essential claims.
func (s *JWTTokenGeneratorTestSuite) TestRefreshTokenClaims() {
	principal := security.NewUser("user1", "Alice", "admin")
	principal.Details = map[string]any{"department": "engineering"}

	tokens, err := s.generator.Generate(principal)
	s.Require().NoError(err, "Should generate tokens without error")

	claims, err := s.jwt.Parse(tokens.RefreshToken)
	s.Require().NoError(err, "Should parse refresh token without error")

	s.Equal("user1@Alice", claims.Subject(), "Should encode id@name in subject")
	s.Equal(security.TokenTypeRefresh, claims.Type(), "Should have refresh token type")
	s.NotEmpty(claims.ID(), "Should have JWT ID")
	s.Empty(claims.Roles(), "Should not include roles in refresh token")
	s.Nil(claims.Details(), "Should not include details in refresh token")
}

// TestSharedJWTID verifies that access and refresh tokens share the same JWT ID.
func (s *JWTTokenGeneratorTestSuite) TestSharedJWTID() {
	principal := security.NewUser("user1", "Alice", "admin")

	tokens, err := s.generator.Generate(principal)
	s.Require().NoError(err, "Should generate tokens without error")

	accessClaims, err := s.jwt.Parse(tokens.AccessToken)
	s.Require().NoError(err, "Should parse access token without error")

	refreshClaims, err := s.jwt.Parse(tokens.RefreshToken)
	s.Require().NoError(err, "Should parse refresh token without error")

	s.Equal(accessClaims.ID(), refreshClaims.ID(), "Should share JWT ID between access and refresh tokens")
}

// TestSubjectWithAtSignInName verifies SplitN-compatible subject encoding.
func (s *JWTTokenGeneratorTestSuite) TestSubjectWithAtSignInName() {
	principal := security.NewUser("user1", "user@example.com", "admin")

	tokens, err := s.generator.Generate(principal)
	s.Require().NoError(err, "Should generate tokens without error")

	claims, err := s.jwt.Parse(tokens.AccessToken)
	s.Require().NoError(err, "Should parse access token without error")

	s.Equal("user1@user@example.com", claims.Subject(), "Should encode full id@name in subject")
}

func TestJWTTokenGenerator(t *testing.T) {
	suite.Run(t, new(JWTTokenGeneratorTestSuite))
}
