package lock

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Lock keys expire with their leases. The one global fencing counter is the
// only persistent key, avoiding one immortal counter key per dynamic lock
// name while still providing a sequence that is strictly ordered per name.
const (
	redisLockKeyPrefix        = "vef:lock:key:"
	redisLockReleaseAckPrefix = "vef:lock:release:"
	redisLockFencingKey       = "vef:lock:fencing"
	redisReleaseAckTTL        = time.Minute
)

// acquireScript installs the ownership token and allocates its fencing token
// in one Redis operation. Replaying the same random token after an ambiguous
// client retry returns the original fencing token and renews the not-yet-
// delivered lease instead of reporting a false contention result.
var acquireScript = redis.NewScript(`
local owner = redis.call("HGET", KEYS[1], "owner")
if owner then
	if owner ~= ARGV[1] then
		return "0"
	end

	local fencing = redis.call("HGET", KEYS[1], "fencing")
	if not fencing or not tonumber(fencing) or tonumber(fencing) <= 0 then
		return redis.error_reply("lock fencing token missing or invalid")
	end

	redis.call("PEXPIRE", KEYS[1], ARGV[2])
	return fencing
end

local next_fencing = redis.call("INCR", KEYS[2])
if next_fencing <= 0 then
	return redis.error_reply("lock fencing counter must be positive")
end

local fencing = redis.call("GET", KEYS[2])
redis.call("HSET", KEYS[1], "owner", ARGV[1], "fencing", fencing)
redis.call("PEXPIRE", KEYS[1], ARGV[2])
return fencing`)

// releaseScript deletes the lock key only while token still owns it, then
// records a short-lived acknowledgement for this release operation. Replaying
// the same operation after an ambiguous client retry returns success without
// touching a successor's lock, while a new Release call still reports not held.
var releaseScript = redis.NewScript(`
if redis.call("HGET", KEYS[1], "owner") == ARGV[1] then
	redis.call("DEL", KEYS[1])
	redis.call("SET", KEYS[2], "1", "PX", ARGV[2])
	return 1
end
if redis.call("EXISTS", KEYS[2]) == 1 then
	return 1
end
return 0`)

// refreshScript extends the lock key's TTL only while token still owns it.
var refreshScript = redis.NewScript(`
if redis.call("HGET", KEYS[1], "owner") == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0`)

// RedisLocker implements Locker on a single Redis instance with the
// atomic Lua acquisition + token-guarded release/refresh pattern, giving
// cross-replica mutual exclusion to every node sharing the Redis. It is not a
// Redlock implementation: quorum locking over independent Redis nodes is out
// of scope, and the lease-based caveats documented on Locker apply.
type RedisLocker struct {
	client *redis.Client
}

// NewRedisLocker creates a Redis-backed locker.
func NewRedisLocker(client *redis.Client) Locker {
	if client == nil {
		panic("lock: redis locker requires a non-nil redis client")
	}

	return &RedisLocker{client: client}
}

func (*RedisLocker) lockKey(name string) string { return redisLockKeyPrefix + name }
func (*RedisLocker) releaseAckKey(operation string) string {
	return redisLockReleaseAckPrefix + operation
}

func (r *RedisLocker) Acquire(ctx context.Context, name string, opts ...Option) (Lock, error) {
	cfg, err := resolveAcquireConfig(opts)
	if err != nil {
		return nil, err
	}

	return acquireLoop(ctx, cfg, func(ctx context.Context) (Lock, error) {
		return r.tryOnce(ctx, name, cfg)
	})
}

func (r *RedisLocker) TryAcquire(ctx context.Context, name string, opts ...Option) (Lock, error) {
	cfg, err := resolveAcquireConfig(opts)
	if err != nil {
		return nil, err
	}

	return r.tryOnce(ctx, name, cfg)
}

// tryOnce atomically installs the ownership token and draws its fencing token.
// The one global fencing counter deliberately carries no TTL: it must remain
// monotonic across every lock name for the Redis dataset's whole lifetime.
func (r *RedisLocker) tryOnce(ctx context.Context, name string, cfg acquireConfig) (Lock, error) {
	token, err := newLockToken()
	if err != nil {
		return nil, err
	}

	fencing, err := acquireScript.Run(
		ctx,
		r.client,
		[]string{r.lockKey(name), redisLockFencingKey},
		token,
		redisTTLMillis(cfg.ttl),
	).Int64()
	if err != nil {
		// A write followed by a lost response is indistinguishable from a command
		// that never reached Redis. Compensate with the same random token so an
		// ambiguous successful acquisition cannot become an orphaned lease.
		releaseCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			min(releaseTimeout, cfg.ttl),
		)
		defer cancel()

		if releaseErr := r.releaseToken(releaseCtx, name, token); releaseErr != nil && !errors.Is(releaseErr, ErrNotHeld) {
			logger.Warnf("Failed to release incompletely acquired lock %q: %v", name, releaseErr)
		}

		return nil, err
	}

	if fencing == 0 {
		return nil, ErrNotAcquired
	}

	backend := leaseBackend{release: r.releaseToken, refresh: r.refreshToken}

	return newLease(name, token, cfg, fencing, backend), nil
}

// releaseToken deletes the lock key if token still owns it.
func (r *RedisLocker) releaseToken(ctx context.Context, name, token string) error {
	operation, err := newLockToken()
	if err != nil {
		return err
	}

	deleted, err := releaseScript.Run(
		ctx,
		r.client,
		[]string{r.lockKey(name), r.releaseAckKey(operation)},
		token,
		redisTTLMillis(redisReleaseAckTTL),
	).Int()
	if err != nil {
		return err
	}

	if deleted == 0 {
		return ErrNotHeld
	}

	return nil
}

// refreshToken extends the lock key's TTL if token still owns it. The TTL is
// floored at one millisecond: PEXPIRE 0 would delete the key while reporting
// success, silently dropping the lease, so acquisition and refresh share the
// same conversion.
func (r *RedisLocker) refreshToken(ctx context.Context, name, token string, ttl time.Duration) error {
	extended, err := refreshScript.Run(ctx, r.client, []string{r.lockKey(name)}, token, redisTTLMillis(ttl)).Int()
	if err != nil {
		return err
	}

	if extended == 0 {
		return ErrNotHeld
	}

	return nil
}

func redisTTLMillis(ttl time.Duration) int64 {
	return max(ttl.Milliseconds(), 1)
}
