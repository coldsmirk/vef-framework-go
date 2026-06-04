package security

import (
	"context"
	"sync"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

const (
	AuthTypePassword = "password"
)

// dummyComparePlaintext is hashed (lazily, once) with the injected encoder to
// produce the comparison hash used on the user-not-found path. Deriving it from
// the real encoder keeps the timing-equalization comparison on the same
// algorithm and cost as a genuine verification, which a fixed low-cost hash does
// not — closing the username-enumeration timing side-channel.
const dummyComparePlaintext = "__vef_dummy_password__"

// PasswordAuthenticator verifies username/password credentials with optional decryption support
// for scenarios where clients encrypt passwords before transmission.
type PasswordAuthenticator struct {
	loader    security.UserLoader
	encoder   password.Encoder
	dummyOnce sync.Once
	dummyHash string
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
		p.equalizeTiming(plaintext)

		return nil, security.ErrCredentialsInvalid(i18n.T("security_invalid_credentials"))
	}

	if principal == nil || passwordHash == "" {
		// Equalize timing with the found-user path by running a comparison.
		p.equalizeTiming(plaintext)

		return nil, security.ErrCredentialsInvalid(i18n.T("security_invalid_credentials"))
	}

	if !p.encoder.Matches(plaintext, passwordHash) {
		return nil, security.ErrCredentialsInvalid(i18n.T("security_invalid_credentials"))
	}

	logger.Infof("Password authentication successful for principal %q", principal.ID)

	return principal, nil
}

// equalizeTiming runs a dummy password comparison so the user-not-found,
// nil-principal and empty-hash paths take the same time as a real verification,
// preventing username enumeration via a timing side-channel (CWE-208). The dummy
// hash is derived once from the configured encoder, so the comparison uses the
// same algorithm and cost as a genuine check.
func (p *PasswordAuthenticator) equalizeTiming(plaintext string) {
	p.dummyOnce.Do(func() {
		if p.encoder != nil {
			if hash, err := p.encoder.Encode(dummyComparePlaintext); err == nil {
				p.dummyHash = hash
			}
		}
	})

	if p.dummyHash != "" {
		p.encoder.Matches(plaintext, p.dummyHash)

		return
	}

	// Deriving the dummy hash failed (e.g. a misconfigured cost or a custom
	// encoder whose Encode errored). Matches against an empty hash returns
	// immediately, which would reopen the enumeration timing channel, so run the
	// encoder's KDF directly to keep the not-found path's cost comparable.
	_, _ = p.encoder.Encode(plaintext)
}
