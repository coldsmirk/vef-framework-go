package security

import (
	"context"
	"errors"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// AuthTypeSignature is the authentication type for signature-based authentication.
const AuthTypeSignature = "signature"

// SignatureAuthenticator validates HMAC-based signatures for external app authentication.
type SignatureAuthenticator struct {
	loader  security.ExternalAppLoader
	options []security.SignatureOption
}

// NewSignatureAuthenticator creates a new signature authenticator.
func NewSignatureAuthenticator(
	loader security.ExternalAppLoader,
	nonceStore security.NonceStore,
) security.Authenticator {
	var options []security.SignatureOption
	if nonceStore != nil {
		options = append(options, security.WithNonceStore(nonceStore))
	}

	return &SignatureAuthenticator{
		loader:  loader,
		options: options,
	}
}

func (*SignatureAuthenticator) Supports(authType string) bool {
	return authType == AuthTypeSignature
}

func (a *SignatureAuthenticator) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	if a.loader == nil {
		return nil, result.ErrNotImplemented(i18n.T(result.ErrMessageExternalAppLoaderNotImplemented))
	}

	appID := authentication.Principal
	if appID == "" {
		return nil, result.ErrAppIDRequired
	}

	credentials, ok := authentication.Credentials.(*security.SignatureCredentials)
	if !ok || credentials == nil {
		return nil, result.ErrCredentialsInvalid(i18n.T(result.ErrMessageCredentialsFormatInvalid))
	}

	principal, secret, err := a.loader.LoadByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	if principal == nil || secret == "" {
		return nil, result.ErrExternalAppNotFound
	}

	if err := a.validateIPWhitelist(ctx, principal); err != nil {
		return nil, err
	}

	if err := a.verifySignature(ctx, appID, secret, credentials); err != nil {
		return nil, err
	}

	logger.Infof("Signature authentication successful for app %q", principal.ID)

	return principal, nil
}

func (a *SignatureAuthenticator) verifySignature(
	ctx context.Context,
	appID, secret string,
	credentials *security.SignatureCredentials,
) error {
	sig, err := security.NewSignature(secret, a.options...)
	if err != nil {
		return mapSignatureError(err)
	}

	if err := sig.Verify(ctx, appID, credentials.Timestamp, credentials.Nonce, credentials.Signature); err != nil {
		return mapSignatureError(err)
	}

	return nil
}

func (*SignatureAuthenticator) validateIPWhitelist(ctx context.Context, principal *security.Principal) error {
	details, ok := principal.Details.(*security.ExternalAppConfig)
	if !ok || details == nil {
		return nil
	}

	if !details.Enabled {
		return result.ErrExternalAppDisabled
	}

	if details.IPWhitelist == "" {
		return nil
	}

	requestIP := contextx.RequestIP(ctx)
	if requestIP == "" {
		return nil
	}

	if validator := security.NewIPWhitelistValidator(details.IPWhitelist); !validator.IsAllowed(requestIP) {
		return result.ErrIPNotAllowed
	}

	return nil
}

// mapSignatureError converts security.Signature errors to result errors.
func mapSignatureError(err error) error {
	switch {
	case errors.Is(err, security.ErrSignatureExpired):
		return result.ErrSignatureExpired
	case errors.Is(err, security.ErrSignatureInvalid):
		return result.ErrSignatureInvalid
	case errors.Is(err, security.ErrSignatureNonceUsed):
		return result.ErrNonceAlreadyUsed
	case errors.Is(err, security.ErrSignatureNonceRequired):
		return result.ErrNonceRequired
	default:
		return result.ErrSignatureInvalid
	}
}
