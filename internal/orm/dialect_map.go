package orm

import (
	"fmt"

	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/config"
)

// DialectFor maps a config.DBKind to the bun schema.Dialect used to build the
// *bun.DB for that kind. It is the orm-side counterpart to the connector
// selection performed in internal/database: both branch on the same DBKind and
// MUST stay in agreement (a kind supported here must also be openable there).
// The dialect_map test asserts that invariant against database.SupportsKind.
func DialectFor(kind config.DBKind) (schema.Dialect, error) {
	switch kind {
	case config.Postgres:
		return pgdialect.New(), nil
	case config.MySQL:
		return mysqldialect.New(), nil
	case config.SQLite:
		return sqlitedialect.New(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDialect, kind)
	}
}
