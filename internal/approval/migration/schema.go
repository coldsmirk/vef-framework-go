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
