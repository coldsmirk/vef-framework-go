package security

import (
	"context"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/cache"
)

// MemoryTrustCodeStore implements TrustCodeStore using an in-memory TTL cache,
// so expired codes are collected instead of accumulating.
//
// It is single-process by construction: a code issued on one replica cannot be
// redeemed on another, which fails the login outright rather than merely
// weakening it. Multi-replica deployments must run NewRedisTrustCodeStore —
// the security module selects it automatically when a Redis client is
// available.
type MemoryTrustCodeStore struct {
	cache cache.Cache[TrustCodeState]
	mu    sync.Mutex
}

// NewMemoryTrustCodeStore creates a new in-memory trust code store.
func NewMemoryTrustCodeStore() TrustCodeStore {
	return &MemoryTrustCodeStore{cache: cache.NewMemory[TrustCodeState]()}
}

func (s *MemoryTrustCodeStore) Issue(ctx context.Context, state TrustCodeState, ttl time.Duration) (string, error) {
	code, err := GenerateOpaqueToken()
	if err != nil {
		return "", err
	}

	if err := s.cache.Set(ctx, HashOpaqueToken(code), state, ttl); err != nil {
		return "", err
	}

	return code, nil
}

// Consume redeems the code under the mutex: the cache offers no atomic
// read-and-delete, and two requests racing on one code must never both succeed.
func (s *MemoryTrustCodeStore) Consume(ctx context.Context, code string) (*TrustCodeState, error) {
	if code == "" {
		return nil, ErrTrustCodeInvalid
	}

	key := HashOpaqueToken(code)

	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.cache.Get(ctx, key)
	if !ok {
		return nil, ErrTrustCodeInvalid
	}

	if err := s.cache.Delete(ctx, key); err != nil {
		return nil, err
	}

	return &state, nil
}
