package vef

import (
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/coldsmirk/vef-framework-go/internal/api"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/config"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/cron"
	"github.com/coldsmirk/vef-framework-go/internal/datasource"
	"github.com/coldsmirk/vef-framework-go/internal/event"
	"github.com/coldsmirk/vef-framework-go/internal/expression"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/internal/mcp"
	"github.com/coldsmirk/vef-framework-go/internal/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/mold"
	"github.com/coldsmirk/vef-framework-go/internal/monitor"
	"github.com/coldsmirk/vef-framework-go/internal/redis"
	"github.com/coldsmirk/vef-framework-go/internal/schema"
	"github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/internal/sequence"
	"github.com/coldsmirk/vef-framework-go/internal/storage"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// Default timeout for framework startup and shutdown.
const defaultTimeout = 30 * time.Second

func newFxLogger() fxevent.Logger {
	return &fxevent.SlogLogger{
		Logger: ilogx.NewSLogger("vef", 5, logx.LevelWarn),
	}
}

// Run starts the VEF framework with the provided options.
// It initializes all core modules and runs the application.
func Run(options ...fx.Option) {
	// Core framework modules
	opts := []fx.Option{
		fx.WithLogger(newFxLogger),
		config.Module,
		datasource.Module,
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

	opts = append(opts, options...)
	opts = append(
		opts,
		fx.Invoke(startApp),
		fx.StartTimeout(defaultTimeout),
		fx.StopTimeout(defaultTimeout*2),
	)

	app := fx.New(opts...)
	app.Run()
}
