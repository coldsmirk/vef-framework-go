package bootmodules

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/api"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/cron"
	"github.com/coldsmirk/vef-framework-go/internal/event"
	"github.com/coldsmirk/vef-framework-go/internal/expression"
	"github.com/coldsmirk/vef-framework-go/internal/mcp"
	"github.com/coldsmirk/vef-framework-go/internal/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/mold"
	"github.com/coldsmirk/vef-framework-go/internal/monitor"
	"github.com/coldsmirk/vef-framework-go/internal/redis"
	"github.com/coldsmirk/vef-framework-go/internal/schema"
	"github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/internal/sequence"
	"github.com/coldsmirk/vef-framework-go/internal/storage"
)

// Core returns the canonical list of business modules shared by the
// production boot sequence (vef.Run) and the test harness
// (internal/apptest), so the two FX graphs cannot drift. The config,
// datasource, and FX-logger modules are intentionally excluded: production
// and test wire those differently (real config/datasource vs. NopConfig +
// an injected test database), while the business modules below must be
// identical in both. FX resolves construction order by dependency, so the
// slice order here is for readability only.
func Core() []fx.Option {
	return []fx.Option{
		middleware.Module,
		api.Module,
		security.Module,
		event.Module,
		expression.Module,
		cqrs.Module,
		cron.Module,
		redis.Module,
		mold.Module,
		storage.Module,
		sequence.Module,
		event.OutboxModule,
		event.RedisStreamTransportModule,
		event.InboxModule,
		schema.Module,
		monitor.Module,
		mcp.Module,
		app.Module,
	}
}
