package security

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/security"
)

const (
	AuthTypeOpaqueToken = "opaque_token"
)

// OpaqueTokenAuthenticator validates a stateful opaque token by resolving it to
// a server-side session, returning the session's principal snapshot. When
// sliding renewal is enabled it extends the session's idle timeout on each
// request, so an active session never expires mid-use.
type OpaqueTokenAuthenticator struct {
	store  security.SessionStore
	policy security.SessionPolicy
}

func NewOpaqueTokenAuthenticator(store security.SessionStore, policy security.SessionPolicy) security.Authenticator {
	return &OpaqueTokenAuthenticator{store: store, policy: policy}
}

func (*OpaqueTokenAuthenticator) Supports(authType string) bool {
	return authType == AuthTypeOpaqueToken
}

func (a *OpaqueTokenAuthenticator) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	token := authentication.Principal
	if token == "" {
		return nil, security.ErrTokenInvalid
	}

	tokenHash := security.HashOpaqueToken(token)

	session, err := a.store.Lookup(ctx, tokenHash)
	if err != nil {
		logger.Warnf("Opaque session lookup failed: %v", err)

		return nil, security.ErrTokenInvalid
	}

	if session == nil {
		return nil, security.ErrTokenInvalid
	}

	if a.policy.Sliding {
		a.renew(ctx, tokenHash, session)
	}

	return session.Principal, nil
}

// renew slides the session's idle timeout forward, capped at its absolute
// max-lifetime. It is best-effort: a store error is logged, not surfaced, so a
// transient store hiccup never rejects an otherwise valid request.
func (a *OpaqueTokenAuthenticator) renew(ctx context.Context, tokenHash string, session *security.Session) {
	expiresAt := time.Now().Add(a.policy.IdleTTL)

	if a.policy.MaxLifetime > 0 {
		maxExpiry := session.CreatedAt.Add(a.policy.MaxLifetime)
		if expiresAt.After(maxExpiry) {
			expiresAt = maxExpiry
		}
	}

	if err := a.store.Renew(ctx, tokenHash, expiresAt, a.policy.IdleTTL); err != nil {
		logger.Warnf("Opaque session renewal failed: %v", err)
	}
}
