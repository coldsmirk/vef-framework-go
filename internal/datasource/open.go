package datasource

import (
	"database/sql"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// open builds a single data source by composing the two lower layers: database
// connects a *sql.DB from cfg, then orm layers the ORM on top of that *sql.DB.
// database owns "how to connect from a DataSourceConfig"; orm owns "how to wrap
// a *sql.DB into an orm.DB". datasource is the only package that knows both, so
// the composition lives here and nowhere else.
//
// The *sql.DB is returned alongside the orm.DB so the registry can own the
// connection lifecycle (ping/close) directly, without ever holding the *bun.DB
// that lives inside the orm.DB. On any post-connect failure the *sql.DB is
// closed so no handle leaks.
func open(cfg config.DataSourceConfig) (*sql.DB, orm.DB, error) {
	rawDB, err := database.Open(cfg)
	if err != nil {
		return nil, nil, err
	}

	ormDB, err := orm.Open(rawDB, cfg.Kind, orm.WithSQLGuard(cfg.EnableSQLGuard))
	if err != nil {
		_ = rawDB.Close()

		return nil, nil, err
	}

	return rawDB, ormDB, nil
}
