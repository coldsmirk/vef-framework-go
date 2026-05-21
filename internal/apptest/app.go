package apptest

import (
	"database/sql"
	"testing"

	"github.com/uptrace/bun"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/api"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	iconfig "github.com/coldsmirk/vef-framework-go/internal/config"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/cron"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/event"
	"github.com/coldsmirk/vef-framework-go/internal/mcp"
	"github.com/coldsmirk/vef-framework-go/internal/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/mold"
	"github.com/coldsmirk/vef-framework-go/internal/monitor"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/redis"
	"github.com/coldsmirk/vef-framework-go/internal/schema"
	"github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/internal/storage"
)

// NopConfig implements config.Config for testing without file dependencies.
type NopConfig struct{}

func (*NopConfig) Unmarshal(string, any) error {
	return nil
}

// NewTestApp creates a test application with Fx dependency injection.
// Returns the app instance and a cleanup function.
func NewTestApp(t testing.TB, options ...fx.Option) (*app.App, func()) {
	return newTestApp(t, buildOptions(options...))
}

// NewTestAppWithDB creates a test application that uses an existing *bun.DB
// instead of creating a new connection via database.Module.
// This avoids redundant database connections when tests already manage their own.
func NewTestAppWithDB(t testing.TB, db *bun.DB, options ...fx.Option) (*app.App, func()) {
	return newTestApp(t, buildOptionsWithDB(db, options...))
}

func newTestApp(t testing.TB, opts []fx.Option) (*app.App, func()) {
	var testApp *app.App

	opts = append(opts, fx.Populate(&testApp))
	fxApp := fxtest.New(t, opts...)
	fxApp.RequireStart()

	return testApp, fxApp.RequireStop
}

func coreOptions() []fx.Option {
	return []fx.Option{
		fx.NopLogger,
		fx.Replace(
			fx.Annotate(&NopConfig{}, fx.As(new(config.Config))),
			&config.AppConfig{
				Name:      "test-app",
				Port:      0,
				BodyLimit: "100mib",
			},
			// Enable the outbox transport by default in tests so command
			// handlers that publish events with event.WithTx have a real
			// TxTransport to route to. Tests can override this with
			// fx.Replace if they want different routing.
			&config.EventConfig{
				DefaultTransport: "memory",
				Transports: config.EventTransportsConfig{
					Outbox: config.EventOutboxTransportConfig{Enabled: true},
				},
				Routing: []config.EventRoutingRule{
					{Pattern: "approval.*", Transports: []string{"outbox"}},
					// Storage events publish with event.WithTx; the storage
					// module's start-up check requires a transactional route
					// for vef.storage.*.
					{Pattern: "vef.storage.*", Transports: []string{"outbox"}},
				},
				Middleware: config.EventMiddlewareConfig{
					Logging: true,
					Tracing: true,
					Metrics: true,
					Recover: true,
					Inbox:   true,
				},
			},
		),
		iconfig.Module,
		orm.Module,
		middleware.Module,
		api.Module,
		security.Module,
		event.Module,
		cqrs.Module,
		cron.Module,
		redis.Module,
		mold.Module,
		storage.Module,
		monitor.Module,
		schema.Module,
		event.OutboxModule,
		// RedisStreamTransportModule is safe to load here because the
		// default RedisConfig has Enabled=false, so redis.NewClient
		// returns nil, the Ping hook short-circuits, and the redisstream
		// constructor falls back to "no transport contributed". Tests
		// that actually exercise redisstream provide an enabled config
		// (see testx.NewRedisContainer).
		event.RedisStreamTransportModule,
		event.InboxModule,
		mcp.Module,
		app.Module,
	}
}

func buildOptions(options ...fx.Option) []fx.Option {
	return buildOptionsWith(database.Module, options...)
}

func buildOptionsWithDB(existingDB *bun.DB, options ...fx.Option) []fx.Option {
	dbProvider := fx.Provide(
		fx.Annotate(
			func() *bun.DB { return existingDB },
			fx.As(new(bun.IDB)),
			fx.As(fx.Self()),
		),
		func(db *bun.DB) *sql.DB { return db.DB },
	)

	return buildOptionsWith(dbProvider, options...)
}

func buildOptionsWith(dbOption fx.Option, extra ...fx.Option) []fx.Option {
	opts := append(coreOptions(), dbOption)

	return append(opts, extra...)
}
