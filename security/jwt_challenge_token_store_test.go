package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/result"
)

type JWTChallengeTokenStoreTestSuite struct {
	suite.Suite

	jwt   *JWT
	store *JWTChallengeTokenStore
}

func (s *JWTChallengeTokenStoreTestSuite) SetupSuite() {
	jwt, err := NewJWT(&JWTConfig{
		Secret:   DefaultJWTSecret,
		Audience: DefaultJWTAudience,
	})
	s.Require().NoError(err, "Should create JWT instance without error")

	s.jwt = jwt
	s.store = NewJWTChallengeTokenStore(jwt).(*JWTChallengeTokenStore)
}

// TestGenerate verifies challenge token generation scenarios.
func (s *JWTChallengeTokenStoreTestSuite) TestGenerate() {
	s.Run("WithPendingAndResolved", func() {
		principal := NewUser("user1", "Alice", "admin")
		token, err := s.store.Generate(principal, []string{"totp", "department"}, []string{"sms"})
		s.Require().NoError(err, "Should generate token without error")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithNilResolved", func() {
		principal := NewUser("user2", "Bob")
		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithEmptySlices", func() {
		principal := NewUser("user3", "Charlie")
		token, err := s.store.Generate(principal, []string{}, []string{})
		s.Require().NoError(err, "Should generate token with empty slices")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithDetails", func() {
		principal := NewUser("user4", "Diana", "editor")
		principal.Details = map[string]any{"department": "engineering", "level": 3}

		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token with details")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithNoRoles", func() {
		principal := NewUser("user5", "Eve")
		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without roles")
		s.NotEmpty(token, "Should return a non-empty token")
	})
}

// TestParse verifies challenge token parsing and error handling.
func (s *JWTChallengeTokenStoreTestSuite) TestParse() {
	s.Run("RoundTripWithFullState", func() {
		principal := NewUser("user1", "Alice", "admin", "editor")
		pending := []string{"totp", "department"}
		resolved := []string{"sms"}

		token, err := s.store.Generate(principal, pending, resolved)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Equal("user1", state.Principal.ID, "Should preserve principal ID")
		s.Equal("Alice", state.Principal.Name, "Should preserve principal name")
		s.Equal(PrincipalTypeUser, state.Principal.Type, "Should create user principal")
		s.Equal([]string{"admin", "editor"}, state.Principal.Roles, "Should preserve roles")
		s.Equal(pending, state.Pending, "Should preserve pending list")
		s.Equal(resolved, state.Resolved, "Should preserve resolved list")
	})

	s.Run("WithNilResolved", func() {
		principal := NewUser("user2", "Bob")
		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Equal([]string{"totp"}, state.Pending, "Should preserve pending list")
		s.Empty(state.Resolved, "Should have empty resolved list")
	})

	s.Run("WithNoRoles", func() {
		principal := NewUser("user3", "Charlie")
		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Empty(state.Principal.Roles, "Should have empty roles")
	})

	s.Run("PreservesExternalAppPrincipalType", func() {
		principal := NewExternalApp("app1", "Payment API", "service")
		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Equal(PrincipalTypeExternalApp, state.Principal.Type, "Should preserve principal type")
		s.Equal("app1", state.Principal.ID, "Should preserve principal ID")
		s.Equal("Payment API", state.Principal.Name, "Should preserve principal name")
		s.Equal([]string{"service"}, state.Principal.Roles, "Should preserve principal roles")
	})

	s.Run("WithDetails", func() {
		principal := NewUser("user4", "Diana", "admin")
		principal.Details = map[string]any{"department": "engineering"}

		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.NotNil(state.Principal.Details, "Should preserve details")
	})

	s.Run("SubjectWithAtSignInName", func() {
		principal := NewUser("user5", "user@example.com")
		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Equal("user5", state.Principal.ID, "Should extract ID before first @")
		s.Equal("user@example.com", state.Principal.Name, "Should preserve name with @ sign")
	})

	s.Run("RejectsInvalidToken", func() {
		_, err := s.store.Parse("invalid.token.string")
		s.Require().Error(err, "Should reject malformed token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsEmptyToken", func() {
		_, err := s.store.Parse("")
		s.Require().Error(err, "Should reject empty token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsAccessToken", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("user@name").
			WithType(TokenTypeAccess)
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate access token without error")

		_, err = s.store.Parse(token)
		s.Require().Error(err, "Should reject access token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsRefreshToken", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("user@name").
			WithType(TokenTypeRefresh)
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate refresh token without error")

		_, err = s.store.Parse(token)
		s.Require().Error(err, "Should reject refresh token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsTokenWithoutType", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("user@name")
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate token without error")

		_, err = s.store.Parse(token)
		s.Require().Error(err, "Should reject token without type")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsExpiredToken", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("user@name").
			WithType(TokenTypeChallenge)
		token, err := s.jwt.Generate(claimsBuilder, -1*time.Hour, 0)
		s.Require().NoError(err, "Should generate expired token without error")

		_, err = s.store.Parse(token)
		s.Require().Error(err, "Should reject expired token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenExpired, resErr.Code, "Should return token expired error code")
	})

	s.Run("RejectsMalformedSubject", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("no-at-sign").
			WithType(TokenTypeChallenge)
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate token without error")

		_, err = s.store.Parse(token)
		s.Require().Error(err, "Should reject token with malformed subject")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsEmptySubject", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("").
			WithType(TokenTypeChallenge)
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate token without error")

		_, err = s.store.Parse(token)
		s.Require().Error(err, "Should reject token with empty subject")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsUnknownPrincipalType", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("user@name").
			WithType(TokenTypeChallenge).
			WithClaim(ClaimChallengePrincipalType, "unknown_type")
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate token without error")

		_, err = s.store.Parse(token)
		s.Require().Error(err, "Should reject token with unknown principal type")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("DefaultsToUserWhenTypeClaimMissing", func() {
		claimsBuilder := NewJWTClaimsBuilder().
			WithSubject("user@name").
			WithType(TokenTypeChallenge)
		token, err := s.jwt.Generate(claimsBuilder, 5*time.Minute, 0)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")
		s.Equal(PrincipalTypeUser, state.Principal.Type, "Should default to user for backward compatibility")
	})
}

func TestJWTChallengeTokenStore(t *testing.T) {
	suite.Run(t, new(JWTChallengeTokenStoreTestSuite))
}
