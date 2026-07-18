package store

import (
	"context"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store/migration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Module provides the durable schedule store: the handler registry, the
// claim/execute engine, the management surface, and the sys/cron resources.
// Everything is inert until vef.cron.store.enabled turns the store on — the
// manager then degrades to ErrStoreDisabled and the resources mount no
// operations.
var Module = fx.Module(
	"vef:cron:store",

	fx.Provide(
		fx.Annotate(NewRegistry, fx.ParamTags(`group:"vef:cron:job_handlers"`)),
		fx.Private,
	),
	fx.Provide(NewRunEventPublisher, fx.Private),
	fx.Provide(newEngine, fx.Private),
	fx.Provide(newScheduleManager),
	fx.Provide(
		fx.Annotate(NewScheduleResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewRunResource, fx.ResultTags(`group:"vef:api:resources"`)),
	),

	fx.Invoke(registerLifecycle),
)

func newEngine(db orm.DB, cfg *config.CronConfig, registry *Registry, publisher *RunEventPublisher) *Engine {
	return NewEngine(db, &cfg.Store, registry, publisher)
}

func newScheduleManager(db orm.DB, cfg *config.CronConfig, registry *Registry, engine *Engine) cron.ScheduleManager {
	return NewScheduleManager(db, cfg.Store.Enabled, registry, engine)
}

// registerLifecycle wires the store into application start-up: migrate,
// seed handler-shipped defaults, then launch the engine. With the store
// disabled it only points out registered handlers that will never fire.
func registerLifecycle(
	lc fx.Lifecycle,
	cfg *config.CronConfig,
	dataSources *config.DataSourcesConfig,
	db orm.DB,
	registry *Registry,
	engine *Engine,
	manager cron.ScheduleManager,
) {
	if !cfg.Store.Enabled {
		if !registry.IsEmpty() {
			logger.Warnf(
				"%d cron job handler(s) registered while vef.cron.store.enabled=false; durable schedules will not fire",
				len(registry.Names()),
			)
		}

		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if cfg.Store.AutoMigrate {
				if err := migration.Migrate(ctx, db, dataSources.Primary().Kind); err != nil {
					return err
				}
			}

			if err := SeedDefaultSchedules(ctx, manager, registry); err != nil {
				return err
			}

			engine.Start()

			return nil
		},
		OnStop: func(context.Context) error {
			engine.Stop()

			return nil
		},
	})
}
