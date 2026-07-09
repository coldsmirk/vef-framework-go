package security

import (
	"context"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/cache"
)

// MemoryLoginGuard implements LoginGuard with in-memory counters. It is suitable
// for single-instance deployments; multi-node deployments should use
// RedisLoginGuard so the failure counters are shared across nodes.
type MemoryLoginGuard struct {
	policy    LockoutPolicy
	failures  cache.Cache[int]
	cooldowns cache.Cache[int64]
	mu        sync.Mutex
}

// NewMemoryLoginGuard creates an in-memory login guard governed by policy.
func NewMemoryLoginGuard(policy LockoutPolicy) LoginGuard {
	return &MemoryLoginGuard{
		policy:    policy,
		failures:  cache.NewMemory[int](),
		cooldowns: cache.NewMemory[int64](),
	}
}

// Check reports whether the identity is currently within a cooldown window.
func (g *MemoryLoginGuard) Check(ctx context.Context, attempt LoginAttempt) (LoginDecision, error) {
	key := g.policy.storageKey(attempt)

	if until, ok := g.cooldowns.Get(ctx, key); ok {
		if remaining := time.Until(time.Unix(0, until)); remaining > 0 {
			return LoginDecision{Allowed: false, RetryAfter: remaining}, nil
		}
	}

	return LoginDecision{Allowed: true}, nil
}

// RecordFailure increments the failure counter and, once the threshold is
// exceeded, opens a cooldown window per the policy.
func (g *MemoryLoginGuard) RecordFailure(ctx context.Context, attempt LoginAttempt) (LoginDecision, error) {
	key := g.policy.storageKey(attempt)

	g.mu.Lock()
	defer g.mu.Unlock()

	count, _ := g.failures.Get(ctx, key)
	count++
	if err := g.failures.Set(ctx, key, count, g.policy.Window); err != nil {
		return LoginDecision{Allowed: true}, err
	}

	cooldown := g.policy.cooldownFor(count)
	if cooldown <= 0 {
		return LoginDecision{Allowed: true}, nil
	}

	until := time.Now().Add(cooldown)
	err := g.cooldowns.Set(ctx, key, until.UnixNano(), cooldown)

	return LoginDecision{Allowed: false, RetryAfter: cooldown}, err
}

// RecordSuccess clears the failure counter and any open cooldown.
func (g *MemoryLoginGuard) RecordSuccess(ctx context.Context, attempt LoginAttempt) error {
	key := g.policy.storageKey(attempt)

	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.failures.Delete(ctx, key); err != nil {
		return err
	}

	return g.cooldowns.Delete(ctx, key)
}
