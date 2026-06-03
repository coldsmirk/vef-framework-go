package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

const (
	AuthTypePassword = "password"
)

// dummyPasswordHash is a pre-computed bcrypt hash (cost 4) used to equalize
// response time on the user-not-found path, preventing username enumeration
// via timing side-channel. The hash is bcrypt cost 4 for the sentinel string
// "__vef_dummy__", computed at build time and never used as a real credential.
const dummyPasswordHash = "$2a$04$mk2k3PgSLa1KgLOM/.Qs7OlWyDXJTp/ezuzkG0eDJwqdG0.HflGHG"

// PasswordAuthenticator verifies username/password credentials with optional decryption support
// for scenarios where clients encrypt passwords before transmission.
type PasswordAuthenticator struct {
	loader  security.UserLoader
	encoder password.Encoder
}

func NewPasswordAuthenticator(
	loader security.UserLoader,
	encoder password.Encoder,
) security.Authenticator {
	return &PasswordAuthenticator{
		loader:  loader,
		encoder: encoder,
	}
}

func (*PasswordAuthenticator) Supports(authType string) bool { return authType == AuthTypePassword }

func (p *PasswordAuthenticator) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	if p.loader == nil {
		return nil, result.ErrNotImplemented(i18n.T(security.ErrMessageUserLoaderNotImplemented))
	}

	username := authentication.Principal
	if username == "" {
		return nil, security.ErrPrincipalInvalid(i18n.T("security_username_required"))
	}

	if username == orm.OperatorSystem || username == orm.OperatorCronJob || username == orm.OperatorAnonymous {
		return nil, security.ErrPrincipalInvalid(i18n.T("security_system_principal_login_forbidden"))
	}

	plaintext, ok := authentication.Credentials.(string)
	if !ok || plaintext == "" {
		return nil, security.ErrCredentialsInvalid(i18n.T("security_password_required"))
	}

	principal, passwordHash, err := p.loader.LoadByUsername(ctx, username)
	if err != nil {
		if result.IsRecordNotFound(err) {
			logger.Infof("User loader returned record not found for username %s", maskPrincipal(username))
		} else {
			logger.Warnf("Failed to load user by username %s: %v", maskPrincipal(username), err)
		}

		// Perform a dummy comparison to equalize response time regardless of user existence,
		// preventing username enumeration via timing side-channel.
		p.encoder.Matches(plaintext, dummyPasswordHash)

		return nil, security.ErrCredentialsInvalid(i18n.T("security_invalid_credentials"))
	}

	if principal == nil || passwordHash == "" {
		// Equalize timing with the found-user path by running a comparison.
		p.encoder.Matches(plaintext, dummyPasswordHash)

		return nil, security.ErrCredentialsInvalid(i18n.T("security_invalid_credentials"))
	}

	if !p.encoder.Matches(plaintext, passwordHash) {
		return nil, security.ErrCredentialsInvalid(i18n.T("security_invalid_credentials"))
	}

	logger.Infof("Password authentication successful for principal %q", principal.ID)

	return principal, nil
}
