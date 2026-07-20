package sqlmigration

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// TableExists reports whether the named table exists in the connection's
// active schema.
func TableExists(ctx context.Context, db orm.DB, kind config.DBKind, table string) (bool, error) {
	count, err := CountTables(ctx, db, kind, []string{table})

	return count > 0, err
}

// CountTables counts how many of the named tables exist in the connection's
// active schema — current_schema() on Postgres, the connected database on
// MySQL, the main database on SQLite. Every migration probe shares this one
// dialect vocabulary so presence semantics cannot drift between modules.
func CountTables(ctx context.Context, db orm.DB, kind config.DBKind, tables []string) (int, error) {
	query := ""

	switch kind {
	case config.Postgres:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ?"
	case config.MySQL:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ?"
	case config.SQLite:
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ?"
	default:
		return 0, fmt.Errorf("%w %q", ErrUnsupportedDBKind, kind)
	}

	// Table names are bound via bun.Tuple so there is no injection risk.
	var count int
	if err := db.NewRaw(query, bun.Tuple(tables)).Scan(ctx, &count); err != nil {
		return 0, err
	}

	return count, nil
}
