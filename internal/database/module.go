package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

var (
	logger = logx.Named("database")
	Module = fx.Module(
		"vef:database",
		fx.Provide(
			fx.Annotate(
				provideRegistry,
				fx.As(new(orm.DataSources)),
				fx.As(fx.Self()),
			),
			providePrimaryBunDB,
			providePrimaryIBunDB,
			providePrimarySQLDB,
		),
		fx.Invoke(seedStaticDataSources),
		fx.Invoke(runDataSourceProviders),
	)
)

func provideRegistry(lc fx.Lifecycle, dataSources *config.DataSourcesConfig) (*Registry, error) {
	r, err := NewRegistry(context.Background(), dataSources.Primary(), logger)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			provider, ok := registry.provider(dataSources.Primary().Kind)
			if !ok {
				return nil
			}

			if err := logDBVersion(provider, r.PrimaryBunDB(), logger); err != nil {
				return err
			}

			logger.Infof("Database client started successfully: %s", provider.Kind())

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing data source registry...")

			return r.Shutdown(ctx)
		},
	})

	return r, nil
}

func providePrimaryBunDB(r *Registry) *bun.DB { return r.PrimaryBunDB() }

func providePrimaryIBunDB(r *Registry) bun.IDB { return r.PrimaryBunDB() }

func providePrimarySQLDB(db *bun.DB) *sql.DB { return db.DB }

// seedStaticDataSources registers every TOML-declared data source besides
// primary. It runs as an FX lifecycle StartHook because Register issues a
// Ping that benefits from the OnStart context (so a misconfigured source
// fails the boot rather than the provide phase).
func seedStaticDataSources(lc fx.Lifecycle, r *Registry, cfg *config.DataSourcesConfig) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for name, dsCfg := range cfg.Map {
				if name == orm.PrimaryDataSourceName {
					continue
				}

				if _, err := r.Register(ctx, name, dsCfg); err != nil {
					return fmt.Errorf("register static data source %q: %w", name, err)
				}

				logger.Infof("Registered static data source: %s (%s)", name, dsCfg.Kind)
			}

			return nil
		},
	})
}

// DataSourceProviderParams collects every orm.DataSourceProvider declared
// through vef.ProvideDataSourceProvider. The group is optional so
// applications with no providers still satisfy the invoke signature.
type DataSourceProviderParams struct {
	fx.In

	Providers []orm.DataSourceProvider `group:"vef:orm:data_source_providers"`
}

// runDataSourceProviders calls Load on every registered DataSourceProvider
// during OnStart and registers the returned specs. Provider order is
// undefined; name collisions (with TOML or another provider) fail boot.
func runDataSourceProviders(lc fx.Lifecycle, r *Registry, p DataSourceProviderParams) {
	if len(p.Providers) == 0 {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for _, provider := range p.Providers {
				specs, err := provider.Load(ctx)
				if err != nil {
					return fmt.Errorf("data source provider %q: %w", provider.Name(), err)
				}

				for _, spec := range specs {
					if _, err := r.Register(ctx, spec.Name, spec.Cfg); err != nil {
						return fmt.Errorf("provider %q: register data source %q: %w",
							provider.Name(), spec.Name, err)
					}

					logger.Infof("Registered provided data source: %s (%s) via %s",
						spec.Name, spec.Cfg.Kind, provider.Name())
				}
			}

			return nil
		},
	})
}
