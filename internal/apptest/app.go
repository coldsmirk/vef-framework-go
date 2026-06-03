package apptest

import (
	"database/sql"
	"testing"

	"github.com/uptrace/bun"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/datasource"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/bootmodules"
	iconfig "github.com/coldsmirk/vef-framework-go/internal/config"
	idatasource "github.com/coldsmirk/vef-framework-go/internal/datasource"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
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

	// Align the test app's JWT signing secret with Suite.GenerateToken so tokens
	// minted by tests verify against the running app.
	if key == "vef.security" {
		if sc, ok := target.(*config.SecurityConfig); ok {
			sc.Secret = testJWTSecret
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
// instead of creating a new connection via datasource.Module.
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

func coreOptions(dataSourceOption fx.Option) []fx.Option {
	opts := []fx.Option{
		fx.NopLogger,
		fx.Replace(
			fx.Annotate(&NopConfig{}, fx.As(new(config.Config))),
			&config.AppConfig{
				Name:      testAppName,
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
		dataSourceOption,
	}

	// Business modules come from bootmodules.Core() — the same canonical list
	// the production boot sequence (vef.Run) uses — so this harness cannot
	// drift from the real graph. The default test RedisConfig has Enabled=false,
	// so the redis_stream transport falls back to "no transport contributed";
	// tests that exercise it supply an enabled config (see testx.NewRedisContainer).
	return append(opts, bootmodules.Core()...)
}

func buildOptions(options ...fx.Option) []fx.Option {
	return buildOptionsWith(idatasource.Module, options...)
}

func buildOptionsWithDBConfig(existingDB *bun.DB, cfg config.DataSourceConfig, options ...fx.Option) []fx.Option {
	// Wrap the caller-supplied bun.DB as the primary entry of a Registry so the
	// rest of the FX graph (datasource.Registry, orm.DB, schema reflection, etc.)
	// sees the same connection. apptest does the bun→orm.DB conversion here so the
	// datasource package itself stays unaware of bun.
	dsr := idatasource.NewFromDB(existingDB.DB, orm.New(existingDB), cfg, nil)

	dbProvider := fx.Provide(
		func() datasource.Registry { return dsr },
		func() *sql.DB { return existingDB.DB },
		func() orm.DB { return dsr.Primary() },
	)

	return buildOptionsWith(
		fx.Options(
			fx.Replace(&config.DataSourcesConfig{
				Map: map[string]config.DataSourceConfig{
					datasource.PrimaryName: cfg,
				},
			}),
			dbProvider,
		),
		options...,
	)
}

func buildOptionsWith(dataSourceOption fx.Option, extra ...fx.Option) []fx.Option {
	opts := coreOptions(dataSourceOption)

	return append(opts, extra...)
}

// WithDataSourcesConfig replaces the DataSourcesConfig produced by the
// framework's config module. Equivalent to fx.Replace(&config.DataSourcesConfig{...}),
// but exposed as a helper so tests do not have to import the internal config
// package or remember the type.
func WithDataSourcesConfig(cfg *config.DataSourcesConfig) fx.Option {
	return fx.Replace(cfg)
}
