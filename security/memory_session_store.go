package security

import (
	"context"
	"slices"
	"sync"
	"time"
)

// MemorySessionStore implements SessionStore with in-process maps. It suits
// single-instance deployments; multi-node deployments must use RedisSessionStore
// so sessions are shared across nodes. Expired sessions are reclaimed lazily on
// access.
type MemorySessionStore struct {
	mu      sync.Mutex
	byID    map[string]*sessionRecord
	byToken map[string]string              // tokenHash -> id
	byUser  map[string]map[string]struct{} // userID -> set of ids
}

type sessionRecord struct {
	session   Session
	tokenHash string
}

// NewMemorySessionStore creates an empty in-memory session store.
func NewMemorySessionStore() SessionStore {
	return &MemorySessionStore{
		byID:    make(map[string]*sessionRecord),
		byToken: make(map[string]string),
		byUser:  make(map[string]map[string]struct{}),
	}
}

func (s *MemorySessionStore) Create(_ context.Context, tokenHash string, session Session, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID[session.ID] = &sessionRecord{session: session, tokenHash: tokenHash}
	s.byToken[tokenHash] = session.ID

	ids := s.byUser[session.UserID]
	if ids == nil {
		ids = make(map[string]struct{})
		s.byUser[session.UserID] = ids
	}

	ids[session.ID] = struct{}{}

	return nil
}

func (s *MemorySessionStore) Lookup(_ context.Context, tokenHash string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.byToken[tokenHash]
	if !ok {
		return nil, nil
	}

	record := s.byID[id]
	if record == nil {
		return nil, nil
	}

	if !record.session.ExpiresAt.After(time.Now()) {
		s.remove(record)

		return nil, nil
	}

	session := record.session

	return &session, nil
}

func (s *MemorySessionStore) Renew(_ context.Context, tokenHash string, expiresAt time.Time, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.byToken[tokenHash]
	if !ok {
		return nil
	}

	if record := s.byID[id]; record != nil {
		record.session.ExpiresAt = expiresAt
		record.session.LastSeenAt = time.Now()
	}

	return nil
}

func (s *MemorySessionStore) Revoke(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record := s.byID[id]; record != nil {
		s.remove(record)
	}

	return nil
}

func (s *MemorySessionStore) ListByUser(_ context.Context, userID string) ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	var sessions []Session
	for id := range s.byUser[userID] {
		record := s.byID[id]
		if record == nil {
			continue
		}

		if !record.session.ExpiresAt.After(now) {
			s.remove(record)

			continue
		}

		sessions = append(sessions, record.session)
	}

	slices.SortFunc(sessions, func(a, b Session) int {
		return b.LastSeenAt.Compare(a.LastSeenAt)
	})

	return sessions, nil
}

func (s *MemorySessionStore) RevokeUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id := range s.byUser[userID] {
		if record := s.byID[id]; record != nil {
			delete(s.byID, id)
			delete(s.byToken, record.tokenHash)
		}
	}

	delete(s.byUser, userID)

	return nil
}

// remove deletes a record from all three indexes. The caller holds the lock.
func (s *MemorySessionStore) remove(record *sessionRecord) {
	delete(s.byID, record.session.ID)
	delete(s.byToken, record.tokenHash)

	if ids := s.byUser[record.session.UserID]; ids != nil {
		delete(ids, record.session.ID)

		if len(ids) == 0 {
			delete(s.byUser, record.session.UserID)
		}
	}
}
