package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module provides Redis client functionality with automatic lifecycle management.
var Module = fx.Module(
	"vef:redis",
	fx.Provide(
		fx.Annotate(
			NewClient,
			fx.OnStart(func(ctx context.Context, client *redis.Client) error {
				if err := client.Ping(ctx).Err(); err != nil {
					return fmt.Errorf("failed to connect to redis: %w", err)
				}

				return logRedisServerInfo(ctx, client)
			}),
			fx.OnStop(func(client *redis.Client) error {
				logger.Info("Closing Redis client...")

				return client.Close()
			}),
		),
	),
)
