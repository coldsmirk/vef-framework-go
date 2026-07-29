package bootmodules

import (
	"slices"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/api"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/cron"
	"github.com/coldsmirk/vef-framework-go/internal/event"
	"github.com/coldsmirk/vef-framework-go/internal/expression"
	"github.com/coldsmirk/vef-framework-go/internal/js"
	"github.com/coldsmirk/vef-framework-go/internal/lock"
	"github.com/coldsmirk/vef-framework-go/internal/mcp"
	"github.com/coldsmirk/vef-framework-go/internal/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/mold"
	"github.com/coldsmirk/vef-framework-go/internal/monitor"
	"github.com/coldsmirk/vef-framework-go/internal/push"
	"github.com/coldsmirk/vef-framework-go/internal/redis"
	"github.com/coldsmirk/vef-framework-go/internal/schema"
	"github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/internal/sequence"
	"github.com/coldsmirk/vef-framework-go/internal/storage"
)

// Assemble composes the framework's boot options in the one order it supports:
// the caller's environment prefix (config, data sources, fx logger), then the
// canonical business modules, then the host's own options, then the trailing
// invoke that must observe every start hook appended above it.
//
// Both boot paths go through it — vef.Run and the internal/apptest harness — so
// neither can place the trailing slot wrong and the ordering has a single
// authority instead of two copies held together by comments.
//
// The trailing slot currently holds cron.StartScheduler, which must run after
// every module's start hook; see that function for why.
func Assemble(prefix, host []fx.Option) []fx.Option {
	opts := slices.Clone(prefix)
	opts = append(opts, core()...)
	opts = append(opts, host...)

	return append(opts, fx.Invoke(cron.StartScheduler))
}

// core returns the canonical list of business modules shared by the
// production boot sequence (vef.Run) and the test harness
// (internal/apptest), so the two FX graphs cannot drift. The config,
// datasource, and FX-logger modules are intentionally excluded: production
// and test wire those differently (real config/datasource vs. NopConfig +
// an injected test database), while the business modules below must be
// identical in both. FX resolves construction order by dependency, so the
// slice order here is for readability only.
func core() []fx.Option {
	return []fx.Option{
		middleware.Module,
		api.Module,
		security.Module,
		event.Module,
		expression.Module,
		js.Module,
		cqrs.Module,
		cron.Module,
		redis.Module,
		lock.Module,
		mold.Module,
		storage.Module,
		sequence.Module,
		event.TxMemoryTransportModule,
		event.OutboxModule,
		event.RedisStreamTransportModule,
		event.InboxModule,
		schema.Module,
		monitor.Module,
		mcp.Module,
		push.Module,
		app.Module,
	}
}
