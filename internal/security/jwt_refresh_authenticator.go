package security

import (
	"context"
	"strings"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

const (
	AuthTypeRefresh = "refresh"
)

type JWTRefreshAuthenticator struct {
	jwt        *security.JWT
	userLoader security.UserLoader
}

func NewJWTRefreshAuthenticator(jwt *security.JWT, userLoader security.UserLoader) security.Authenticator {
	return &JWTRefreshAuthenticator{
		jwt:        jwt,
		userLoader: userLoader,
	}
}

func (*JWTRefreshAuthenticator) Supports(authType string) bool {
	return authType == AuthTypeRefresh
}

func (j *JWTRefreshAuthenticator) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	if j.userLoader == nil {
		return nil, result.ErrNotImplemented(i18n.T(result.ErrMessageUserLoaderNotImplemented))
	}

	token := authentication.Principal
	if token == "" {
		return nil, result.ErrTokenInvalid
	}

	claimsAccessor, err := j.jwt.Parse(token)
	if err != nil {
		logger.Warnf("JWT refresh token validation failed: %v", err)

		return nil, err
	}

	if claimsAccessor.Type() != security.TokenTypeRefresh {
		return nil, result.ErrTokenInvalid
	}

	subjectParts := strings.SplitN(claimsAccessor.Subject(), "@", 2)
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

	return principal, nil
}
