package lock

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/lock"
)

func TestNewLocker(t *testing.T) {
	t.Run("NilClientFallsBackToMemory", func(t *testing.T) {
		locker := newLocker(nil)
		assert.IsType(t, lock.NewMemoryLocker(), locker, "without Redis the default locker must be the in-process one")
	})

	t.Run("RedisClientSelectsRedisLocker", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{Addr: "localhost:0"})
		t.Cleanup(func() { _ = client.Close() })

		locker := newLocker(client)
		assert.IsType(t, lock.NewRedisLocker(client), locker, "with Redis available the default locker must span replicas")
	})
}
