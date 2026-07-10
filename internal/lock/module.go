package lock

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/lock"
)

var logger = logx.Named("lock")

// Module provides the framework's default distributed lock.Locker.
var Module = fx.Module(
	"vef:lock",
	fx.Provide(
		fx.Annotate(
			newLocker,
			fx.ParamTags(`optional:"true"`),
		),
	),
)

// newLocker selects the default Locker by deployment topology: Redis-backed
// whenever the Redis client is available (correct on both single- and
// multi-node), otherwise the in-process memory locker with a loud warning,
// since a lock that does not span replicas silently stops guarding invariants
// the moment a second replica starts. Unlike the session/guard stores this is
// deliberately not "memory unless decorated": applications reach for a Locker
// precisely because they scale out. Swap in a custom backend via fx.Decorate.
func newLocker(client *redis.Client) lock.Locker {
	if client != nil {
		return lock.NewRedisLocker(client)
	}

	logger.Warnf("vef.redis is disabled; using the in-process memory locker, which provides NO cross-replica mutual exclusion — enable vef.redis before scaling beyond one replica.")

	return lock.NewMemoryLocker()
}
