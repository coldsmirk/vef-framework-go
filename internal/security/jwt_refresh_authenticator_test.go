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
	auth := NewJWTRefreshAuthenticator(s.jwt, nil, nil)
	s.True(auth.Supports(AuthTypeRefresh), "Should support refresh type")
	s.False(auth.Supports("token"), "Should not support token type")
	s.False(auth.Supports(""), "Should not support empty type")
}

// TestAuthenticate verifies all authentication paths.
func (s *JWTRefreshAuthenticatorTestSuite) TestAuthenticate() {
	ctx := context.Background()

	s.Run("NilUserLoader", func() {
		auth := NewJWTRefreshAuthenticator(s.jwt, nil, nil)

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
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: "",
		})
		s.Require().Error(err, "Should return error for empty token")
	})

	s.Run("InvalidJWT", func() {
		loader := new(MockUserLoader)
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: "invalid.jwt.token",
		})
		s.Require().Error(err, "Should return error for invalid JWT")
	})

	s.Run("MalformedSubjectNoAtSign", func() {
		// Craft a refresh token whose subject has no '@' separator.
		claimsBuilder := security.NewJWTClaimsBuilder().
			WithSubject("malformedsubject").
			WithType(security.TokenTypeRefresh)
		token, err := s.jwt.Generate(claimsBuilder, time.Hour, 0)
		s.Require().NoError(err, "Should generate token with malformed subject")

		loader := new(MockUserLoader)
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: token,
		})
		s.Require().Error(err, "Should reject token with malformed subject")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(security.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid code")
	})

	s.Run("AccessTokenRejected", func() {
		principal := security.NewUser("user1", "Alice")
		tokens, err := s.gen.Generate(context.Background(), principal, security.SessionMeta{})
		s.Require().NoError(err, "Should generate tokens")

		loader := new(MockUserLoader)
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: tokens.AccessToken,
		})
		s.Require().Error(err, "Should reject access token")
	})

	s.Run("UserNotFoundOnReload", func() {
		principal := security.NewUser("user1", "Alice")
		tokens, err := s.gen.Generate(context.Background(), principal, security.SessionMeta{})
		s.Require().NoError(err, "Should generate tokens")

		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(nil, nil)

		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: tokens.RefreshToken,
		})
		s.Require().Error(err, "Should return error when user not found on reload")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsError", func() {
		principal := security.NewUser("user1", "Alice")
		tokens, err := s.gen.Generate(context.Background(), principal, security.SessionMeta{})
		s.Require().NoError(err, "Should generate tokens")

		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(nil, errors.New("db error"))

		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

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
		tokens, err := s.gen.Generate(context.Background(), original, security.SessionMeta{})
		s.Require().NoError(err, "Should generate tokens")

		// Reload returns updated principal with new roles
		reloaded := security.NewUser("user1", "Alice", "admin", "editor")
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(reloaded, nil)

		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

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
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, nil)

		_, err = auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: token,
		})
		s.Require().Error(err, "Should reject expired refresh token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(security.ErrCodeTokenExpired, resErr.Code, "Should return token expired code")
	})
}

// StubChoiceProvider is a SessionChoiceProvider whose re-application answers
// with preset values and records the order it was reached in.
type StubChoiceProvider struct {
	ChallengeType  string
	ChallengeOrder int
	Principal      *security.Principal
	Err            error
	// ReturnsNil makes ReapplyChoice break its contract by answering no
	// principal and no error.
	ReturnsNil bool

	Reapplied []string
	Log       *[]string
}

func (p *StubChoiceProvider) Type() string { return p.ChallengeType }
func (p *StubChoiceProvider) Order() int   { return p.ChallengeOrder }

func (*StubChoiceProvider) Evaluate(context.Context, *security.LoginContext) (*security.LoginChallenge, error) {
	return nil, nil
}

func (*StubChoiceProvider) Resolve(_ context.Context, login *security.LoginContext, _ any) (*security.Principal, error) {
	return login.Principal, nil
}

func (*StubChoiceProvider) SessionChoice(response any) (string, bool) {
	choice, ok := response.(string)

	return choice, ok
}

func (p *StubChoiceProvider) ReapplyChoice(_ context.Context, principal *security.Principal, choice string) (*security.Principal, error) {
	p.Reapplied = append(p.Reapplied, choice)
	if p.Log != nil {
		*p.Log = append(*p.Log, p.ChallengeType)
	}

	if p.ReturnsNil || p.Err != nil || p.Principal != nil {
		return p.Principal, p.Err
	}

	return principal, nil
}

// StubPlainProvider is a ChallengeProvider whose answer is no session choice.
type StubPlainProvider struct {
	ChallengeType string
}

func (p *StubPlainProvider) Type() string { return p.ChallengeType }
func (*StubPlainProvider) Order() int     { return 0 }

func (*StubPlainProvider) Evaluate(context.Context, *security.LoginContext) (*security.LoginChallenge, error) {
	return nil, nil
}

func (*StubPlainProvider) Resolve(_ context.Context, login *security.LoginContext, _ any) (*security.Principal, error) {
	return login.Principal, nil
}

