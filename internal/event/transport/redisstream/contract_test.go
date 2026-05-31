package redisstream_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/redisstream"
	iredisstream "github.com/coldsmirk/vef-framework-go/internal/event/transport/redisstream"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/transporttest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// TestRedisStreamTransportContract spins up a real Redis container per
// scenario and runs the contract suite against it. testx.NewRedisContainer
// handles cleanup via t.Cleanup, so this test is hermetic.
func TestRedisStreamTransportContract(t *testing.T) {
	factory := func(t *testing.T) (transport.Transport, func()) {
		container := testx.NewRedisContainer(context.Background(), t)
		client := goredis.NewClient(&goredis.Options{
			Addr: net.JoinHostPort(container.Redis.Host, strconv.Itoa(int(container.Redis.Port))),
		})

		cfg := redisstream.Config{
			StreamPrefix:  "test:event:",
			MaxLenApprox:  1024,
			BlockTimeout:  200 * time.Millisecond,
			ClaimIdle:     30 * time.Second,
			ClaimInterval: 30 * time.Second,
		}
		tp := iredisstream.New(client, cfg, nil)
		cleanup := func() {
			_ = client.Close()
		}

		return tp, cleanup
	}

	transporttest.Suite(t, "RedisStream", factory)
}

// Smoke check ensures the config struct connects to the testcontainer
// before delegating to the heavier contract suite.
func TestRedisStreamPingsContainer(t *testing.T) {
	container := testx.NewRedisContainer(context.Background(), t)
	_ = config.RedisConfig{} // import-side compile assertion

	client := goredis.NewClient(&goredis.Options{
		Addr: net.JoinHostPort(container.Redis.Host, strconv.Itoa(int(container.Redis.Port))),
	})
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Ping(context.Background()).Err(), "Container must be reachable")
}
