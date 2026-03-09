package security

import (
	"context"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/cache"
)

// MemoryNonceStore implements NonceStore using an in-memory cache.
// This implementation is suitable for development and single-instance deployments.
// For distributed systems, use RedisNonceStore instead.
type MemoryNonceStore struct {
	cache cache.Cache[bool]
	mu    sync.Mutex
}

// NewMemoryNonceStore creates a new in-memory nonce store.
func NewMemoryNonceStore() NonceStore {
	return &MemoryNonceStore{
		cache: cache.NewMemory[bool](),
	}
}

// buildKey creates a unique cache key for the app-nonce combination.
func (*MemoryNonceStore) buildKey(appID, nonce string) string {
	return appID + ":" + nonce
}

// StoreIfAbsent atomically stores the nonce only when it does not exist.
func (m *MemoryNonceStore) StoreIfAbsent(ctx context.Context, appID, nonce string, ttl time.Duration) (bool, error) {
	key := m.buildKey(appID, nonce)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cache.Contains(ctx, key) {
		return false, nil
	}

	if err := m.cache.Set(ctx, key, true, ttl); err != nil {
		return false, err
	}

	return true, nil
}
