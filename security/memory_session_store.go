package security

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/cache"
)

// MemorySessionStore implements SessionStore on the framework's in-memory TTL
// cache, so expired sessions are reclaimed by the cache's lazy checks and
// background GC instead of accumulating for the process lifetime. It suits
// single-instance deployments; multi-node deployments must use
// RedisSessionStore so sessions are shared across nodes.
//
// There is no per-user index: ListByUser and ListAll scan all live sessions,
// which is fine at the single-node scale this store targets (mirroring the
// Redis store's keyspace-scan trade-off for ListAll).
type MemorySessionStore struct {
	// mu serializes mutations so the two caches never diverge (e.g. a renewal
	// resurrecting a concurrently revoked session); reads stay lock-free.
	mu      sync.Mutex
	records cache.Cache[sessionRecord] // session ID -> record
	tokens  cache.Cache[string]        // tokenHash -> session ID
}

type sessionRecord struct {
	session   Session
	tokenHash string
}

// NewMemorySessionStore creates an empty in-memory session store.
func NewMemorySessionStore() SessionStore {
	return &MemorySessionStore{
		records: cache.NewMemory[sessionRecord](),
		tokens:  cache.NewMemory[string](),
	}
}

func (s *MemorySessionStore) Create(ctx context.Context, tokenHash string, session Session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.records.Set(ctx, session.ID, sessionRecord{session: session, tokenHash: tokenHash}, ttl); err != nil {
		return err
	}

	return s.tokens.Set(ctx, tokenHash, session.ID, ttl)
}

func (s *MemorySessionStore) Lookup(ctx context.Context, tokenHash string) (*Session, error) {
	id, ok := s.tokens.Get(ctx, tokenHash)
	if !ok {
		return nil, nil
	}

	record, ok := s.records.Get(ctx, id)
	if !ok || !isLive(record) {
		return nil, nil
	}

	session := cloneSessionPrincipal(record.session)

	return &session, nil
}

func (s *MemorySessionStore) Renew(ctx context.Context, tokenHash string, expiresAt time.Time, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.tokens.Get(ctx, tokenHash)
	if !ok {
		return nil
	}

	record, ok := s.records.Get(ctx, id)
	if !ok {
		return nil
	}

	record.session.ExpiresAt = expiresAt
	record.session.LastSeenAt = time.Now()

	if err := s.records.Set(ctx, id, record, ttl); err != nil {
		return err
	}

	return s.tokens.Set(ctx, tokenHash, id, ttl)
}

func (s *MemorySessionStore) Revoke(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records.Get(ctx, id)
	if !ok {
		return nil
	}

	if err := s.records.Delete(ctx, id); err != nil {
		return err
	}

	return s.tokens.Delete(ctx, record.tokenHash)
}

func (s *MemorySessionStore) ListByUser(ctx context.Context, userID string) ([]Session, error) {
	return s.collect(ctx, func(record sessionRecord) bool {
		return record.session.UserID == userID
	})
}

func (s *MemorySessionStore) RevokeUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var targets []sessionRecord

	err := s.records.ForEach(ctx, func(_ string, record sessionRecord) bool {
		if record.session.UserID == userID {
			targets = append(targets, record)
		}

		return true
	})
	if err != nil {
		return err
	}

	for _, record := range targets {
		if err := s.records.Delete(ctx, record.session.ID); err != nil {
			return err
		}

		if err := s.tokens.Delete(ctx, record.tokenHash); err != nil {
			return err
		}
	}

	return nil
}

// ListAll returns every live session across all users, newest activity first.
func (s *MemorySessionStore) ListAll(ctx context.Context) ([]Session, error) {
	return s.collect(ctx, func(sessionRecord) bool { return true })
}

// collect scans the record cache for live sessions matching the filter,
// returning isolated copies sorted by most recent activity.
func (s *MemorySessionStore) collect(ctx context.Context, match func(sessionRecord) bool) ([]Session, error) {
	var sessions []Session

	err := s.records.ForEach(ctx, func(_ string, record sessionRecord) bool {
		if isLive(record) && match(record) {
			sessions = append(sessions, cloneSessionPrincipal(record.session))
		}

		return true
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(sessions, func(a, b Session) int {
		return b.LastSeenAt.Compare(a.LastSeenAt)
	})

	return sessions, nil
}

// isLive applies the authoritative Session.ExpiresAt check on top of the cache
// TTL: renewals slide the cache TTL by the idle window, but ExpiresAt carries
// the absolute max-lifetime cap, so both must gate a read (mirroring the Redis
// store's load).
func isLive(record sessionRecord) bool {
	return record.session.ExpiresAt.After(time.Now())
}

// cloneSessionPrincipal returns session with an isolated Principal — its Roles
// slice cloned — so callers cannot mutate the principal shared with the stored
// record and with other concurrent requests. This matches the fresh-per-read
// principal the Redis store yields by deserialization. (Details is an opaque
// any and is left aliased, since it cannot be deep-copied generically.)
func cloneSessionPrincipal(session Session) Session {
	if session.Principal != nil {
		principal := *session.Principal
		principal.Roles = slices.Clone(principal.Roles)
		session.Principal = &principal
	}

	return session
}
