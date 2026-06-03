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

// expectedTables lists all tables the storage module requires.
var expectedTables = []string{
	"sys_storage_upload_claim",
	"sys_storage_upload_part",
	"sys_storage_pending_delete",
}

// Migrate runs the storage module's DDL migration for the given
// database kind. Idempotent: no-op when every expected table is
// already present.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{
		Label:          "storage",
		Kind:           kind,
		Scripts:        scripts,
		ExpectedTables: expectedTables,
	})
}
