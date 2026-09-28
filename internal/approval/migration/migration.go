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
	"apv_instance",
	"apv_business_projection",
	"apv_node_visit",
	"apv_task",
	"apv_action_log",
	"apv_cc_record",
	"apv_delegation",
	"apv_form_snapshot",
	"apv_urge_record",
	"apv_form_table",
	"apv_form_table_column",
}

// obsoleteTables lists tables that earlier versions of the approval
// module created but no longer uses. Migrate drops them unconditionally:
// apv_event_outbox / apv_parallel_record were replaced by the
// framework-level outbox, and apv_flow_form_field was dead DDL — form
// schemas live in apv_flow_version.form_schema (JSONB) and no code ever
// read or wrote the table.
var obsoleteTables = []string{
	"apv_event_outbox",
	"apv_parallel_record",
	"apv_flow_form_field",
}

// addedColumns are the approval columns introduced after their table first
// shipped: Migrate adds them to an existing table and Verify requires them.
var addedColumns = []sqlmigration.AddedColumn{
	{Table: "apv_flow_node", Name: "pass_count", Definition: "INTEGER NOT NULL DEFAULT 0", Comment: "Pass Count"},
}

// Migrate runs the approval module's DDL migration for the given
// database kind. Obsolete tables from earlier revisions are dropped
// before the schema probe so upgrades stay clean.
//
// The scripts' CREATE TABLE IF NOT EXISTS statements provision missing tables,
// and addedColumns brings an existing table up to date. Nothing else about an
// existing table is migrated: Verify reports any other drift as
// ErrSchemaOutdated.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	return sqlmigration.Run(ctx, db, sqlmigration.Plan{
		Label:          "approval",
		Kind:           kind,
		Scripts:        scripts,
		ExpectedTables: expectedTables,
		Pre:            []func(ctx context.Context, db orm.DB) error{dropObsoleteTables},
		AddedColumns:   addedColumns,
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
