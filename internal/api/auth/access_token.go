package auth

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

// AccessTokenAuthenticator delegates incoming API-token authentication to the
// security.AuthManager, dispatching the deployment's configured token mechanism
// (jwt_token or opaque_token) so the matching authenticator handles it.
type AccessTokenAuthenticator struct {
	manager  security.AuthManager
	authType string
}

// NewAccessTokenAuthenticator creates a new access token authenticator.
func NewAccessTokenAuthenticator(manager security.AuthManager, cfg *config.SecurityConfig) TokenAuthenticator {
	return &AccessTokenAuthenticator{
		manager:  manager,
		authType: string(cfg.EffectiveTokenType()),
	}
}

func (a *AccessTokenAuthenticator) Authenticate(ctx context.Context, token string) (*security.Principal, error) {
	return a.manager.Authenticate(ctx, security.Authentication{
		Type:      a.authType,
		Principal: token,
	})
}
