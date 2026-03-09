package security

import (
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/security"
)

const (
	AccessTokenExpires = time.Minute * 30
)

type JWTTokenGenerator struct {
	jwt              *security.JWT
	tokenExpires     time.Duration
	refreshNotBefore time.Duration
}

func NewJWTTokenGenerator(jwt *security.JWT, securityConfig *config.SecurityConfig) security.TokenGenerator {
	return &JWTTokenGenerator{
		jwt:              jwt,
		tokenExpires:     securityConfig.TokenExpires,
		refreshNotBefore: securityConfig.RefreshNotBefore,
	}
}

func (g *JWTTokenGenerator) Generate(principal *security.Principal) (*security.AuthTokens, error) {
	jwtID := id.GenerateUUID()

	accessToken, err := g.generateAccessToken(jwtID, principal)
	if err != nil {
		logger.Errorf("Failed to generate access token for principal %q: %v", principal.ID, err)

		return nil, err
	}

	refreshToken, err := g.generateRefreshToken(jwtID, principal)
	if err != nil {
		logger.Errorf("Failed to generate refresh token for principal %q: %v", principal.ID, err)

		return nil, err
	}

	return &security.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateAccessToken encodes id@name in subject to avoid DB lookups during authentication.
func (g *JWTTokenGenerator) generateAccessToken(jwtID string, principal *security.Principal) (string, error) {
	claimsBuilder := security.NewJWTClaimsBuilder().
		WithID(jwtID).
		WithSubject(fmt.Sprintf("%s@%s", principal.ID, principal.Name)).
		WithRoles(principal.Roles).
		WithDetails(principal.Details).
		WithType(security.TokenTypeAccess)

	return g.jwt.Generate(claimsBuilder, AccessTokenExpires, 0)
}

func (g *JWTTokenGenerator) generateRefreshToken(jwtID string, principal *security.Principal) (string, error) {
	claimsBuilder := security.NewJWTClaimsBuilder().
		WithID(jwtID).
		WithSubject(fmt.Sprintf("%s@%s", principal.ID, principal.Name)).
		WithType(security.TokenTypeRefresh)

	return g.jwt.Generate(claimsBuilder, g.tokenExpires, g.refreshNotBefore)
}
