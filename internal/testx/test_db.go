package testx

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// openBunDB opens a *sql.DB for cfg via database.Open and builds a *bun.DB with
// the matching dialect. It mirrors orm.openDataSource for tests that need the
// raw *bun.DB handle. The caller owns closing the returned *sql.DB.
func openBunDB(t *testing.T, cfg config.DataSourceConfig) (*sql.DB, *bun.DB) {
	t.Helper()

	sqlDB, err := database.Open(cfg)
	require.NoError(t, err, "database.Open should succeed for %s", cfg.Kind)

	dialect, err := orm.DialectFor(cfg.Kind)
	require.NoError(t, err, "orm.DialectFor should succeed for %s", cfg.Kind)

	return sqlDB, bun.NewDB(sqlDB, dialect, bun.WithDiscardUnknownColumns())
}

// NewTestDB creates a lightweight SQLite in-memory orm.DB for unit tests.
// The database connection is automatically closed via t.Cleanup.
func NewTestDB(t *testing.T) orm.DB {
	t.Helper()

	sqlDB, bunDB := openBunDB(t, config.DataSourceConfig{Kind: config.SQLite})

	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close(), "Database should close without error")
	})

	return orm.New(bunDB)
}
