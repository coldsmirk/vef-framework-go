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

// expectedTables lists all tables the integration module requires.
var expectedTables = []string{
	"itg_contract",
	"itg_system",
	"itg_adapter",
	"itg_route",
	"itg_code_map",
	"itg_invocation_log",
}

// Migrate runs the integration module's DDL migration for the given database
// kind. The migration is forward-only: CREATE TABLE IF NOT EXISTS statements
// guarded by a presence probe, provisioning missing tables but never altering
// existing ones.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{
		Label:          "integration",
		Kind:           kind,
		Scripts:        scripts,
		ExpectedTables: expectedTables,
	})
}
