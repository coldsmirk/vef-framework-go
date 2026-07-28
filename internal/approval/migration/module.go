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

// autoMigrate provisions the approval schema when configured to, then verifies
// it either way. AutoMigrate decides whether missing tables get created — not
// whether the resulting schema is trusted: CREATE TABLE IF NOT EXISTS accepts
// any pre-existing table, so a database provisioned by something other than
// this migration (a restored dump, a hand-built schema) would otherwise reach
// the write paths unchecked.
func autoMigrate(ctx context.Context, cfg *config.ApprovalConfig, db orm.DB, dataSources *config.DataSourcesConfig) error {
	kind := dataSources.Primary().Kind

	if cfg.AutoMigrate {
		if err := Migrate(ctx, db, kind); err != nil {
			return err
		}
	}

	return Verify(ctx, db, kind)
}
