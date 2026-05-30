package orm

import (
	"database/sql"

	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/database/sqlguard"
)

// openDataSource opens the raw *sql.DB via database.Open, resolves the bun
// dialect for cfg.Kind, builds the (transient) *bun.DB with the query-logging
// hook attached, and wraps it into an orm.DB via New. The *sql.DB is returned
// alongside the orm.DB so the registry can own the connection lifecycle
// (ping/close/version) without ever holding the *bun.DB directly — the bun.DB
// lives only inside the returned orm.DB.
//
// The dialect is resolved before the connection is opened so an unsupported
// kind fails fast without leaking a handle.
func openDataSource(cfg config.DataSourceConfig) (*sql.DB, DB, error) {
	dialect, err := DialectFor(cfg.Kind)
	if err != nil {
		return nil, nil, err
	}

	sqlDB, err := database.Open(cfg)
	if err != nil {
		return nil, nil, err
	}

	bunDB := bun.NewDB(sqlDB, dialect, bun.WithDiscardUnknownColumns())

	var guardConfig *sqlguard.Config
	if cfg.EnableSQLGuard {
		guardConfig = sqlguard.DefaultConfig()
	}

	addQueryHook(bunDB, logger, guardConfig)

	return sqlDB, New(bunDB), nil
}
