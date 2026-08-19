package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/security"
)

// AuthTypeTrustCode is the login mechanism that redeems a trust-login code.
const AuthTypeTrustCode = "trust_code"

// TrustCodeAuthenticator redeems the one-time code the trust-login gateway
// handed to the browser, completing the second leg of the handoff.
//
// It is an ordinary Authenticator on purpose: redeeming the code is the only
// thing specific to trust login, and routing it through security/auth.login
// means the handoff inherits the whole pipeline — brute-force guard, the full
// challenge chain (a forced password change or department selection still runs;
// the external system authenticated the user, it did not satisfy the
// application's own login policy), token issuance under the configured
// mechanism, session concurrency, and the login audit event.
//
// The authentication identifier is the app ID that initiated the handoff,
// checked here against the one the gateway recorded — so what reaches the
// lockout counter and the audit trail is a value the framework verified, not
// one the client asserted.
type TrustCodeAuthenticator struct {
	store security.TrustCodeStore
	cfg   config.TrustLoginConfig
}

// NewTrustCodeAuthenticator creates the trust-login code authenticator.
func NewTrustCodeAuthenticator(store security.TrustCodeStore, cfg config.TrustLoginConfig) *TrustCodeAuthenticator {
	return &TrustCodeAuthenticator{store: store, cfg: cfg}
}

func (*TrustCodeAuthenticator) Supports(authType string) bool { return authType == AuthTypeTrustCode }

func (a *TrustCodeAuthenticator) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	code, ok := authentication.Credentials.(string)
	if !ok || code == "" {
		return nil, security.ErrTrustCodeInvalid
	}

	state, err := a.store.Consume(ctx, code)
	if err != nil {
		return nil, err
	}

	// A plain comparison on purpose: the app ID is not a secret — it rides the
	// same redirect URL as the code — so a constant-time compare here would
	// only imply a timing channel that does not exist. Every mismatch below
	// returns the same verdict as an unknown code; the code is spent either
	// way, so the distinction tells a caller nothing it can use.
	if state.AppID != authentication.Principal {
		logger.Warnf("Trust code rejected: issued for app %q, presented as %q", state.AppID, authentication.Principal)

		return nil, security.ErrTrustCodeInvalid
	}

	if err := a.checkBrowserBinding(ctx, state); err != nil {
		return nil, err
	}

	return state.Principal, nil
}

// checkBrowserBinding rejects a code redeemed by a browser other than the one
// the gateway redirected. It is what keeps a code that leaked out of a URL —
// browser history, a Referer header, a proxy log — from being redeemed
// somewhere else inside its short lifetime.
func (a *TrustCodeAuthenticator) checkBrowserBinding(ctx context.Context, state *security.TrustCodeState) error {
	if a.cfg.IsUserAgentBound() && contextx.RequestUserAgent(ctx) != state.UserAgent {
		logger.Warnf("Trust code rejected for app %q: User-Agent differs from the one it was issued to", state.AppID)

		return security.ErrTrustCodeInvalid
	}

	if a.cfg.BindClientIP && contextx.RequestIP(ctx) != state.ClientIP {
		logger.Warnf("Trust code rejected for app %q: client IP differs from the one it was issued to", state.AppID)

		return security.ErrTrustCodeInvalid
	}

	return nil
}
