package security

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/result"
)

const (
	ChallengeTokenExpires       = 5 * time.Minute
	ClaimChallengePending       = "pnd"
	ClaimChallengePrincipalType = "ptp"
	ClaimChallengeResolved      = "rsd"
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

func (s *JWTChallengeTokenStore) Generate(principal *Principal, pending, resolved []string) (string, error) {
	claimsBuilder := NewJWTClaimsBuilder().
		WithID(id.GenerateUUID()).
		WithSubject(fmt.Sprintf("%s@%s", principal.ID, principal.Name)).
		WithRoles(principal.Roles).
		WithDetails(principal.Details).
		WithType(TokenTypeChallenge).
		WithClaim(ClaimChallengePrincipalType, principal.Type).
		WithClaim(ClaimChallengePending, pending).
		WithClaim(ClaimChallengeResolved, resolved)

	return s.jwt.Generate(claimsBuilder, ChallengeTokenExpires, 0)
}

func (s *JWTChallengeTokenStore) Parse(token string) (*ChallengeState, error) {
	claimsAccessor, err := s.jwt.Parse(token)
	if err != nil {
		return nil, err
	}

	if claimsAccessor.Type() != TokenTypeChallenge {
		return nil, result.ErrTokenInvalid
	}

	subjectParts := strings.SplitN(claimsAccessor.Subject(), "@", 2)
	if len(subjectParts) < 2 {
		return nil, result.ErrTokenInvalid
	}

	principalType := PrincipalType(cast.ToString(claimsAccessor.Claim(ClaimChallengePrincipalType)))

	var principal *Principal
	switch principalType {
	case "", PrincipalTypeUser:
		// Empty type keeps backward compatibility for tokens generated before type claim was added.
		principal = NewUser(subjectParts[0], subjectParts[1], claimsAccessor.Roles()...)
	case PrincipalTypeExternalApp:
		principal = NewExternalApp(subjectParts[0], subjectParts[1], claimsAccessor.Roles()...)
	case PrincipalTypeSystem:
		principal = &Principal{
			Type:  PrincipalTypeSystem,
			ID:    subjectParts[0],
			Name:  subjectParts[1],
			Roles: claimsAccessor.Roles(),
		}

	default:
		return nil, result.ErrTokenInvalid
	}

	principal.AttemptUnmarshalDetails(claimsAccessor.Details())

	return &ChallengeState{
		Principal: principal,
		Pending:   cast.ToStringSlice(claimsAccessor.Claim(ClaimChallengePending)),
		Resolved:  cast.ToStringSlice(claimsAccessor.Claim(ClaimChallengeResolved)),
	}, nil
}
