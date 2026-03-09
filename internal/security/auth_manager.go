package security

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type AuthenticatorAuthManager struct {
	authenticators []security.Authenticator
}

func NewAuthManager(authenticators []security.Authenticator) security.AuthManager {
	return &AuthenticatorAuthManager{
		authenticators: authenticators,
	}
}

func (am *AuthenticatorAuthManager) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	authenticator := am.findAuthenticator(authentication.Type)
	if authenticator == nil {
		logger.Warnf("No authenticator found for authentication type: %s", authentication.Type)

		return nil, result.Err(
			i18n.T(result.ErrMessageUnsupportedAuthenticationType, map[string]any{"kind": authentication.Type}),
			result.WithCode(result.ErrCodeUnsupportedAuthenticationType),
			result.WithStatus(fiber.StatusBadRequest),
		)
	}

	principal, err := authenticator.Authenticate(ctx, authentication)
	if err != nil {
		if _, ok := result.AsErr(err); !ok {
			maskedPrincipal := maskPrincipal(authentication.Principal)
			logger.Warnf("Authentication failed: type=%s, principal=%s, authenticator=%T, error=%v",
				authentication.Type, maskedPrincipal, authenticator, err)
		}

		return nil, err
	}

	return principal, nil
}

func (am *AuthenticatorAuthManager) findAuthenticator(authType string) security.Authenticator {
	for _, authenticator := range am.authenticators {
		if authenticator.Supports(authType) {
			return authenticator
		}
	}

	return nil
}

// maskPrincipal prevents credential leakage in logs by showing only the first 3 chars.
func maskPrincipal(principal string) string {
	if principal == "" {
		return "<empty>"
	}

	if length := len(principal); length <= 3 {
		return "***"
	}

	return principal[:3] + "***"
}
