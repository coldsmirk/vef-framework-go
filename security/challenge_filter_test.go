package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StubChallengeProvider answers with preset values and records what a filtered
// provider forwards to it.
type StubChallengeProvider struct {
	ChallengeType  string
	ChallengeOrder int
	Challenge      *LoginChallenge
	Principal      *Principal
	Err            error

	Evaluated []*LoginContext
	Resolved  []*LoginContext
	Responses []any
}

func (p *StubChallengeProvider) Type() string { return p.ChallengeType }
func (p *StubChallengeProvider) Order() int   { return p.ChallengeOrder }

func (p *StubChallengeProvider) Evaluate(_ context.Context, login *LoginContext) (*LoginChallenge, error) {
	p.Evaluated = append(p.Evaluated, login)

	return p.Challenge, p.Err
}

func (p *StubChallengeProvider) Resolve(_ context.Context, login *LoginContext, response any) (*Principal, error) {
	p.Resolved = append(p.Resolved, login)
	p.Responses = append(p.Responses, response)

	return p.Principal, p.Err
}

func TestLoginFilterMatches(t *testing.T) {
	const miniProgram = "wechat_mini"

	both := LoginFilter{
		AuthTypes:         []string{AuthTypePassword, AuthTypeTrustCode},
		ExcludedAuthTypes: []string{AuthTypeTrustCode},
	}

	tests := []struct {
		name     string
		filter   LoginFilter
		authType string
		want     bool
	}{
		{name: "AllowListHit", filter: ForAuthTypes(AuthTypePassword, miniProgram), authType: miniProgram, want: true},
		{name: "AllowListMiss", filter: ForAuthTypes(AuthTypePassword), authType: AuthTypeTrustCode, want: false},
		{name: "AllowListIsCaseSensitive", filter: ForAuthTypes(AuthTypePassword), authType: "Password", want: false},
		{name: "AllowListMissesEmptyAuthType", filter: ForAuthTypes(AuthTypePassword), authType: "", want: false},
		{name: "DenyListHit", filter: ExceptAuthTypes(AuthTypeTrustCode), authType: AuthTypeTrustCode, want: false},
		{name: "DenyListMiss", filter: ExceptAuthTypes(AuthTypeTrustCode), authType: miniProgram, want: true},
		{name: "ZeroFilterIsUnconstrained", filter: LoginFilter{}, authType: miniProgram, want: true},
		{name: "EmptyListsAreUnconstrained", filter: LoginFilter{AuthTypes: []string{}, ExcludedAuthTypes: []string{}}, authType: AuthTypePassword, want: true},
		{name: "NoAllowedTypesIsUnconstrained", filter: ForAuthTypes(), authType: AuthTypeTrustCode, want: true},
		{name: "NoExcludedTypesIsUnconstrained", filter: ExceptAuthTypes(), authType: AuthTypeTrustCode, want: true},
		{name: "BothDimensionsAllowedAndNotExcluded", filter: both, authType: AuthTypePassword, want: true},
		{name: "BothDimensionsAllowedButExcluded", filter: both, authType: AuthTypeTrustCode, want: false},
		{name: "BothDimensionsNotAllowed", filter: both, authType: miniProgram, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.filter.Matches(&LoginContext{AuthType: tt.authType}),
				"Matches should report %v for auth type %q", tt.want, tt.authType)
		})
	}
}

