package lock

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func TestRedisLocker(t *testing.T) {
	ctx := context.Background()
	container := testx.NewRedisContainer(ctx, t)

	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", container.Redis.Host, container.Redis.Port),
		DB:   int(container.Redis.Database),
	})
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Ping(ctx).Err(), "should connect to Redis")

	locker := NewRedisLocker(client)

	steal := func(t *testing.T, name string) {
		t.Helper()
		require.NoError(t, client.Del(ctx, redisLockKeyPrefix+name).Err(), "deleting the lock key out-of-band should succeed")
	}

	runLockerContract(t, locker, steal)

	t.Run("AcquireIsIdempotentForClientRetry", func(t *testing.T) {
		name := "redis-acquire:" + t.Name()
		keys := []string{redisLockKeyPrefix + name, redisLockFencingKey}

		first, err := acquireScript.Run(ctx, client, keys, "owner-a", redisTTLMillis(time.Second)).Int64()
		require.NoError(t, err, "the first script execution should acquire the lock")
		require.Positive(t, first, "a successful acquisition should allocate a positive fencing token")

		retried, err := acquireScript.Run(ctx, client, keys, "owner-a", redisTTLMillis(time.Second)).Int64()
		require.NoError(t, err, "replaying the same ownership token should recover an ambiguous response")
		assert.Equal(t, first, retried, "a client retry must reuse the original fencing token")

		contended, err := acquireScript.Run(ctx, client, keys, "owner-b", redisTTLMillis(time.Second)).Int64()
		require.NoError(t, err, "a competing acquisition should return a contention result")
		assert.Zero(t, contended, "another ownership token must not acquire the held lock")

		counter, err := client.Get(ctx, keys[1]).Int64()
		require.NoError(t, err, "the fencing counter should remain readable")
		assert.Equal(t, first, counter, "retries and failed contenders must not advance the fencing counter")

		require.NoError(t, client.Del(ctx, keys[0]).Err(), "cleanup should delete the lock key")
	})

	t.Run("ExpiredOwnerCannotJumpSuccessorFencingToken", func(t *testing.T) {
		name := "redis-fencing:" + t.Name()
		keys := []string{redisLockKeyPrefix + name, redisLockFencingKey}

		first, err := acquireScript.Run(ctx, client, keys, "owner-a", redisTTLMillis(50*time.Millisecond)).Int64()
		require.NoError(t, err, "the first owner should acquire the lock")

		require.Eventually(t, func() bool {
			return client.Exists(ctx, keys[0]).Val() == 0
		}, time.Second, 10*time.Millisecond, "the first lease should expire")

		successor, err := acquireScript.Run(ctx, client, keys, "owner-b", redisTTLMillis(time.Second)).Int64()
		require.NoError(t, err, "the successor should acquire the expired lock")
		assert.Greater(t, successor, first, "the successor must receive the next fencing token")

		staleRetry, err := acquireScript.Run(ctx, client, keys, "owner-a", redisTTLMillis(time.Second)).Int64()
		require.NoError(t, err, "the stale retry should return a contention result")
		assert.Zero(t, staleRetry, "an expired owner must not obtain a newer fencing token")

		counter, err := client.Get(ctx, keys[1]).Int64()
		require.NoError(t, err, "the fencing counter should remain readable")
		assert.Equal(t, successor, counter, "the stale retry must not advance the fencing counter")

		require.NoError(t, client.Del(ctx, keys[0]).Err(), "cleanup should delete the lock key")
	})

	t.Run("ReleaseIsIdempotentForClientRetry", func(t *testing.T) {
		name := "redis-release:" + t.Name()
		lockKey := redisLockKeyPrefix + name
		acquireKeys := []string{lockKey, redisLockFencingKey}
		releaseKeys := []string{lockKey, redisLockReleaseAckPrefix + "operation-a"}

		_, err := acquireScript.Run(ctx, client, acquireKeys, "owner-a", redisTTLMillis(time.Second)).Int64()
		require.NoError(t, err, "the first owner should acquire the lock")

		released, err := releaseScript.Run(
			ctx,
			client,
			releaseKeys,
			"owner-a",
			redisTTLMillis(redisReleaseAckTTL),
		).Int()
		require.NoError(t, err, "the first release execution should succeed")
		require.Equal(t, 1, released, "the first release should delete the owned lock")

		_, err = acquireScript.Run(ctx, client, acquireKeys, "owner-b", redisTTLMillis(time.Second)).Int64()
		require.NoError(t, err, "a successor should acquire after the release")

		retried, err := releaseScript.Run(
			ctx,
			client,
			releaseKeys,
			"owner-a",
			redisTTLMillis(redisReleaseAckTTL),
		).Int()
		require.NoError(t, err, "replaying the same release operation should recover an ambiguous response")
		assert.Equal(t, 1, retried, "the acknowledged operation should remain successful")

		newOperation, err := releaseScript.Run(
			ctx,
			client,
			[]string{lockKey, redisLockReleaseAckPrefix + "operation-b"},
			"owner-a",
			redisTTLMillis(redisReleaseAckTTL),
		).Int()
		require.NoError(t, err, "a distinct stale release should return a not-held result")
		assert.Zero(t, newOperation, "a new release operation from the stale owner must not be acknowledged")

		owner, err := client.HGet(ctx, lockKey, "owner").Result()
		require.NoError(t, err, "the successor lock should remain readable")
		assert.Equal(t, "owner-b", owner, "replaying a stale release must not delete the successor")

		ackTTL, err := client.PTTL(ctx, releaseKeys[1]).Result()
		require.NoError(t, err, "the release acknowledgement TTL should be readable")
		assert.Positive(t, ackTTL, "release acknowledgements must expire")

		require.NoError(t, client.Del(ctx, lockKey).Err(), "cleanup should delete the successor lock key")
	})

	t.Run("DynamicNamesShareOnePersistentFencingCounter", func(t *testing.T) {
		first, err := locker.TryAcquire(ctx, "redis-global-fencing:first")
		require.NoError(t, err, "the first name should acquire")
		require.NoError(t, first.Release(ctx), "the first name should release")

		second, err := locker.TryAcquire(ctx, "redis-global-fencing:second")
		require.NoError(t, err, "the second name should acquire")
		assert.Greater(t, second.FencingToken(), first.FencingToken(), "the shared sequence should advance across names")
		require.NoError(t, second.Release(ctx), "the second name should release")

		lockKeys, err := client.Keys(ctx, redisLockKeyPrefix+"redis-global-fencing:*").Result()
		require.NoError(t, err, "released lock keys should be enumerable")
		assert.Empty(t, lockKeys, "released dynamic lock names must not leave persistent keys")

		counterType, err := client.Type(ctx, redisLockFencingKey).Result()
		require.NoError(t, err, "the shared fencing counter should remain readable")
		assert.Equal(t, "string", counterType, "one shared string counter should be the only persistent fencing state")

		ackKeys, err := client.Keys(ctx, redisLockReleaseAckPrefix+"*").Result()
		require.NoError(t, err, "release acknowledgements should be enumerable")
		require.NotEmpty(t, ackKeys, "successful releases should record retry acknowledgements")

		for _, key := range ackKeys {
			ackTTL, ttlErr := client.PTTL(ctx, key).Result()
			require.NoError(t, ttlErr, "release acknowledgement TTL should be readable")
			assert.Positive(t, ackTTL, "release acknowledgement %q must not persist forever", key)
		}
	})
}

// TestNewRedisLocker needs no container, so it stays outside the Docker-gated
// suite and runs everywhere.
func TestNewRedisLocker(t *testing.T) {
	t.Run("NilClientPanics", func(t *testing.T) {
		assert.Panics(t, func() { NewRedisLocker(nil) }, "a nil client must fail fast at construction")
	})
}
