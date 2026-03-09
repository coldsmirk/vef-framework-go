package redis

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var logger = log.Named("redis")

// NewClient creates a Redis client with adaptive pool sizing to balance performance and resource usage.
func NewClient(cfg *config.RedisConfig, appCfg *config.AppConfig) *redis.Client {
	clientName := lo.CoalesceOrEmpty(appCfg.Name, "vef-app")

	poolSize := getPoolSize()
	poolTimeout, idleTimeout, maxRetries := getConnectionConfig(poolSize)

	options := &redis.Options{
		ClientName:            clientName,
		IdentitySuffix:        "vef",
		Protocol:              3,
		ContextTimeoutEnabled: true,
		Network:               lo.CoalesceOrEmpty(cfg.Network, "tcp"),
		Addr:                  buildRedisAddr(cfg),
		Username:              cfg.User,
		Password:              cfg.Password,
		DB:                    int(cfg.Database),

		PoolSize:    poolSize,
		PoolTimeout: poolTimeout,
		MaxRetries:  maxRetries,

		MinIdleConns:    poolSize / 4,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: idleTimeout,

		DialTimeout:  10 * time.Second,
		ReadTimeout:  6 * time.Second,
		WriteTimeout: 6 * time.Second,
	}

	client := redis.NewClient(options)

	logger.Infof(
		"Redis client configured - Pool: %d, Timeout: %v, Idle: %v, Retries: %d",
		poolSize, poolTimeout, idleTimeout, maxRetries,
	)

	return client
}

// getPoolSize scales pool size with CPU cores (2x GOMAXPROCS) to handle concurrent requests efficiently
// while capping at 100 to prevent resource exhaustion on large machines.
func getPoolSize() int {
	return min(max(runtime.GOMAXPROCS(0)*2, 4), 100)
}

// getConnectionConfig scales pool timeout with pool size to reduce contention under load.
func getConnectionConfig(poolSize int) (poolTimeout, idleTimeout time.Duration, maxRetries int) {
	poolTimeout = min(max(time.Duration(poolSize*50)*time.Millisecond, 1*time.Second), 5*time.Second)
	idleTimeout = 5 * time.Minute
	maxRetries = 3

	return poolTimeout, idleTimeout, maxRetries
}

func logRedisServerInfo(ctx context.Context, client *redis.Client) error {
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return fmt.Errorf("failed to get redis server info: %w", err)
	}

	version := "unknown"

	for line := range strings.SplitSeq(info, "\r\n") {
		if after, ok := strings.CutPrefix(line, "redis_version:"); ok {
			version = strings.TrimSpace(after)

			break
		}
	}

	logger.Infof("Connected to Redis server: %s, version: %s", client.Options().Addr, version)

	return nil
}

func buildRedisAddr(cfg *config.RedisConfig) string {
	host := lo.CoalesceOrEmpty(cfg.Host, "127.0.0.1")
	port := lo.CoalesceOrEmpty(cfg.Port, 6379)

	return fmt.Sprintf("%s:%d", host, port)
}

// HealthCheck verifies Redis availability for monitoring endpoints.
func HealthCheck(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}
