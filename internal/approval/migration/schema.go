package migration

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ErrSchemaOutdated means the approval tables exist but lack a structure the
// module's correctness depends on. It is fatal at start-up by design: the
// missing structure cannot be repaired forward (the migration is
// CREATE TABLE IF NOT EXISTS and never alters an existing table), and running
// on top of it corrupts data silently rather than loudly.
var ErrSchemaOutdated = errors.New("approval schema is outdated")

// primaryKeyColumns is the primary key every approval table must declare.
// All of them are keyed by the generated XID in `id`.
var primaryKeyColumns = []string{"id"}

// kindColumnTables are the tables whose `kind` column stores an
// open-vocabulary identifier: the assignee, CC, and initiator kinds are
// registries a host extends, so the stored value is whatever a registered
// resolver describes itself with — not one of a handful of known words. The
// node table is deliberately absent; NodeKind is a closed enum.
var kindColumnTables = []string{
	"apv_flow_initiator",
	"apv_flow_node_assignee",
	"apv_flow_node_cc",
}

// minKindColumnLength is the character bound those columns must declare. The
// framework's own `department_leader` is already 17 characters, so a narrower
// column makes a built-in kind undeployable; 64 matches the bound the schema
// gives every other identifier column and leaves room for host kinds.
const minKindColumnLength = 64

// Verify asserts the approval tables carry the structure the module's write
// paths assume. It runs on every boot — including when auto-migration is on,
// because CREATE TABLE IF NOT EXISTS silently accepts a pre-existing table of
// any shape, so provisioning proves nothing about a table someone else created.
//
// The check that matters is the primary key. Approval's writes are
// compare-and-set statements whose correctness rests on a row being uniquely
// addressable: engine.ConcludeActiveNodeVisit requires its UPDATE to touch
// exactly one row and fails the transaction otherwise, and the task/instance
// transitions treat "zero rows affected" as a lost race. Against a table with
// no primary key — the shape a dump restored without constraints produces, and
// which then admits duplicate ids — those statements match a duplicate set
// instead, so approvals roll back or silently double-apply. Failing the boot
// turns that into one clear error instead of an intermittent engine bug.
func Verify(ctx context.Context, db orm.DB, kind config.DBKind) error {
	for _, table := range expectedTables {
		exists, err := sqlmigration.TableExists(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("verify table %s: %w", table, err)
		}

		if !exists {
			return outdated("missing table %s", table)
		}

		primaryKey, err := sqlmigration.LoadTablePrimaryKey(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("load primary key for %s: %w", table, err)
		}

		if len(primaryKey) == 0 {
			return outdated(
				"table %s has no primary key, so its rows are not uniquely addressable", table,
			)
		}

		if !slices.Equal(primaryKey, primaryKeyColumns) {
			return outdated(
				"table %s is keyed by %v, expected %v", table, primaryKey, primaryKeyColumns,
			)
		}
	}

	return verifyKindColumns(ctx, db, kind)
}

// verifyKindColumns asserts the open-vocabulary kind columns are wide enough
// to hold a registered kind identifier. A narrow column does not corrupt
// anything — the insert fails outright — but it fails at the moment an
// administrator deploys a flow, with a driver-level "value too long" that
// names neither the kind nor the remedy. Checking it at boot turns that into
// one clear error, and unlike the primary-key check the fix really is a
// forward ALTER, so the message says so instead of asking for a recreate.
//
// SQLite is skipped: it does not enforce a declared VARCHAR bound, so a narrow
// declaration there is cosmetic and failing the boot would be a false alarm.
func verifyKindColumns(ctx context.Context, db orm.DB, kind config.DBKind) error {
	if kind == config.SQLite {
		return nil
	}

	for _, table := range kindColumnTables {
		columns, err := sqlmigration.LoadTableColumns(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("load columns for %s: %w", table, err)
		}

		column, ok := columns["kind"]
		if !ok {
			return outdated("table %s has no kind column", table)
		}

		// Zero means the dialect declares no bound (a text column), which
		// holds any identifier.
		if column.MaxLength != 0 && column.MaxLength < minKindColumnLength {
			return fmt.Errorf(
				"%w: %s.kind is VARCHAR(%d), too narrow for a registered assignee / CC / initiator kind; "+
					"widen it to at least VARCHAR(%d)",
				ErrSchemaOutdated, table, column.MaxLength, minKindColumnLength,
			)
		}
	}

	return nil
}

// outdated builds an ErrSchemaOutdated with the recreation hint appended once,
// so every diagnostic names the same remedy.
func outdated(format string, args ...any) error {
	return fmt.Errorf(
		"%w: %s; recreate the approval tables (the migration never alters an existing schema)",
		ErrSchemaOutdated,
		fmt.Sprintf(format, args...),
	)
}
