package orm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/sqlguard"
)

// TestSQLGuard tests SQL guard integration with raw SQL through the orm query
// hook. The GoSQLX parser doesn't support double-quoted identifiers (bun's
// default), so the tests use NewRaw with unquoted SQL. Each subtest opens its
// own data source and closes it via t.Cleanup so the shared in-memory SQLite
// database is fresh between subtests.
func TestSQLGuard(t *testing.T) {
	ctx := context.Background()

	newGuardedDB := func(t *testing.T, enableGuard bool) DB {
		t.Helper()

		sqlDB, db, err := openDataSource(config.DataSourceConfig{
			Kind:           config.SQLite,
			EnableSQLGuard: enableGuard,
		})
		require.NoError(t, err, "openDataSource should succeed")

		t.Cleanup(func() { _ = sqlDB.Close() })

		_, err = db.NewRaw("CREATE TABLE IF NOT EXISTS test_guard (id INTEGER PRIMARY KEY, name TEXT)").Exec(ctx)
		require.NoError(t, err, "create test table should succeed")

		return db
	}

	t.Run("DropStatementBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("DROP TABLE test_guard").Exec(ctx)
		require.Error(t, err, "DROP should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "blocked query cancels the context")

		var count int
		require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(ctx, &count),
			"table should still exist after blocked DROP")
	})

	t.Run("TruncateStatementBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("INSERT INTO test_guard (name) VALUES ('test')").Exec(ctx)
		require.NoError(t, err, "insert should succeed")

		_, err = db.NewRaw("TRUNCATE TABLE test_guard").Exec(ctx)
		require.Error(t, err, "TRUNCATE should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "blocked query cancels the context")

		var count int
		require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(ctx, &count),
			"count query should succeed after blocked TRUNCATE")
		require.Equal(t, 1, count, "data should still exist after blocked TRUNCATE")
	})

	t.Run("DeleteWithoutWhereBlocked", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("INSERT INTO test_guard (name) VALUES ('test')").Exec(ctx)
		require.NoError(t, err, "insert should succeed")

		_, err = db.NewRaw("DELETE FROM test_guard").Exec(ctx)
		require.Error(t, err, "DELETE without WHERE should be blocked by SQL guard")
		require.ErrorIs(t, err, context.Canceled, "blocked query cancels the context")

		var count int
		require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(ctx, &count),
			"count query should succeed after blocked DELETE")
		require.Equal(t, 1, count, "data should still exist after blocked DELETE without WHERE")
	})

	t.Run("DeleteWithWhereAllowed", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("DELETE FROM test_guard WHERE name = 'nonexistent'").Exec(ctx)
		require.NoError(t, err, "DELETE with WHERE should be allowed")
	})

	t.Run("SelectAllowed", func(t *testing.T) {
		db := newGuardedDB(t, true)

		var result []struct {
			ID   int
			Name string
		}

		require.NoError(t, db.NewRaw("SELECT id, name FROM test_guard").Scan(ctx, &result),
			"SELECT should be allowed")
	})

	t.Run("WhitelistBypassesGuard", func(t *testing.T) {
		db := newGuardedDB(t, true)

		_, err := db.NewRaw("DROP TABLE test_guard").Exec(ctx)
		require.Error(t, err, "DROP should be blocked without whitelist")

		_, err = db.NewRaw("DROP TABLE test_guard").Exec(sqlguard.WithWhitelist(ctx))
		require.NoError(t, err, "DROP should work with whitelisted context")
	})

	t.Run("DisabledGuardAllowsDangerousSQL", func(t *testing.T) {
		db := newGuardedDB(t, false)

		_, err := db.NewRaw("DROP TABLE test_guard").Exec(ctx)
		require.NoError(t, err, "DROP should work when SQL guard is disabled")
	})
}
