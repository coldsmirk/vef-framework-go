package lock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// The lock-key and fencing-counter namespaces are disjoint: deriving one from
// the other (e.g. "vef:lock:" + "fencing:") would let a lock literally named
// "fencing:x" collide with lock "x"'s counter, corrupting both.
const (
	redisLockKeyPrefix     = "vef:lock:key:"
	redisLockFencingPrefix = "vef:lock:fencing:"
)

// releaseScript deletes the lock key only while token still owns it, so a
// slow holder whose lease expired can never delete a successor's lock.
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0`)

// refreshScript extends the lock key's TTL only while token still owns it.
var refreshScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0`)

// RedisLocker implements Locker on a single Redis instance with the
// canonical SET NX PX + token-guarded Lua pattern, giving cross-replica
// mutual exclusion to every node sharing the Redis. It is not a Redlock
// implementation: quorum locking over independent Redis nodes is out of
// scope, and the lease-based caveats documented on Locker apply.
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

func (*RedisLocker) lockKey(name string) string    { return redisLockKeyPrefix + name }
func (*RedisLocker) fencingKey(name string) string { return redisLockFencingPrefix + name }

func (r *RedisLocker) Acquire(ctx context.Context, name string, opts ...Option) (Lock, error) {
	cfg := resolveAcquireConfig(opts)

	return acquireLoop(ctx, cfg, func(ctx context.Context) (Lock, error) {
		return r.tryOnce(ctx, name, cfg)
	})
}

func (r *RedisLocker) TryAcquire(ctx context.Context, name string, opts ...Option) (Lock, error) {
	return r.tryOnce(ctx, name, resolveAcquireConfig(opts))
}

// tryOnce performs a single SET NX PX attempt and, on success, draws the
// lease's fencing token. The fencing counter key deliberately carries no TTL:
// it must stay monotonic across the lock name's whole lifetime.
func (r *RedisLocker) tryOnce(ctx context.Context, name string, cfg acquireConfig) (Lock, error) {
	token, err := newLockToken()
	if err != nil {
		return nil, err
	}

	acquired, err := r.client.SetNX(ctx, r.lockKey(name), token, cfg.ttl).Result()
	if err != nil {
		return nil, err
	}

	if !acquired {
		return nil, ErrNotAcquired
	}

	fencing, err := r.client.Incr(ctx, r.fencingKey(name)).Result()
	if err != nil {
		// The lock key was written but the lease cannot be completed; free it
		// eagerly rather than blocking others until the TTL. The most likely
		// cause is a dead ctx, so the compensation runs on its own context.
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
		defer cancel()

		if releaseErr := r.releaseToken(releaseCtx, name, token); releaseErr != nil {
			logger.Warnf("Failed to release incompletely acquired lock %q: %v", name, releaseErr)
		}

		return nil, err
	}

	backend := leaseBackend{release: r.releaseToken, refresh: r.refreshToken}

	return newLease(name, token, cfg, fencing, backend), nil
}

// releaseToken deletes the lock key if token still owns it.
func (r *RedisLocker) releaseToken(ctx context.Context, name, token string) error {
	deleted, err := releaseScript.Run(ctx, r.client, []string{r.lockKey(name)}, token).Int()
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
// success, silently dropping the lease (SetNX rounds sub-millisecond TTLs up,
// so acquisition and refresh must agree).
func (r *RedisLocker) refreshToken(ctx context.Context, name, token string, ttl time.Duration) error {
	extended, err := refreshScript.Run(ctx, r.client, []string{r.lockKey(name)}, token, max(ttl.Milliseconds(), 1)).Int()
	if err != nil {
		return err
	}

	if extended == 0 {
		return ErrNotHeld
	}

	return nil
}
