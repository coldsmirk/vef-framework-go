package security

import (
	"context"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/id"
)

const (
	ChallengeTokenExpires       = 5 * time.Minute
	ClaimChallengePending       = "pnd"
	ClaimChallengePrincipalType = "ptp"
	ClaimChallengeResolved      = "rsd"
	// ClaimChallengePrincipalName stores the principal Name as a dedicated claim;
	// the subject carries only the principal ID.
	ClaimChallengePrincipalName = "pnm"
	// ClaimChallengeUsername stores the original login identifier so audit events
	// emitted after a challenge report the same identifier as the initial login.
	ClaimChallengeUsername = "unm"
	// ClaimChallengeAuthType stores the login mechanism, so every later step is
	// evaluated, resolved and audited against the login it belongs to.
	ClaimChallengeAuthType = "atp"
)

// JWTChallengeTokenStore implements ChallengeTokenStore using stateless JWT tokens.
// Challenge state (principal, pending/resolved types) is encoded directly in the token,
// avoiding server-side session storage.
type JWTChallengeTokenStore struct {
	jwt *JWT
}

// NewJWTChallengeTokenStore creates a new JWT-based challenge token store.
func NewJWTChallengeTokenStore(jwt *JWT) ChallengeTokenStore {
	return &JWTChallengeTokenStore{jwt: jwt}
}

func (s *JWTChallengeTokenStore) Generate(_ context.Context, state *ChallengeState) (string, error) {
	principal := state.Principal
	claimsBuilder := NewJWTClaimsBuilder().
		WithID(id.GenerateUUID()).
		WithSubject(principal.ID).
		WithRoles(principal.Roles).
		WithDetails(principal.Details).
		WithType(TokenTypeChallenge).
		WithClaim(ClaimChallengePrincipalType, principal.Type).
		WithClaim(ClaimChallengePrincipalName, principal.Name).
		WithClaim(ClaimChallengeAuthType, state.AuthType).
		WithClaim(ClaimChallengeUsername, state.Username).
		WithClaim(ClaimChallengePending, state.Pending).
		WithClaim(ClaimChallengeResolved, state.Resolved)

	return s.jwt.Generate(claimsBuilder, ChallengeTokenExpires, 0)
}

func (s *JWTChallengeTokenStore) Parse(_ context.Context, token string) (*ChallengeState, error) {
	claimsAccessor, err := s.jwt.Parse(token)
	if err != nil {
		return nil, err
	}

	if claimsAccessor.Type() != TokenTypeChallenge {
		return nil, ErrTokenInvalid
	}

	principalID := claimsAccessor.Subject()
	if principalID == "" {
		return nil, ErrTokenInvalid
	}

	authType := cast.ToString(claimsAccessor.Claim(ClaimChallengeAuthType))
	if authType == "" {
		return nil, ErrTokenInvalid
	}

	principalName := cast.ToString(claimsAccessor.Claim(ClaimChallengePrincipalName))
	principalType := PrincipalType(cast.ToString(claimsAccessor.Claim(ClaimChallengePrincipalType)))

	var principal *Principal
	switch principalType {
	case PrincipalTypeUser:
		principal = NewUser(principalID, principalName, claimsAccessor.Roles()...)
	case PrincipalTypeExternalApp:
		principal = NewExternalApp(principalID, principalName, claimsAccessor.Roles()...)

	// PrincipalTypeSystem is deliberately absent: a challenge token carrying the
	// framework's internal identity has no legitimate origin, since no
	// authenticator may produce one to start a challenge with.
	default:
		return nil, ErrTokenInvalid
	}

	// Catches reserved ids smuggled under a user or external-app type.
	if principal.IsReserved() {
		return nil, ErrTokenInvalid
	}

	principal.AttemptUnmarshalDetails(claimsAccessor.Details())

	return &ChallengeState{
		AuthType:  authType,
		Username:  cast.ToString(claimsAccessor.Claim(ClaimChallengeUsername)),
		Principal: principal,
		Resolved:  cast.ToStringSlice(claimsAccessor.Claim(ClaimChallengeResolved)),
		Pending:   cast.ToStringSlice(claimsAccessor.Claim(ClaimChallengePending)),
	}, nil
}
