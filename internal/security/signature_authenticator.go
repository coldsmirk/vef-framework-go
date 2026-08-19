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
	loader   security.ExternalAppLoader
	verifier *security.Signature
}

// NewSignatureAuthenticator creates a new signature authenticator.
//
// A single long-lived Signature verifier is built here and reused across every
// request. This is what makes server-side replay protection actually work: the
// nonce store is shared process-wide rather than recreated per request (a fresh
// per-request in-memory store would always see every nonce as absent, turning
// replay detection into a no-op and leaking a GC goroutine on each call). The
// per-request app secret is supplied to VerifyWithSecret at verification time,
// so the verifier's own bound secret is never used.
func NewSignatureAuthenticator(
	loader security.ExternalAppLoader,
	nonceStore security.NonceStore,
) security.Authenticator {
	// Only override the nonce store when one is injected. A nil store is left
	// to NewSignature's secure-by-default in-memory store rather than passed
	// through WithNonceStore(nil), which would disable replay protection.
	var options []security.SignatureOption
	if nonceStore != nil {
		options = append(options, security.WithNonceStore(nonceStore))
	}

	// The verifier always authenticates with the caller-supplied per-request
	// secret via VerifyWithSecret, so signatureVerifierPlaceholderSecret is a
	// compile-time-constant placeholder that never computes or checks an HMAC.
	// NewSignature only fails on an invalid secret, so a constant valid hex
	// secret makes the error path unreachable.
	verifier, err := security.NewSignature(signatureVerifierPlaceholderSecret, options...)
	if err != nil {
		panic(err)
	}

	return &SignatureAuthenticator{
		loader:   loader,
		verifier: verifier,
	}
}

// signatureVerifierPlaceholderSecret is a syntactically valid hex secret used
// only to satisfy NewSignature's construction-time validation. It never
// participates in signature verification — VerifyWithSecret always overrides it
// with the per-request app secret.
const signatureVerifierPlaceholderSecret = "00"

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

	if err := validateExternalAppPolicy(ctx, principal); err != nil {
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
	// The auth middleware records the request method/path on ctx; the
	// signature binds them so a captured signature cannot be replayed against
	// a different endpoint. The per-request app secret is supplied here so the
	// shared verifier's placeholder secret is never used.
	method := contextx.RequestMethod(ctx)
	path := contextx.RequestPath(ctx)

	request := security.SignatureRequest{AppID: appID, Method: method, Path: path}
	if err := a.verifier.VerifyWithSecret(ctx, secret, request, *credentials); err != nil {
		logger.Warnf("Signature verify failed for app %q: %v", appID, err)

		return mapSignatureError(err)
	}

	return nil
}

// mapSignatureError converts errors raised during signature verification into
// the corresponding API-facing error.
//
// Server-side configuration errors (signature secret decode failure or missing
// secret) are explicitly mapped to the generic invalid-signature reply so they
// never leak to the client. The originating cause is always logged at the call
// site in verifySignature for ops diagnosis.
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
