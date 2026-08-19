package security

import (
	"context"
	"time"
)

// TrustCodeState is the identity a verified trust-login handoff parks for its
// one-time code to redeem.
//
// The gateway resolves the external user before it issues the code, so a code
// that exists always names a real principal: an unresolvable user fails at the
// redirect, where the browser can still be told what went wrong, instead of
// after it as an opaque login error on a blank page.
type TrustCodeState struct {
	// Principal is the local identity the code logs in.
	Principal *Principal
	// AppID names the external system that initiated the handoff. The exchange
	// re-checks it against the login identifier the client supplies, so the
	// value that reaches the audit trail is one the gateway verified rather
	// than one the client asserted.
	AppID string
	// UserAgent and ClientIP describe the browser the gateway redirected. They
	// bind the code to that browser as far as bind_user_agent / bind_client_ip
	// ask for; both are recorded unconditionally so turning a binding on takes
	// effect on the next handoff rather than requiring a reissue.
	UserAgent string
	ClientIP  string
}

// TrustCodeStore parks the one-time authorization codes issued by the
// trust-login gateway and redeems them at the login exchange.
//
// The code travels through a browser redirect, so it is a bearer credential
// with all that implies: implementations generate it with GenerateOpaqueToken,
// key it by HashOpaqueToken so a store leak yields no live codes, and hold it
// only for the configured TTL.
type TrustCodeStore interface {
	// Issue parks state under a freshly generated code and returns that code.
	Issue(ctx context.Context, state TrustCodeState, ttl time.Duration) (string, error)
	// Consume redeems a code exactly once and returns the state it parked. A
	// code that is unknown, already redeemed, or expired yields
	// ErrTrustCodeInvalid — the three are deliberately indistinguishable to the
	// caller. Reading and invalidating must be atomic: single use is the whole
	// security value of the code, and a redirect URL sitting in browser history
	// must not be replayable.
	Consume(ctx context.Context, code string) (*TrustCodeState, error)
}

// TrustUserResolver maps an external system's user identifier onto a local
// Principal for trust login.
//
// Registering one is optional. With no resolver the gateway resolves through
// UserLoader.LoadByID, which is already correct whenever both systems key users
// by the same identifier (a shared HR master, for instance). Implement it when
// the identifiers differ, or when the mapping depends on which external system
// initiated the handoff.
type TrustUserResolver interface {
	// ResolveUser returns the local principal that externalUserID denotes for
	// appID, or a nil principal when no local user corresponds to it. Returning
	// an error reports a resolution failure (an unreachable directory, say);
	// "this user does not exist here" is a nil principal, not an error.
	ResolveUser(ctx context.Context, appID, externalUserID string) (*Principal, error)
}
