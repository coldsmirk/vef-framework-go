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
	store  security.SessionStore
	policy security.SessionPolicy
}

func NewOpaqueTokenGenerator(store security.SessionStore, policy security.SessionPolicy) *OpaqueTokenGenerator {
	return &OpaqueTokenGenerator{store: store, policy: policy}
}

func (g *OpaqueTokenGenerator) Generate(ctx context.Context, principal *security.Principal, meta security.SessionMeta) (*security.AuthTokens, error) {
	if err := g.enforceConcurrency(ctx, principal.ID); err != nil {
		return nil, err
	}

	token, err := security.GenerateOpaqueToken()
	if err != nil {
		return nil, err
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
		ExpiresAt:  now.Add(g.policy.IdleTTL),
	}

	if err := g.store.Create(ctx, security.HashOpaqueToken(token), session, g.policy.IdleTTL); err != nil {
		return nil, err
	}

	return &security.AuthTokens{AccessToken: token}, nil
}

// enforceConcurrency applies the per-account session limit before a new session
// is opened: reject the login, or evict the oldest sessions to make room.
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

	for _, stale := range sessions[:len(sessions)-g.policy.MaxConcurrent+1] {
		if err := g.store.Revoke(ctx, stale.ID); err != nil {
			return err
		}
	}

	return nil
}
