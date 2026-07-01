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

// Migrate runs the approval module's DDL migration for the given
// database kind. Obsolete tables from earlier revisions are dropped
// before the schema probe so upgrades stay clean.
//
// The migration is forward-only: each script is a set of CREATE TABLE IF NOT
// EXISTS statements guarded by a presence probe (needsMigration), so it
// provisions missing tables on a fresh or partially-migrated database but never
// alters an existing table. In-place column changes — an added column (e.g. the
// v0.33.0 action-log name snapshots added_assignee_names / removed_assignee_names
// / cc_user_names), a type/constraint change such as the pass_ratio rescale, or a
// dropped column — therefore take effect only on a freshly created database; an
// existing deployment that must adopt them has to be recreated. This is a hard
// requirement, not graceful degradation: once the code writes or reads an added
// column, an un-recreated older database errors on that column. The approval
// module is pre-1.0 and assumes recreation over in-place schema evolution; table
// removals are the one exception, handled explicitly through dropObsoleteTables.
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
