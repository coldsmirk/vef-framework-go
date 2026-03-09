package testx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/coldsmirk/vef-framework-go/config"
)

func NewRedisContainer(ctx context.Context, t testing.TB) *RedisContainer {
	t.Helper()

	container, err := redis.Run(
		ctx,
		RedisImage,
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(DefaultContainerTimeout),
		),
	)
	require.NoError(t, err)
	t.Log("Redis container started successfully")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	rc := &RedisContainer{
		container: container,
		Redis: &config.RedisConfig{
			Host:     host,
			Port:     uint16(port.Int()),
			Database: 0,
		},
	}

	t.Cleanup(func() {
		if err := rc.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate redis container: %v", err)
		}
	})

	return rc
}

type RedisContainer struct {
	Redis *config.RedisConfig

	container *redis.RedisContainer
}

func (c *RedisContainer) Terminate(ctx context.Context) error {
	return c.container.Terminate(ctx)
}
