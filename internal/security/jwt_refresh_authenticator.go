package security

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

const (
	AuthTypeRefresh = "refresh"
)

// JWTRefreshAuthenticator exchanges a refresh token for the principal a new
// token pair is issued to. It reloads the user rather than trusting the token,
// so roles and status are current, and then re-applies the choices the login
// made (Principal.SessionChoices) through the providers that recorded them —
// the user record alone would otherwise replace them with its defaults.
type JWTRefreshAuthenticator struct {
	jwt        *security.JWT
	userLoader security.UserLoader
	// choosers are the SessionChoiceProviders among the challenge providers, in
	// the order the login resolves them, one per challenge type.
	choosers []security.SessionChoiceProvider
}

// NewJWTRefreshAuthenticator creates the refresh authenticator. providers are
// the application's challenge providers; the SessionChoiceProviders among them
// re-apply the choices a refresh token carries.
func NewJWTRefreshAuthenticator(
	jwt *security.JWT,
	userLoader security.UserLoader,
	providers []security.ChallengeProvider,
) security.Authenticator {
	return &JWTRefreshAuthenticator{
		jwt:        jwt,
		userLoader: userLoader,
		choosers:   sessionChoosers(providers),
	}
}

// sessionChoosers picks the SessionChoiceProviders out of providers, ordered and
// deduplicated by type the way the login flow finds a provider, so a choice is
// re-applied by the provider that recorded it.
func sessionChoosers(providers []security.ChallengeProvider) []security.SessionChoiceProvider {
	ordered := slices.Clone(providers)
	slices.SortStableFunc(ordered, func(a, b security.ChallengeProvider) int {
		return cmp.Compare(a.Order(), b.Order())
	})

	var (
		choosers []security.SessionChoiceProvider
		seen     = make(map[string]bool, len(ordered))
	)

	for _, provider := range ordered {
		if seen[provider.Type()] {
			continue
		}

		seen[provider.Type()] = true

		if chooser, ok := provider.(security.SessionChoiceProvider); ok {
			choosers = append(choosers, chooser)
		}
	}

	return choosers
}

func (*JWTRefreshAuthenticator) Supports(authType string) bool {
	return authType == AuthTypeRefresh
}

func (j *JWTRefreshAuthenticator) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	if j.userLoader == nil {
		return nil, result.ErrNotImplemented(i18n.T(security.ErrMessageUserLoaderNotImplemented))
	}

	token := authentication.Principal
	if token == "" {
		return nil, security.ErrTokenInvalid
	}

	claimsAccessor, err := j.jwt.Parse(token)
	if err != nil {
		logger.Warnf("JWT refresh token validation failed: %v", err)

		return nil, err
	}

	if claimsAccessor.Type() != security.TokenTypeRefresh {
		return nil, security.ErrTokenInvalid
	}

	subjectParts := strings.SplitN(claimsAccessor.Subject(), "@", 2)
	if len(subjectParts) < 2 {
		return nil, security.ErrTokenInvalid
	}

	userID := subjectParts[0]

	// Reload user to get latest permissions/status instead of relying on stale token data.
	principal, err := j.userLoader.LoadByID(ctx, userID)
	if err != nil {
		logger.Warnf("Failed to reload user by ID %q: %v", userID, err)

		return nil, err
	}

	if principal == nil {
		logger.Warnf("User not found by ID %q", userID)

		return nil, result.ErrRecordNotFound
	}

	return j.reapplyChoices(ctx, principal, claimsAccessor.SessionChoices())
}

// reapplyChoices re-applies the choices recorded at login to the reloaded
// principal, in login order, and records them on the result so the next refresh
// token carries them again. A choice that no registered provider can re-apply
// fails the refresh: the session it describes can no longer be rebuilt, so the
// user logs in again rather than continuing as somebody it never chose to be.
func (j *JWTRefreshAuthenticator) reapplyChoices(
	ctx context.Context,
	principal *security.Principal,
	choices map[string]string,
) (*security.Principal, error) {
	if len(choices) == 0 {
		return principal, nil
	}

	reapplied := make(map[string]string, len(choices))

	for _, chooser := range j.choosers {
		challengeType := chooser.Type()

		choice, ok := choices[challengeType]
		if !ok {
			continue
		}

		next, err := chooser.ReapplyChoice(ctx, principal, choice)
		if err != nil {
			logger.Warnf("Failed to re-apply %q choice for user %q on refresh: %v", challengeType, principal.ID, err)

			return nil, err
		}

		if next == nil {
			logger.Errorf("Refresh rejected: provider %q re-applied a choice to a nil principal", challengeType)

			return nil, security.ErrTokenInvalid
		}

		principal = next
		reapplied[challengeType] = choice
	}

	if len(reapplied) != len(choices) {
		logger.Warnf("Refresh rejected: user %q carries session choices no registered provider re-applies", principal.ID)

		return nil, security.ErrTokenInvalid
	}

	principal.SessionChoices = reapplied

	return principal, nil
}
