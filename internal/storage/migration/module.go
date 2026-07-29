package migration

import (
	"context"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Module provides automatic database migration for the storage module.
var Module = fx.Module(
	"vef:storage:migration",

	fx.Invoke(autoMigrate),
)

// autoMigrate registers a startup hook that applies the storage module's
// DDL when StorageConfig.AutoMigrate is true. It runs from a lifecycle hook
// because the fx graph has no context.Context to hand a constructor at all
// (see the CLAUDE.md gotcha); the hook receives the one fx manages for start-up.
func autoMigrate(lc fx.Lifecycle, cfg *config.StorageConfig, db orm.DB, dataSources *config.DataSourcesConfig) {
	if !cfg.AutoMigrate {
		return
	}

	kind := dataSources.Primary().Kind

	lc.Append(fx.StartHook(func(ctx context.Context) error {
		return Migrate(ctx, db, kind)
	}))
}
