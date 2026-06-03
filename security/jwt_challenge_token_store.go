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
	// ClaimChallengePrincipalName stores the principal Name as a dedicated claim,
	// removing the fragile "@"-delimited subject encoding.
	// PUBLIC BEHAVIOR CHANGE: tokens generated after this change use a separate "pnm" claim
	// for Name; the subject carries only the principal ID. Tokens issued before this change
	// cannot be parsed (they will fail the empty-subject guard in Parse).
	ClaimChallengePrincipalName = "pnm"
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

func (s *JWTChallengeTokenStore) Generate(_ context.Context, principal *Principal, pending, resolved []string) (string, error) {
	claimsBuilder := NewJWTClaimsBuilder().
		WithID(id.GenerateUUID()).
		WithSubject(principal.ID).
		WithRoles(principal.Roles).
		WithDetails(principal.Details).
		WithType(TokenTypeChallenge).
		WithClaim(ClaimChallengePrincipalType, principal.Type).
		WithClaim(ClaimChallengePrincipalName, principal.Name).
		WithClaim(ClaimChallengePending, pending).
		WithClaim(ClaimChallengeResolved, resolved)

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

	principalName := cast.ToString(claimsAccessor.Claim(ClaimChallengePrincipalName))
	principalType := PrincipalType(cast.ToString(claimsAccessor.Claim(ClaimChallengePrincipalType)))

	var principal *Principal
	switch principalType {
	case "", PrincipalTypeUser:
		// Empty type keeps backward compatibility for tokens generated before type claim was added.
		principal = NewUser(principalID, principalName, claimsAccessor.Roles()...)
	case PrincipalTypeExternalApp:
		principal = NewExternalApp(principalID, principalName, claimsAccessor.Roles()...)
	case PrincipalTypeSystem:
		principal = &Principal{
			Type:  PrincipalTypeSystem,
			ID:    principalID,
			Name:  principalName,
			Roles: claimsAccessor.Roles(),
		}

	default:
		return nil, ErrTokenInvalid
	}

	principal.AttemptUnmarshalDetails(claimsAccessor.Details())

	return &ChallengeState{
		Principal: principal,
		Pending:   cast.ToStringSlice(claimsAccessor.Claim(ClaimChallengePending)),
		Resolved:  cast.ToStringSlice(claimsAccessor.Claim(ClaimChallengeResolved)),
	}, nil
}
