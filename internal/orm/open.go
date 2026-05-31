package orm

import (
	"database/sql"

	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm/sqlguard"
)

// Option configures how Open wraps a *sql.DB into an orm.DB.
type Option func(*options)

type options struct {
	sqlGuard *sqlguard.Config
}

// WithSQLGuard toggles the SQL-guard query hook. When enabled, dangerous
// statements (DROP / TRUNCATE / DELETE without WHERE) are blocked unless the
// query context is whitelisted. It is disabled by default.
func WithSQLGuard(enabled bool) Option {
	return func(o *options) {
		if enabled {
			o.sqlGuard = sqlguard.DefaultConfig()
		}
	}
}

// Open builds an orm.DB over an already-connected *sql.DB. The caller is
// responsible for opening the connection (database.Open) and owns its lifecycle;
// Open only layers the ORM on top — it resolves the bun dialect for kind, builds
// the (transient) *bun.DB, attaches the query-logging hook, and wraps the result
// via New. The *bun.DB lives only inside the returned orm.DB.
//
// kind selects the SQL dialect, not the connection: Open never touches the
// connection config, so deciding how to construct the *sql.DB stays entirely
// with the database package. An unsupported kind returns ErrUnsupportedDialect.
func Open(sqlDB *sql.DB, kind config.DBKind, opts ...Option) (DB, error) {
	dialect, err := DialectFor(kind)
	if err != nil {
		return nil, err
	}

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	bunDB := bun.NewDB(sqlDB, dialect, bun.WithDiscardUnknownColumns())
	addQueryHook(bunDB, logger, o.sqlGuard)

	return New(bunDB), nil
}
