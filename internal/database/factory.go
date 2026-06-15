package database

import (
	"database/sql"

	"github.com/coldsmirk/vef-framework-go/config"
)

// Open establishes a connection to the configured data source and returns the
// raw *sql.DB with a dialect-appropriate connection pool applied (see
// poolConfigFor — file-mode SQLite gets a small pool, server dialects the large
// one). Building an ORM handle on top of it (bun.DB, dialect, query hooks) is
// the caller's concern — see internal/orm.
func Open(cfg config.DataSourceConfig) (*sql.DB, error) {
	provider, exists := registry.lookup(cfg.Kind)
	if !exists {
		return nil, newUnsupportedDBKindError(cfg.Kind)
	}

	db, err := provider.Connect(&cfg)
	if err != nil {
		return nil, err
	}

	poolConfigFor(cfg).ApplyToDB(db)

	return db, nil
}
