package security

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const redisLoginGuardPrefix = "vef:security:lockout:"

// RedisLoginGuard implements LoginGuard with Redis-backed counters shared across
// all nodes, the correct choice for multi-node deployments.
type RedisLoginGuard struct {
	policy LockoutPolicy
	client *redis.Client
}

// NewRedisLoginGuard creates a Redis-backed login guard governed by policy.
func NewRedisLoginGuard(client *redis.Client, policy LockoutPolicy) LoginGuard {
	return &RedisLoginGuard{policy: policy, client: client}
}

func (g *RedisLoginGuard) failureKey(attempt LoginAttempt) string {
	return redisLoginGuardPrefix + "fail:" + g.policy.storageKey(attempt)
}

func (g *RedisLoginGuard) cooldownKey(attempt LoginAttempt) string {
	return redisLoginGuardPrefix + "cooldown:" + g.policy.storageKey(attempt)
}

// Check reports whether an unexpired cooldown key exists for the identity.
func (g *RedisLoginGuard) Check(ctx context.Context, attempt LoginAttempt) (LoginDecision, error) {
	ttl, err := g.client.PTTL(ctx, g.cooldownKey(attempt)).Result()
	if err != nil {
		return LoginDecision{Allowed: true}, err
	}

	if ttl > 0 {
		return LoginDecision{Allowed: false, RetryAfter: ttl}, nil
	}

	return LoginDecision{Allowed: true}, nil
}

// RecordFailure increments the shared failure counter and, once the threshold is
// exceeded, sets a cooldown key whose TTL is the block duration.
func (g *RedisLoginGuard) RecordFailure(ctx context.Context, attempt LoginAttempt) (LoginDecision, error) {
	failureKey := g.failureKey(attempt)

	count, err := g.client.Incr(ctx, failureKey).Result()
	if err != nil {
		return LoginDecision{Allowed: true}, err
	}

	// Refresh the counting window on each failure; a spell of no failures this
	// long lets the key expire and the counter reset.
	if err := g.client.Expire(ctx, failureKey, g.policy.Window).Err(); err != nil {
		return LoginDecision{Allowed: true}, err
	}

	cooldown := g.policy.cooldownFor(int(count))
	if cooldown <= 0 {
		return LoginDecision{Allowed: true}, nil
	}

	err = g.client.Set(ctx, g.cooldownKey(attempt), "1", cooldown).Err()

	return LoginDecision{Allowed: false, RetryAfter: cooldown}, err
}

// RecordSuccess clears the failure counter and any open cooldown.
func (g *RedisLoginGuard) RecordSuccess(ctx context.Context, attempt LoginAttempt) error {
	return g.client.Del(ctx, g.failureKey(attempt), g.cooldownKey(attempt)).Err()
}
