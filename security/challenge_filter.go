package security

import (
	"context"
	"slices"
)

// LoginFilter is a declarative, data-backed filter that scopes a login challenge
// to the logins it applies to. It captures the scoping question — "which kinds
// of login does this challenge belong to?" — as data rather than an opaque
// predicate, so a provider's scope reads off its registration site.
//
// Semantics: within one filter, an empty dimension is unconstrained; a populated
// AuthTypes matches when the login's mechanism is listed, and a populated
// ExcludedAuthTypes matches when it is not. Multiple filters passed to the same
// provider must all match (AND). Per-user predicates (whether the user has a
// TOTP secret, whether the password has expired) deliberately stay out — they
// belong in the provider's own evaluation.
type LoginFilter struct {
	AuthTypes         []string
	ExcludedAuthTypes []string
}

// ForAuthTypes restricts a challenge provider to logins made with the named
// mechanisms.
func ForAuthTypes(authTypes ...string) LoginFilter {
	return LoginFilter{AuthTypes: authTypes}
}

// ExceptAuthTypes exempts logins made with the named mechanisms from a challenge
// provider, which keeps applying to every other mechanism — including ones
// added later.
func ExceptAuthTypes(authTypes ...string) LoginFilter {
	return LoginFilter{ExcludedAuthTypes: authTypes}
}

// Matches reports whether login passes this filter.
func (f LoginFilter) Matches(login *LoginContext) bool {
	if len(f.AuthTypes) > 0 && !slices.Contains(f.AuthTypes, login.AuthType) {
		return false
	}

	return !slices.Contains(f.ExcludedAuthTypes, login.AuthType)
}

// NewFilteredChallengeProvider wraps a provider with declarative login filters,
// so its challenge is evaluated only for the logins it declared interest in;
// for any other login it is skipped exactly as if it had evaluated to nil.
// Providers apply to every login by default; hosts whose challenge belongs to
// some login mechanisms wrap it at registration:
//
//	vef.ProvideChallengeProvider(func(
//	    checker security.PasswordChangeChecker,
//	    changer security.PasswordChanger,
//	    validator security.PasswordValidator,
//	) security.ChallengeProvider {
//	    return security.NewFilteredChallengeProvider(
//	        security.NewPasswordChangeChallengeProvider(checker, changer, validator),
//	        security.ForAuthTypes(security.AuthTypePassword),
//	    )
//	})
//
// Choose the filter by what the challenge guards. A challenge tied to one
// credential suits an allow-list: a forced password change concerns the
// password, so ForAuthTypes(AuthTypePassword) keeps it off logins that never
// presented one. A second factor should use a deny-list: exempt the mechanisms
// that already carry equivalent assurance with ExceptAuthTypes, so a login
// mechanism added later is still challenged instead of silently exempt.
//
// No filters returns the provider unchanged.
func NewFilteredChallengeProvider(provider ChallengeProvider, filters ...LoginFilter) ChallengeProvider {
	if len(filters) == 0 {
		return provider
	}

	return &filteredChallengeProvider{inner: provider, filters: filters}
}

type filteredChallengeProvider struct {
	inner   ChallengeProvider
	filters []LoginFilter
}

func (p *filteredChallengeProvider) Type() string { return p.inner.Type() }
func (p *filteredChallengeProvider) Order() int   { return p.inner.Order() }

// Evaluate forwards to the wrapped provider when the login matches every
// filter, and skips the challenge otherwise.
func (p *filteredChallengeProvider) Evaluate(ctx context.Context, login *LoginContext) (*LoginChallenge, error) {
	for _, filter := range p.filters {
		if !filter.Matches(login) {
			return nil, nil
		}
	}

	return p.inner.Evaluate(ctx, login)
}

// Resolve forwards to the wrapped provider. It needs no filtering of its own:
// the login flow resolves only the challenge Evaluate presented, and a login's
// mechanism never changes between its steps.
func (p *filteredChallengeProvider) Resolve(ctx context.Context, login *LoginContext, response any) (*Principal, error) {
	return p.inner.Resolve(ctx, login, response)
}
