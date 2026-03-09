package auth

import (
	"context"

	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/security"
)

// AccessTokenAuthenticator delegates token authentication to the security.AuthManager.
type AccessTokenAuthenticator struct {
	manager security.AuthManager
}

// NewAccessTokenAuthenticator creates a new access token authenticator.
func NewAccessTokenAuthenticator(manager security.AuthManager) TokenAuthenticator {
	return &AccessTokenAuthenticator{manager: manager}
}

func (a *AccessTokenAuthenticator) Authenticate(ctx context.Context, token string) (*security.Principal, error) {
	return a.manager.Authenticate(ctx, security.Authentication{
		Type:      isecurity.AuthTypeToken,
		Principal: token,
	})
}
