package security

import (
	"context"
	"slices"
	"time"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/security"
)

// OpaqueTokenGenerator issues stateful opaque tokens: it opens a server-side
// session (enforcing the per-account concurrency policy) and returns a random
// token whose hash keys that session. It carries no refresh token, since the
// session renews itself on use.
type OpaqueTokenGenerator struct {
	store    security.SessionStore
	policy   security.SessionPolicy
	notifier *security.SessionRevocationNotifier
}

func NewOpaqueTokenGenerator(store security.SessionStore, policy security.SessionPolicy, notifier *security.SessionRevocationNotifier) *OpaqueTokenGenerator {
	return &OpaqueTokenGenerator{store: store, policy: policy, notifier: notifier}
}

func (g *OpaqueTokenGenerator) Generate(ctx context.Context, principal *security.Principal, meta security.SessionMeta) (*security.AuthTokens, error) {
	// Refuse before any store write: a session must never open for a rejected principal.
	if principal == nil || principal.IsReserved() {
		return nil, security.ErrReservedPrincipal
	}

	if err := g.enforceConcurrency(ctx, principal.ID); err != nil {
		return nil, err
	}

	token, err := security.GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}

	// The initial lifetime is the idle window, capped by the absolute
	// max-lifetime so a session never outlives the cap even when idle_ttl is
	// configured larger than max_lifetime (mirrors the renewal-time clamp).
	lifetime := g.policy.IdleTTL
	if g.policy.MaxLifetime > 0 {
		lifetime = min(lifetime, g.policy.MaxLifetime)
	}

	now := time.Now()
	session := security.Session{
		ID:         id.Generate(),
		UserID:     principal.ID,
		Principal:  principal,
		ClientIP:   meta.ClientIP,
		UserAgent:  meta.UserAgent,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(lifetime),
	}

	if err := g.store.Create(ctx, security.HashOpaqueToken(token), session, lifetime); err != nil {
		return nil, err
	}

	return &security.AuthTokens{AccessToken: token}, nil
}

// enforceConcurrency applies the per-account session limit before a new session
// is opened: reject the login, or evict the oldest sessions to make room.
//
// Enforcement is best-effort: the count-then-create sequence is not atomic
// across store calls, so simultaneous logins for one account may briefly
// overshoot MaxConcurrent by the number of racing requests (evict-oldest
// self-heals on the next login). It is a policy/blast-radius limit, not a hard
// security boundary.
func (g *OpaqueTokenGenerator) enforceConcurrency(ctx context.Context, userID string) error {
	if g.policy.MaxConcurrent <= 0 {
		return nil
	}

	sessions, err := g.store.ListByUser(ctx, userID)
	if err != nil {
		return err
	}

	if len(sessions) < g.policy.MaxConcurrent {
		return nil
	}

	if g.policy.OnExceed == security.SessionExceedReject {
		return security.ErrTooManyConcurrentSessions
	}

	// Evict the oldest sessions so that, once the new one is added, the count is
	// exactly MaxConcurrent.
	slices.SortFunc(sessions, func(a, b security.Session) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	evicted := make([]security.SessionRevocation, 0, len(sessions)-g.policy.MaxConcurrent+1)

	for _, stale := range sessions[:len(sessions)-g.policy.MaxConcurrent+1] {
		if err := g.store.Revoke(ctx, stale.ID); err != nil {
			return err
		}

		evicted = append(evicted, security.SessionRevocation{SessionID: stale.ID, UserID: stale.UserID})
	}

	g.notifier.NotifyRevoked(ctx, evicted...)

	return nil
}
