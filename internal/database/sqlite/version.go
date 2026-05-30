package sqlite

import (
	"context"
	"database/sql"
)

func queryVersion(ctx context.Context, db *sql.DB) (string, error) {
	var version string

	return version, db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
}
