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

	fx.Invoke(registerMigration),
)

// registerMigration schedules the approval schema work on application start,
// from a lifecycle hook because the fx graph has no context.Context to hand a
// constructor (see the CLAUDE.md gotcha).
//
// Unlike its siblings there is no early return when AutoMigrate is off: the hook
// provisions the schema only when configured to, but verifies it either way.
// AutoMigrate decides whether missing tables get created — not whether the
// resulting schema is trusted, since CREATE TABLE IF NOT EXISTS accepts any
// pre-existing table, so a database provisioned by something else (a restored
// dump, a hand-built schema) would otherwise reach the write paths unchecked.
func registerMigration(
	lc fx.Lifecycle,
	cfg *config.ApprovalConfig,
	db orm.DB,
	dataSources *config.DataSourcesConfig,
) {
	lc.Append(fx.StartHook(func(ctx context.Context) error {
		kind := dataSources.Primary().Kind

		if cfg.AutoMigrate {
			if err := Migrate(ctx, db, kind); err != nil {
				return err
			}
		}

		return Verify(ctx, db, kind)
	}))
}
