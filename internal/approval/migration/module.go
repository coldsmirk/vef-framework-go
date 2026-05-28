package migration

import (
	"context"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Module provides automatic database migration for the approval module.
var Module = fx.Module(
	"vef:approval:migration",

	fx.Invoke(autoMigrate),
)

func autoMigrate(ctx context.Context, cfg *config.ApprovalConfig, db orm.DB, dataSources *config.DataSourcesConfig) error {
	if !cfg.AutoMigrate {
		return nil
	}

	return Migrate(ctx, db, dataSources.Primary().Kind)
}
