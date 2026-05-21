package inbox

import (
	"context"
	"embed"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

//go:embed scripts/*.sql
var scripts embed.FS

// Migrate runs the inbox DDL migration. Idempotent: no-op when the
// expected tables already exist.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{
		Label:          "event inbox",
		Kind:           kind,
		Scripts:        scripts,
		ExpectedTables: []string{"sys_event_inbox"},
	})
}