func TestNewFilteredChallengeProvider(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	passwordLogin := &LoginContext{AuthType: AuthTypePassword, Username: "alice", Principal: principal}
	trustLogin := &LoginContext{AuthType: AuthTypeTrustCode, Username: "his", Principal: principal}

	t.Run("NoFiltersReturnsTheProvider", func(t *testing.T) {
		inner := new(StubChallengeProvider)

		assert.Same(t, inner, NewFilteredChallengeProvider(inner), "Without filters the provider should be returned unwrapped")
	})

	t.Run("FilteredOutLoginSkipsTheChallenge", func(t *testing.T) {
		inner := &StubChallengeProvider{Challenge: &LoginChallenge{Type: ChallengeTypePasswordChange, Required: true}}
		provider := NewFilteredChallengeProvider(inner, ForAuthTypes(AuthTypePassword))

		challenge, err := provider.Evaluate(ctx, trustLogin)

		require.NoError(t, err, "A filtered-out login should not error")
		assert.Nil(t, challenge, "A filtered-out login should skip the challenge")
		assert.Empty(t, inner.Evaluated, "A filtered-out login must never reach the wrapped provider")
	})

	t.Run("MatchingLoginIsForwarded", func(t *testing.T) {
		inner := &StubChallengeProvider{Challenge: &LoginChallenge{Type: ChallengeTypePasswordChange, Required: true}}
		provider := NewFilteredChallengeProvider(inner, ForAuthTypes(AuthTypePassword))

		challenge, err := provider.Evaluate(ctx, passwordLogin)

		require.NoError(t, err, "A matching login should not error")
		assert.Same(t, inner.Challenge, challenge, "A matching login should get the wrapped provider's challenge")
		require.Len(t, inner.Evaluated, 1, "A matching login should reach the wrapped provider once")
		assert.Same(t, passwordLogin, inner.Evaluated[0], "The wrapped provider should see the login as given")
	})

	t.Run("MatchingLoginPropagatesError", func(t *testing.T) {
		evalErr := errors.New("evaluate failed")
		inner := &StubChallengeProvider{Err: evalErr}
		provider := NewFilteredChallengeProvider(inner, ExceptAuthTypes(AuthTypeTrustCode))

		challenge, err := provider.Evaluate(ctx, passwordLogin)

		require.ErrorIs(t, err, evalErr, "A matching login should propagate the wrapped provider's error")
		assert.Nil(t, challenge, "A failed evaluation should present no challenge")
	})

	t.Run("FiltersMustAllMatch", func(t *testing.T) {
		inner := &StubChallengeProvider{Challenge: &LoginChallenge{Type: ChallengeTypeTOTP, Required: true}}
		provider := NewFilteredChallengeProvider(inner,
			ForAuthTypes(AuthTypePassword, AuthTypeTrustCode),
			ExceptAuthTypes(AuthTypeTrustCode),
		)

		accepted, err := provider.Evaluate(ctx, passwordLogin)
		require.NoError(t, err, "A login every filter accepts should not error")
		assert.NotNil(t, accepted, "A login every filter accepts should be challenged")

		rejected, err := provider.Evaluate(ctx, trustLogin)
		require.NoError(t, err, "A login one filter rejects should not error")
		assert.Nil(t, rejected, "A login one filter rejects should be skipped even though another filter accepts it")

		require.Len(t, inner.Evaluated, 1, "Only the login every filter accepts should reach the wrapped provider")
		assert.Same(t, passwordLogin, inner.Evaluated[0], "The login that reached the wrapped provider should be the accepted one")
	})

	t.Run("DelegatesTypeOrderAndResolve", func(t *testing.T) {
		enriched := NewUser("u1", "Alice (Engineering)")
		inner := &StubChallengeProvider{ChallengeType: ChallengeTypeTOTP, ChallengeOrder: 42, Principal: enriched}
		provider := NewFilteredChallengeProvider(inner, ForAuthTypes(AuthTypePassword))

		assert.Equal(t, ChallengeTypeTOTP, provider.Type(), "Type should come from the wrapped provider")
		assert.Equal(t, 42, provider.Order(), "Order should come from the wrapped provider")

		resolved, err := provider.Resolve(ctx, passwordLogin, "123456")

		require.NoError(t, err, "Resolve should not error when the wrapped provider accepts")
		assert.Same(t, enriched, resolved, "Resolve should return the wrapped provider's principal")
		require.Len(t, inner.Resolved, 1, "Resolve should reach the wrapped provider once")
		assert.Same(t, passwordLogin, inner.Resolved[0], "Resolve should forward the login as given")
		assert.Equal(t, []any{"123456"}, inner.Responses, "Resolve should forward the response as given")
	})

	t.Run("ResolvePropagatesError", func(t *testing.T) {
		resolveErr := errors.New("resolve failed")
		inner := &StubChallengeProvider{Err: resolveErr}
		provider := NewFilteredChallengeProvider(inner, ForAuthTypes(AuthTypePassword))

		resolved, err := provider.Resolve(ctx, passwordLogin, "000000")

		require.ErrorIs(t, err, resolveErr, "Resolve should propagate the wrapped provider's error")
		assert.Nil(t, resolved, "A rejected response should continue with no principal")
	})
}
