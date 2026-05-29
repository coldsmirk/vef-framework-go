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
// It returns nil for every Unmarshal call except vef.data_sources, where it
// injects a default in-memory SQLite primary so the framework's
// newDataSourcesConfig (which requires a primary entry) can boot. Tests that
// need a different primary or extra sources override the result via
// fx.Replace(&config.DataSourcesConfig{...}) or apptest.WithDataSourcesConfig.
type NopConfig struct{}

func (*NopConfig) Unmarshal(key string, target any) error {
	if key == "vef.data_sources" {
		if m, ok := target.(*map[string]config.DataSourceConfig); ok {
			*m = map[string]config.DataSourceConfig{
				"primary": {Kind: config.SQLite},
			}
		}
	}

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
	return NewTestAppWithDBConfig(t, db, config.DataSourceConfig{Kind: config.SQLite}, options...)
}

// NewTestAppWithDBConfig creates a test application that uses an existing
// *bun.DB and the matching primary data source config. Use this for tests that
// reuse a non-SQLite connection so migration and schema services pick the
// correct dialect metadata.
func NewTestAppWithDBConfig(
	t testing.TB,
	db *bun.DB,
	cfg config.DataSourceConfig,
	options ...fx.Option,
) (*app.App, func()) {
	return newTestApp(t, buildOptionsWithDBConfig(db, cfg, options...))
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
					{Pattern: "approval.*", Transports: []string{"outbox", "memory"}},
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
		// returns nil, the Ping hook short-circuits, and the redis_stream
		// constructor falls back to "no transport contributed". Tests
		// that actually exercise redis_stream provide an enabled config
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

func buildOptionsWithDBConfig(existingDB *bun.DB, cfg config.DataSourceConfig, options ...fx.Option) []fx.Option {
	// Wrap the caller-supplied bun.DB as the primary entry of a Registry so
	// the rest of the FX graph (orm.DataSources, orm.DB, schema reflection,
	// etc.) sees the same connection.
	r := database.NewRegistryFromBunDB(existingDB, cfg, nil)

	dbProvider := fx.Provide(
		fx.Annotate(
			func() *database.Registry { return r },
			fx.As(new(orm.DataSources)),
			fx.As(fx.Self()),
		),
		func() *bun.DB { return existingDB },
		func() bun.IDB { return existingDB },
		func(db *bun.DB) *sql.DB { return db.DB },
	)

	return buildOptionsWith(
		fx.Options(
			fx.Replace(&config.DataSourcesConfig{
				Map: map[string]config.DataSourceConfig{
					orm.PrimaryDataSourceName: cfg,
				},
			}),
			dbProvider,
		),
		options...,
	)
}

func buildOptionsWith(dbOption fx.Option, extra ...fx.Option) []fx.Option {
	opts := append(coreOptions(), dbOption)

	return append(opts, extra...)
}

// WithDataSourcesConfig replaces the DataSourcesConfig produced by the
// framework's config module. Equivalent to fx.Replace(&config.DataSourcesConfig{...}),
// but exposed as a helper so tests do not have to import the internal config
// package or remember the type.
func WithDataSourcesConfig(cfg *config.DataSourcesConfig) fx.Option {
	return fx.Replace(cfg)
}
