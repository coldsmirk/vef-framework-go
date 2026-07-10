package lock

import (
	"context"
	"sync"
	"time"
)

// MemoryLocker implements Locker with in-process state. It provides NO
// cross-replica mutual exclusion — it suits single-instance deployments and
// tests; multi-node deployments must use RedisLocker. Semantics (TTL expiry,
// ownership tokens, fencing tokens, waiting) match RedisLocker exactly so
// behavior does not change between environments.
type MemoryLocker struct {
	mu      sync.Mutex
	holders map[string]*memoryHolder
	fencing int64
}

type memoryHolder struct {
	token     string
	expiresAt time.Time
	timer     *time.Timer
}

// NewMemoryLocker creates an empty in-process locker.
func NewMemoryLocker() Locker {
	return &MemoryLocker{
		holders: make(map[string]*memoryHolder),
	}
}

func (m *MemoryLocker) Acquire(ctx context.Context, name string, opts ...Option) (Lock, error) {
	cfg, err := resolveAcquireConfig(opts)
	if err != nil {
		return nil, err
	}

	return acquireLoop(ctx, cfg, func(ctx context.Context) (Lock, error) {
		return m.tryOnce(ctx, name, cfg)
	})
}

func (m *MemoryLocker) TryAcquire(ctx context.Context, name string, opts ...Option) (Lock, error) {
	cfg, err := resolveAcquireConfig(opts)
	if err != nil {
		return nil, err
	}

	return m.tryOnce(ctx, name, cfg)
}

// tryOnce performs a single acquisition attempt under the mutex. It honors
// context cancellation up front so a dead context fails the acquisition just
// as it would against Redis.
func (m *MemoryLocker) tryOnce(ctx context.Context, name string, cfg acquireConfig) (Lock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	token, err := newLockToken()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if holder := m.holders[name]; holder != nil && holder.expiresAt.After(time.Now()) {
		return nil, ErrNotAcquired
	}

	expiresAt := time.Now().Add(cfg.ttl)
	holder := &memoryHolder{token: token, expiresAt: expiresAt}
	holder.timer = m.expiryTimer(name, token, expiresAt)
	m.holders[name] = holder
	m.fencing++

	backend := leaseBackend{release: m.releaseToken, refresh: m.refreshToken}

	return newLease(name, token, cfg, m.fencing, backend), nil
}

// releaseToken removes the holder entry if token still owns a live lease.
func (m *MemoryLocker) releaseToken(_ context.Context, name, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	holder := m.holders[name]
	if holder == nil || holder.token != token || !holder.expiresAt.After(time.Now()) {
		return ErrNotHeld
	}

	holder.timer.Stop()
	delete(m.holders, name)

	return nil
}

// refreshToken extends the lease if token still owns a live lease.
func (m *MemoryLocker) refreshToken(_ context.Context, name, token string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	holder := m.holders[name]
	if holder == nil || holder.token != token || !holder.expiresAt.After(time.Now()) {
		return ErrNotHeld
	}

	holder.timer.Stop()
	holder.expiresAt = time.Now().Add(ttl)
	holder.timer = m.expiryTimer(name, token, holder.expiresAt)

	return nil
}

func (m *MemoryLocker) expiryTimer(name, token string, expiresAt time.Time) *time.Timer {
	return time.AfterFunc(time.Until(expiresAt), func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		holder := m.holders[name]
		if holder == nil || holder.token != token || !holder.expiresAt.Equal(expiresAt) {
			return
		}

		delete(m.holders, name)
	})
}
