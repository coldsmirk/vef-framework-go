package cache

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
)

type memoryConfig struct {
	maxSize        int64
	defaultTTL     time.Duration
	evictionPolicy EvictionPolicy
	gcInterval     time.Duration
}

func defaultMemoryConfig() *memoryConfig {
	return &memoryConfig{
		maxSize:        0,
		defaultTTL:     0,
		evictionPolicy: EvictionPolicyLRU,
		gcInterval:     5 * time.Minute,
	}
}

// cacheEntry represents a single entry in the memory cache.
type cacheEntry[T any] struct {
	data      T
	expiresAt int64 // Unix nanoseconds, 0 means no expiration
}

// isExpired checks if the cache entry has expired.
func (e *cacheEntry[T]) isExpired() bool {
	if e.expiresAt == 0 {
		return false
	}

	return time.Now().UnixNano() > e.expiresAt
}

// memoryCache implements the Cache interface using xsync.Map for pure in-memory caching.
type memoryCache[T any] struct {
	data            *xsync.Map[string, *cacheEntry[T]]
	maxSize         int64
	defaultTTL      time.Duration
	evictionPolicy  EvictionPolicy
	evictionHandler EvictionHandler
	stopGC          chan struct{}
	gcInterval      time.Duration
	size            atomic.Int64 // Atomic counter for cache size
	mu              sync.Mutex   // Protects eviction logic
	loadMixin       SingleflightMixin[T]
	closed          atomic.Bool // Tracks if cache is closed
}

// newMemoryCache creates a new in-memory cache with specified behavior.
func newMemoryCache[T any](cfg *memoryConfig) Cache[T] {
	if cfg == nil {
		cfg = defaultMemoryConfig()
	}

	factory := &EvictionHandlerFactory{}

	// Use default if gcInterval is not set
	if cfg.gcInterval <= 0 {
		cfg.gcInterval = 5 * time.Minute
	}

	// When maxSize is unlimited, use NoOp handler (no eviction needed)
	// When maxSize is set, ensure we have a valid eviction policy (default to LRU)
	if cfg.maxSize <= 0 {
		cfg.evictionPolicy = EvictionPolicyNone
	} else {
		switch cfg.evictionPolicy {
		case EvictionPolicyLRU, EvictionPolicyLFU, EvictionPolicyFIFO:
			// keep requested policy
		default:
			// Force supported policy when size constrained
			cfg.evictionPolicy = EvictionPolicyLRU
		}
	}

	m := &memoryCache[T]{
		data:            xsync.NewMap[string, *cacheEntry[T]](),
		maxSize:         cfg.maxSize,
		defaultTTL:      cfg.defaultTTL,
		evictionPolicy:  cfg.evictionPolicy,
		evictionHandler: factory.CreateHandler(cfg.evictionPolicy),
		stopGC:          make(chan struct{}),
		gcInterval:      cfg.gcInterval,
	}

	// Start background garbage collection
	go m.runGC()

	return m
}

// checkExpired checks if an entry is expired and removes it if so.
// Returns true if the entry was expired and removed.
func (m *memoryCache[T]) checkExpired(key string, entry *cacheEntry[T]) bool {
	if entry.isExpired() {
		m.data.Delete(key)
		m.evictionHandler.OnEvict(key)
		m.size.Add(-1)

		return true
	}

	return false
}

// Get retrieves a value by key.
func (m *memoryCache[T]) Get(_ context.Context, key string) (value T, _ bool) {
	if m.closed.Load() {
		return value, false
	}

	entry, exists := m.data.Load(key)
	if !exists {
		return value, false
	}

	if m.checkExpired(key, entry) {
		return value, false
	}

	// Track access for eviction policies
	m.evictionHandler.OnAccess(key)

	return entry.data, true
}

// GetOrLoad retrieves a value or loads it when absent using singleflight coordination.
func (m *memoryCache[T]) GetOrLoad(ctx context.Context, key string, loader LoaderFunc[T], ttl ...time.Duration) (T, error) {
	return m.loadMixin.GetOrLoad(ctx, key, loader, ttl, m.Get, m.Set)
}

