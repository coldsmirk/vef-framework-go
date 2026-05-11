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
// DDL when StorageConfig.AutoMigrate is true. Registering via fx.Lifecycle
// is required because the fx graph does not provide a long-lived
// context.Context directly; the hook receives the lifecycle context that
// fx manages for startup.
func autoMigrate(lc fx.Lifecycle, cfg *config.StorageConfig, db orm.DB, ds *config.DataSourceConfig) {
	if !cfg.AutoMigrate {
		return
	}

	lc.Append(fx.StartHook(func(ctx context.Context) error {
		return Migrate(ctx, db, ds.Kind)
	}))
}
