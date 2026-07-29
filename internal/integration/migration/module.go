package migration

import (
	"context"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Module provides automatic database migration for the integration module.
var Module = fx.Module(
	"vef:integration:migration",

	fx.Invoke(autoMigrate),
)

// autoMigrate registers a start-up hook that applies the integration module's
// DDL when IntegrationConfig.AutoMigrate is true. It runs from a lifecycle hook
// because the fx graph has no context.Context to hand a constructor (see the
// CLAUDE.md gotcha).
func autoMigrate(lc fx.Lifecycle, cfg *config.IntegrationConfig, db orm.DB, dataSources *config.DataSourcesConfig) {
	if !cfg.AutoMigrate {
		return
	}

	kind := dataSources.Primary().Kind

	lc.Append(fx.StartHook(func(ctx context.Context) error {
		return Migrate(ctx, db, kind)
	}))
}