// Set stores a value with the given key and optional Ttl.
func (m *memoryCache[T]) Set(_ context.Context, key string, value T, ttl ...time.Duration) error {
	if m.closed.Load() {
		return ErrCacheClosed
	}

	// Lock to prevent race conditions during eviction
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if key already exists (update case)
	_, exists := m.data.Load(key)

	// Handle memory limit and eviction only for new entries
	if !exists && m.maxSize > 0 {
		for m.size.Load() >= m.maxSize {
			// Find candidate to evict
			candidate := m.evictionHandler.SelectEvictionCandidate()
			if candidate == "" {
				// This should not happen with proper eviction policies (LRU/LFU/FIFO)
				// but if it does, return error instead of panic
				return ErrMemoryLimitExceeded
			}

			// Evict the candidate
			if _, loaded := m.data.LoadAndDelete(candidate); loaded {
				m.evictionHandler.OnEvict(candidate)
				m.size.Add(-1)
			}
		}
	}

	// Determine Ttl
	var expireTimeNs int64
	if len(ttl) > 0 && ttl[0] > 0 {
		expireTimeNs = time.Now().Add(ttl[0]).UnixNano()
	} else if m.defaultTTL > 0 {
		expireTimeNs = time.Now().Add(m.defaultTTL).UnixNano()
	}

	entry := &cacheEntry[T]{
		data:      value,
		expiresAt: expireTimeNs,
	}

	m.data.Store(key, entry)

	// Update size counter and eviction handler only for new entries
	if !exists {
		m.size.Add(1)
		m.evictionHandler.OnInsert(key)
	} else {
		// For updates, treat as access
		m.evictionHandler.OnAccess(key)
	}

	return nil
}

// Contains checks if a key exists in the cache.
func (m *memoryCache[T]) Contains(_ context.Context, key string) bool {
	if m.closed.Load() {
		return false
	}

	entry, exists := m.data.Load(key)
	if !exists {
		return false
	}

	return !m.checkExpired(key, entry)
}

// Delete removes a key from the cache.
func (m *memoryCache[T]) Delete(_ context.Context, key string) error {
	if m.closed.Load() {
		return nil
	}

	if _, loaded := m.data.LoadAndDelete(key); loaded {
		m.evictionHandler.OnEvict(key)
		m.size.Add(-1)
	}

	return nil
}

// Clear removes all entries from the cache.
func (m *memoryCache[T]) Clear(_ context.Context) error {
	if m.closed.Load() {
		return nil
	}

	// Clear all entries and reset eviction handler
	m.data.Clear()
	m.evictionHandler.Reset()
	m.size.Store(0)

	return nil
}

// Keys returns all keys in the cache, optionally filtered by prefix.
func (m *memoryCache[T]) Keys(_ context.Context, prefix ...string) ([]string, error) {
	if m.closed.Load() {
		return nil, nil
	}

	var keys []string

	prefixStr := ""
	if len(prefix) > 0 {
		prefixStr = prefix[0]
	}

	if prefixStr == "" {
		// If no prefix, return all keys
		m.data.Range(func(key string, entry *cacheEntry[T]) bool {
			if !entry.isExpired() {
				keys = append(keys, key)
			}

			return true
		})
	} else {
		// With prefix, return only matching keys
		m.data.Range(func(key string, entry *cacheEntry[T]) bool {
			if strings.HasPrefix(key, prefixStr) && !entry.isExpired() {
				keys = append(keys, key)
			}

			return true
		})
	}

	return keys, nil
}

// ForEach iterates over all key-value pairs in the cache, optionally filtered by prefix.
// The iteration stops if the callback returns false.
func (m *memoryCache[T]) ForEach(_ context.Context, callback func(key string, value T) bool, prefix ...string) error {
	if m.closed.Load() {
		return nil
	}

	prefixStr := ""
	if len(prefix) > 0 {
		prefixStr = prefix[0]
	}

	m.data.Range(func(key string, entry *cacheEntry[T]) bool {
		// Check prefix filter
		if prefixStr != "" && !strings.HasPrefix(key, prefixStr) {
			return true
		}

		// Check expiration
		if entry.isExpired() {
			return true
		}

		// Call the callback, return false if it wants to stop iteration
		return callback(key, entry.data)
	})

	return nil
}

// Size returns the number of entries in the cache.
func (m *memoryCache[T]) Size(_ context.Context) (int64, error) {
	if m.closed.Load() {
		return 0, nil
	}

	return m.size.Load(), nil
}

// Close stops the background GC goroutine and marks the cache as closed.
func (m *memoryCache[T]) Close() error {
	if m.closed.Swap(true) {
		// Already closed
		return nil
	}

	// Signal GC goroutine to stop
	close(m.stopGC)

	return nil
}

// runGC runs background garbage collection to clean up expired entries.
func (m *memoryCache[T]) runGC() {
	ticker := time.NewTicker(m.gcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpired()
		case <-m.stopGC:
			return
		}
	}
}

// cleanupExpired removes all expired entries from the cache.
func (m *memoryCache[T]) cleanupExpired() {
	if m.closed.Load() {
		return
	}

	var keysToDelete []string

	// Find expired entries
	m.data.Range(func(key string, entry *cacheEntry[T]) bool {
		if entry.isExpired() {
			keysToDelete = append(keysToDelete, key)
		}

		return true
	})

	// Delete expired entries and notify eviction handler
	for _, key := range keysToDelete {
		if _, loaded := m.data.LoadAndDelete(key); loaded {
			m.evictionHandler.OnEvict(key)
			m.size.Add(-1)
		}
	}
}
