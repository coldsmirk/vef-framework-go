package migration

import (
	"context"
	"embed"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

//go:embed scripts/*.sql
var scripts embed.FS

// expectedTables lists all tables the approval module requires.
var expectedTables = []string{
	"apv_flow_category",
	"apv_flow",
	"apv_flow_initiator",
	"apv_flow_version",
	"apv_flow_node",
	"apv_flow_node_assignee",
	"apv_flow_node_cc",
	"apv_flow_edge",
	"apv_flow_form_field",
	"apv_instance",
	"apv_task",
	"apv_action_log",
	"apv_cc_record",
	"apv_delegation",
	"apv_form_snapshot",
	"apv_urge_record",
}

// obsoleteTables lists tables that earlier versions of the approval
// module created but no longer uses. Migrate drops them unconditionally
// so upgrades clean up after the framework-level outbox replaced
// per-module bookkeeping.
var obsoleteTables = []string{
	"apv_event_outbox",
	"apv_parallel_record",
}

// Migrate runs the approval module's DDL migration for the given
// database kind. Obsolete tables from earlier revisions are dropped
// before the schema probe so upgrades stay clean.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{
		Label:          "approval",
		Kind:           kind,
		Scripts:        scripts,
		ExpectedTables: expectedTables,
		Pre:            []func(ctx context.Context, db orm.DB) error{dropObsoleteTables},
	})
}

// dropObsoleteTables removes tables retired in past schema revisions.
// IF EXISTS keeps the statement idempotent across both fresh and
// upgraded databases.
func dropObsoleteTables(ctx context.Context, db orm.DB) error {
	for _, table := range obsoleteTables {
		if _, err := db.NewRaw("DROP TABLE IF EXISTS " + table).Exec(ctx); err != nil {
			return fmt.Errorf("drop %s: %w", table, err)
		}
	}

	return nil
}

// GetMigrationSQL returns the approval DDL script for the given dialect.
// Retained for tests and tooling that needs the raw SQL.
func GetMigrationSQL(kind config.DBKind) (string, error) {
	return sqlmigration.LoadScript(scripts, kind)
}
