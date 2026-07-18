package migration

import (
	"context"
	"embed"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

//go:embed scripts/*.sql
var scripts embed.FS

// expectedTables lists all tables the cron store requires.
var expectedTables = []string{
	"crn_schedule",
	"crn_run",
}

// Migrate runs the cron store's DDL migration for the given database kind.
// The migration is forward-only: CREATE TABLE IF NOT EXISTS statements
// guarded by a presence probe, provisioning missing tables but never
// altering existing ones.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{
		Label:          "cron store",
		Kind:           kind,
		Scripts:        scripts,
		ExpectedTables: expectedTables,
	})
}
