package event

import (
	"go.uber.org/fx"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	pubredisstream "github.com/coldsmirk/vef-framework-go/event/transport/redisstream"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/redisstream"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var redisStreamLogger = logx.Named("event:redis_stream")

// RedisStreamTransportModule wires the cross-process Redis Streams
// transport. Disabled by default; enable via
// vef.event.transports.redis_stream.enabled = true.
//
// The *redis.Client dependency is optional so fx does not force the
// redis module to construct (and connect) when the transport is off —
// applications without redis configured can leave the module loaded
// without paying the connection penalty.
var RedisStreamTransportModule = fx.Module(
	"vef:event:redis_stream",
	fx.Provide(
		fx.Annotate(
			newRedisStreamTransport,
			fx.ParamTags(``, `optional:"true"`),
			fx.ResultTags(`group:"vef:event:transports"`),
			fx.As(new(transport.Transport)),
		),
	),
)

func newRedisStreamTransport(cfg *config.EventConfig, client *goredis.Client) transport.Transport {
	if !cfg.Transports.RedisStream.Enabled || client == nil {
		return nil
	}

	rsCfg := pubredisstream.Config{
		StreamPrefix:  cfg.Transports.RedisStream.StreamPrefix,
		MaxLenApprox:  cfg.Transports.RedisStream.MaxLenApprox,
		BlockTimeout:  cfg.Transports.RedisStream.BlockTimeout,
		ClaimIdle:     cfg.Transports.RedisStream.ClaimIdle,
		ClaimInterval: cfg.Transports.RedisStream.ClaimInterval,
		ConsumerID:    cfg.Transports.RedisStream.ConsumerID,
	}

	return redisstream.New(client, rsCfg, redisStreamLogger)
}
