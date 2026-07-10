package lock

import (
	"context"
	"fmt"
	"testing"

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
}

// TestNewRedisLocker needs no container, so it stays outside the Docker-gated
// suite and runs everywhere.
func TestNewRedisLocker(t *testing.T) {
	t.Run("NilClientPanics", func(t *testing.T) {
		assert.Panics(t, func() { NewRedisLocker(nil) }, "a nil client must fail fast at construction")
	})
}
