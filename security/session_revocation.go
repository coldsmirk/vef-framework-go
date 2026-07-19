package security

import "context"

// SessionRevocation describes one revoked login session.
type SessionRevocation struct {
	// SessionID is the session's public ID.
	SessionID string
	// UserID is the owning principal's id.
	UserID string
}

// SessionRevocationListener observes session revocations — logout, concurrent-
// login eviction, or administrative kicks. The framework fires it from its own
// revocation paths; application code that revokes sessions directly against
// the SessionStore should fire the injected SessionRevocationNotifier itself
// (a missed notification degrades to the next periodic session check, never to
// a stale grant). Register with vef.ProvideSessionRevocationListener.
type SessionRevocationListener interface {
	// OnSessionsRevoked is invoked after the sessions were removed from the
	// store. It runs synchronously on the revoking call path, so
	// implementations must be fast and must not block.
	OnSessionsRevoked(ctx context.Context, revocations []SessionRevocation)
}

// SessionRevocationNotifier fans a revocation out to the registered listeners.
// It is exposed in DI so application-owned revocation paths (session-admin
// endpoints built on SessionStore) can fire the same notifications the
// framework's own paths fire.
type SessionRevocationNotifier struct {
	listeners []SessionRevocationListener
}

// NewSessionRevocationNotifier builds a notifier over the registered
// listeners; nil entries (disabled contributors) are dropped.
func NewSessionRevocationNotifier(listeners []SessionRevocationListener) *SessionRevocationNotifier {
	live := make([]SessionRevocationListener, 0, len(listeners))
	for _, listener := range listeners {
		if listener != nil {
			live = append(live, listener)
		}
	}

	return &SessionRevocationNotifier{listeners: live}
}

// NotifyRevoked delivers the revocations to every listener. It is nil-receiver
// safe and a no-op for an empty revocation set, so call sites need no guards.
func (n *SessionRevocationNotifier) NotifyRevoked(ctx context.Context, revocations ...SessionRevocation) {
	if n == nil || len(revocations) == 0 {
		return
	}

	for _, listener := range n.listeners {
		listener.OnSessionsRevoked(ctx, revocations)
	}
}
