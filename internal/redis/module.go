package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module provides Redis client functionality with automatic lifecycle management.
// When vef.redis.enabled=false the constructor returns nil; the lifecycle hooks
// guard against that so applications without Redis can still load the module
// without paying the connection penalty.
var Module = fx.Module(
	"vef:redis",
	fx.Provide(
		fx.Annotate(
			NewClient,
			fx.OnStart(func(ctx context.Context, client *redis.Client) error {
				if client == nil {
					return nil
				}

				if err := client.Ping(ctx).Err(); err != nil {
					return fmt.Errorf("failed to connect to redis: %w", err)
				}

				return logRedisServerInfo(ctx, client)
			}),
			fx.OnStop(func(client *redis.Client) error {
				if client == nil {
					return nil
				}

				logger.Info("Closing Redis client...")

				return client.Close()
			}),
		),
	),
)
