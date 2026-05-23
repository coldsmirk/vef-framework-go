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
		return nil, result.ErrNotImplemented(i18n.T(security.ErrMessageExternalAppLoaderNotImplemented))
	}

	appID := authentication.Principal
	if appID == "" {
		return nil, security.ErrAppIDRequired
	}

	credentials, ok := authentication.Credentials.(*security.SignatureCredentials)
	if !ok || credentials == nil {
		return nil, security.ErrCredentialsInvalid(i18n.T(security.ErrMessageCredentialsFormatInvalid))
	}

	principal, secret, err := a.loader.LoadByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	if principal == nil || secret == "" {
		return nil, security.ErrExternalAppNotFound
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
		logger.Warnf("Signature construction failed for app %q (likely misconfigured secret): %v", appID, err)

		return mapSignatureError(err)
	}

	if err := sig.Verify(ctx, appID, credentials.Timestamp, credentials.Nonce, credentials.Signature); err != nil {
		logger.Warnf("Signature verify failed for app %q: %v", appID, err)

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
		return security.ErrExternalAppDisabled
	}

	if details.IPWhitelist == "" {
		return nil
	}

	requestIP := contextx.RequestIP(ctx)
	if requestIP == "" {
		return nil
	}

	if validator := security.NewIPWhitelistValidator(details.IPWhitelist); !validator.IsAllowed(requestIP) {
		return security.ErrIPNotAllowed
	}

	return nil
}

// mapSignatureError converts errors raised during signature construction
// or verification into the corresponding API-facing error.
//
// Server-side configuration errors (signature secret decode failure or
// missing secret) are explicitly mapped to the generic invalid-signature
// reply so they never leak to the client. The originating cause is
// always logged at the call site in verifySignature for ops diagnosis.
func mapSignatureError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, security.ErrSignatureSecretRequired),
		errors.Is(err, security.ErrDecodeSignatureSecretFailed):
		return security.ErrSignatureInvalid
	}

	if apiErr, ok := result.AsErr(err); ok {
		return apiErr
	}

	return security.ErrSignatureInvalid
}
