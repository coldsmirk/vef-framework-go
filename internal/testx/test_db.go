package testx

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// NewTestDB creates a lightweight SQLite in-memory orm.DB for unit tests.
// The database connection is automatically closed via t.Cleanup.
func NewTestDB(t *testing.T) orm.DB {
	t.Helper()

	bunDB, err := database.New(&config.DataSourceConfig{Kind: config.SQLite})
	require.NoError(t, err, "SQLite connection should succeed")

	t.Cleanup(func() {
		require.NoError(t, bunDB.Close(), "Database should close without error")
	})

	return orm.New(bunDB)
}
