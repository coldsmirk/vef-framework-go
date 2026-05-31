package datasource

import (
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/datasource"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

var logger = logx.Named("datasource")

// RegistryModule constructs the data source Registry from configuration, seeds
// the non-primary static (TOML) and provider-supplied sources during startup,
// and exposes the primary connection's raw *sql.DB (consumed by the schema
// reflection service). It is the production provider of datasource.Registry;
// test harnesses that want to share an existing connection supply their own via
// NewFromDB instead. The agnostic Module then derives the primary orm.DB from
// whichever Registry is in the container.
var RegistryModule = fx.Module(
	"vef:datasource:registry",
	fx.Provide(provideRegistry),
)

// Module derives the primary orm.DB from whichever datasource.Registry is in the
// container. Most callers inject orm.DB directly and get the primary source;
// cross-source access goes through datasource.Registry. Keeping it separate from
// RegistryModule lets test harnesses supply their own Registry and still reuse
// this derivation.
var Module = fx.Module(
	"vef:datasource",
	fx.Provide(providePrimary),
)

func providePrimary(r datasource.Registry) orm.DB {
	return r.Primary()
}

type registryOut struct {
	fx.Out

	Sources datasource.Registry
	RawDB   *sql.DB
}

// ProviderParams collects every datasource.Provider declared through
// vef.ProvideDataSourceProvider. The group is optional so applications with no
// providers still satisfy the signature.
type ProviderParams struct {
	fx.In

	Providers []datasource.Provider `group:"vef:datasource:providers"`
}

// provideRegistry opens the primary source (fail-fast in the provide phase) and
// registers two lifecycle hooks: a Shutdown hook that drains and closes every
// source on stop, and a start-only hook that logs the primary version, seeds the
// non-primary static sources, then runs the data source providers.
//
// The two are kept separate on purpose. Fx only runs a hook's OnStop when that
// same hook's OnStart succeeded (see fx internal/lifecycle: Start increments
// numStarted only after OnStart returns nil; Stop runs OnStop for the started
// prefix only). Folding Shutdown together with the fallible seed/provider work
// would mean a mid-start failure skips the drain entirely, leaking the primary
// and any partially-registered sources. The Shutdown hook carries no OnStart, so
// it always counts as started and its OnStop is guaranteed once Start reaches it,
// regardless of which later step fails.
func provideRegistry(lc fx.Lifecycle, cfg *config.DataSourcesConfig, p ProviderParams) (registryOut, error) {
	r, err := newRegistry(context.Background(), cfg.Primary(), logger)
	if err != nil {
		return registryOut{}, err
	}

	primaryKind := cfg.Primary().Kind

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing data source registry...")

			return r.Shutdown(ctx)
		},
	})

	lc.Append(fx.StartHook(func(ctx context.Context) error {
		if err := database.LogVersion(ctx, primaryKind, r.PrimaryRawDB(), logger); err != nil {
			return err
		}

		if err := seedStatic(ctx, r, cfg); err != nil {
			return err
		}

		return runProviders(ctx, r, p.Providers)
	}))

	return registryOut{Sources: r, RawDB: r.PrimaryRawDB()}, nil
}

// seedStatic registers every TOML-declared data source besides primary. It runs
// during OnStart so the Register Ping benefits from the start context and a
// misconfigured source fails the boot rather than the provide phase.
func seedStatic(ctx context.Context, r *registry, cfg *config.DataSourcesConfig) error {
	for name, dsCfg := range cfg.Map {
		if name == datasource.PrimaryName {
			continue
		}

		if err := registerSource(ctx, r, name, dsCfg, "static"); err != nil {
			return err
		}
	}

	return nil
}

// runProviders calls Load on every registered Provider and registers the
// returned specs. Provider order is undefined; a name collision (with TOML or
// another provider) fails boot.
func runProviders(ctx context.Context, r *registry, providers []datasource.Provider) error {
	for _, provider := range providers {
		specs, err := provider.Load(ctx)
		if err != nil {
			return fmt.Errorf("data source provider %q: %w", provider.Name(), err)
		}

		for _, spec := range specs {
			if err := registerSource(ctx, r, spec.Name, spec.Cfg, provider.Name()); err != nil {
				return err
			}
		}
	}

	return nil
}

// registerSource registers a single non-primary data source and logs it, tagging
// the error and log line with origin (e.g. "static" or a provider name) so a
// misconfigured source is easy to trace at boot.
func registerSource(ctx context.Context, r *registry, name string, cfg config.DataSourceConfig, origin string) error {
	if _, err := r.Register(ctx, name, cfg); err != nil {
		return fmt.Errorf("register %s data source %q: %w", origin, name, err)
	}

	logger.Infof("Registered %s data source: %s (%s)", origin, name, cfg.Kind)

	return nil
}
