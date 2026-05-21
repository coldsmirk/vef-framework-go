package outbox

import (
	"context"
	"embed"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

//go:embed scripts/*.sql
var scripts embed.FS

// Migrate runs the outbox transport's DDL migration for the supplied
// database kind. The migration is idempotent and a no-op when every
// expected table already exists.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{
		Label:          "event outbox",
		Kind:           kind,
		Scripts:        scripts,
		ExpectedTables: []string{"sys_event_outbox"},
	})
}
