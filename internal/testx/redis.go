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

	host, port := hostPort(ctx, t, container, "6379")
	terminateOnCleanup(ctx, t, container, "redis")

	return &RedisContainer{
		container: container,
		Redis: &config.RedisConfig{
			Enabled:  true,
			Host:     host,
			Port:     port.Num(),
			Database: 0,
		},
	}
}

type RedisContainer struct {
	Redis *config.RedisConfig

	container *redis.RedisContainer
}
