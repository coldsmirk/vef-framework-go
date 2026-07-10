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

		return nil, errUnsupportedAuthenticationType(authentication.Type)
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

// errUnsupportedAuthenticationType reports an authentication type no registered
// authenticator accepts; Login also raises it to refuse framework-issued token
// types presented as login credentials.
func errUnsupportedAuthenticationType(kind string) error {
	return result.Err(
		i18n.T(security.ErrMessageUnsupportedAuthenticationType, map[string]any{"kind": kind}),
		result.WithCode(security.ErrCodeUnsupportedAuthenticationType),
		result.WithStatus(fiber.StatusBadRequest),
	)
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