// refreshTokenWith issues a refresh token for user1 carrying choices.
func (s *JWTRefreshAuthenticatorTestSuite) refreshTokenWith(choices map[string]string) string {
	principal := security.NewUser("user1", "Alice")
	principal.SessionChoices = choices

	tokens, err := s.gen.Generate(context.Background(), principal, security.SessionMeta{})
	s.Require().NoError(err, "Should generate tokens")

	return tokens.RefreshToken
}

// TestReapplySessionChoices verifies that a refresh rebuilds the session the
// login chose rather than the one the user record alone yields.
func (s *JWTRefreshAuthenticatorTestSuite) TestReapplySessionChoices() {
	ctx := context.Background()
	departmentChoice := map[string]string{security.ChallengeTypeDepartmentSelection: "dept-2"}

	s.Run("ReappliesTheChoiceMadeAtLogin", func() {
		reloaded := security.NewUser("user1", "Alice")
		reloaded.Details = map[string]any{"departmentId": "dept-default"}
		chosen := security.NewUser("user1", "Alice")
		chosen.Details = map[string]any{"departmentId": "dept-2"}

		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(reloaded, nil)

		chooser := &StubChoiceProvider{ChallengeType: security.ChallengeTypeDepartmentSelection, Principal: chosen}
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, []security.ChallengeProvider{chooser})

		got, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeRefresh, Principal: s.refreshTokenWith(departmentChoice)})

		s.Require().NoError(err, "A choice the user can still make should refresh")
		s.Equal([]string{"dept-2"}, chooser.Reapplied, "The department chosen at login should be re-applied")
		s.Equal(map[string]any{"departmentId": "dept-2"}, got.Details, "The refreshed session should keep the chosen department, not the default")
		s.Equal(departmentChoice, got.SessionChoices, "The choice should be kept for the next refresh")
	})

	s.Run("ReappliesThroughAFilteredProvider", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(security.NewUser("user1", "Alice"), nil)

		chooser := &StubChoiceProvider{ChallengeType: security.ChallengeTypeDepartmentSelection}
		filtered := security.NewFilteredChallengeProvider(chooser, security.ForAuthTypes(security.AuthTypePassword))
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, []security.ChallengeProvider{filtered})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeRefresh, Principal: s.refreshTokenWith(departmentChoice)})

		s.Require().NoError(err, "A filtered provider should still re-apply its choice")
		s.Equal([]string{"dept-2"}, chooser.Reapplied, "The wrapped provider should receive the choice")
	})

	s.Run("ReappliesInLoginOrder", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(security.NewUser("user1", "Alice"), nil)

		var order []string

		late := &StubChoiceProvider{ChallengeType: "late", ChallengeOrder: 20, Log: &order}
		early := &StubChoiceProvider{ChallengeType: "early", ChallengeOrder: 10, Log: &order}
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, []security.ChallengeProvider{late, early})

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeRefresh,
			Principal: s.refreshTokenWith(map[string]string{"late": "b", "early": "a"}),
		})

		s.Require().NoError(err, "Every recorded choice should be re-applied")
		s.Equal([]string{"early", "late"}, order, "Choices should be re-applied in the order the login resolves them")
	})

	s.Run("SessionWithoutChoicesReachesNoProvider", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(security.NewUser("user1", "Alice"), nil)

		chooser := &StubChoiceProvider{ChallengeType: security.ChallengeTypeDepartmentSelection}
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, []security.ChallengeProvider{chooser})

		got, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeRefresh, Principal: s.refreshTokenWith(nil)})

		s.Require().NoError(err, "A session that chose nothing should refresh")
		s.Empty(chooser.Reapplied, "No choice should be re-applied when none was made")
		s.Nil(got.SessionChoices, "The refreshed session should still carry no choices")
	})

	s.Run("RefusesAChoiceNoProviderReapplies", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(security.NewUser("user1", "Alice"), nil)

		plain := &StubPlainProvider{ChallengeType: security.ChallengeTypeDepartmentSelection}
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, []security.ChallengeProvider{plain})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeRefresh, Principal: s.refreshTokenWith(departmentChoice)})

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(security.ErrCodeTokenInvalid, resErr.Code, "A choice that cannot be re-applied should end the session")
	})

	s.Run("PropagatesARefusedChoice", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(security.NewUser("user1", "Alice"), nil)

		refusal := result.Err("department no longer available")
		chooser := &StubChoiceProvider{ChallengeType: security.ChallengeTypeDepartmentSelection, Err: refusal}
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, []security.ChallengeProvider{chooser})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeRefresh, Principal: s.refreshTokenWith(departmentChoice)})

		s.Require().ErrorIs(err, refusal, "A department the user has lost should fail the refresh with the provider's error")
	})

	s.Run("RefusesAProviderAnsweringNoPrincipal", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByID", mock.Anything, "user1").Return(security.NewUser("user1", "Alice"), nil)

		chooser := &StubChoiceProvider{ChallengeType: security.ChallengeTypeDepartmentSelection, ReturnsNil: true}
		auth := NewJWTRefreshAuthenticator(s.jwt, loader, []security.ChallengeProvider{chooser})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeRefresh, Principal: s.refreshTokenWith(departmentChoice)})

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(security.ErrCodeTokenInvalid, resErr.Code, "A provider answering no principal should end the session")
	})
}

func TestJWTRefreshAuthenticator(t *testing.T) {
	suite.Run(t, new(JWTRefreshAuthenticatorTestSuite))
}
