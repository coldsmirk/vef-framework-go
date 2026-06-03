package database

import (
	"database/sql"

	"github.com/coldsmirk/vef-framework-go/config"
)

// Open establishes a connection to the configured data source and returns the
// raw *sql.DB with the default connection pool applied. Building an ORM handle on
// top of it (bun.DB, dialect, query hooks) is the caller's concern — see
// internal/orm.
func Open(cfg config.DataSourceConfig) (*sql.DB, error) {
	provider, exists := registry.lookup(cfg.Kind)
	if !exists {
		return nil, newUnsupportedDBKindError(cfg.Kind)
	}

	db, err := provider.Connect(&cfg)
	if err != nil || db == nil {
		return nil, err
	}

	NewDefaultConnectionPoolConfig().ApplyToDB(db)

	return db, nil
}
