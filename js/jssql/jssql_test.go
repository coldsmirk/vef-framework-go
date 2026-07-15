package jssql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jssql"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// newSQLRuntime builds a bare runtime with the sql library enabled over a
// seeded in-memory database.
func newSQLRuntime(t *testing.T, opts ...jssql.Option) (*js.Runtime, orm.DB) {
	t.Helper()

	db := testx.NewTestDB(t)

	_, err := db.NewRaw(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, age INTEGER NOT NULL)`).Exec(t.Context())
	require.NoError(t, err, "Schema should be created")

	_, err = db.NewRaw(`INSERT INTO users (name, age) VALUES ('alice', 30), ('bob', 17), ('carol', 25)`).Exec(t.Context())
	require.NoError(t, err, "Seed rows should be inserted")

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(jssql.New(db, config.SQLite, opts...)))
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(js.EnableLibs(jssql.Name))
	require.NoError(t, err, "NewRuntime should succeed")

	return rt, db
}

// TestQuery tests parameterized read access.
func TestQuery(t *testing.T) {
	t.Run("RowsAsObjects", func(t *testing.T) {
		rt, _ := newSQLRuntime(t)

		result, err := rt.RunString(t.Context(), `
			const rows = sql.query('SELECT name, age FROM users WHERE age > ? ORDER BY age', 18);
			rows.map(r => r.name + ':' + r.age).join(',')
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "carol:25,alice:30", result.String(), "Rows should be filtered, ordered, and navigable as objects")
	})

	t.Run("MultipleParameters", func(t *testing.T) {
		rt, _ := newSQLRuntime(t)

		result, err := rt.RunString(t.Context(), `
			sql.query('SELECT COUNT(*) AS n FROM users WHERE age > ? AND name <> ?', 10, 'bob')[0].n
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(2), result.ToInteger(), "Both parameters should bind in order")
	})

	t.Run("CommonTableExpression", func(t *testing.T) {
		rt, _ := newSQLRuntime(t)

		result, err := rt.RunString(t.Context(), `
			sql.query('WITH adults AS (SELECT * FROM users WHERE age >= 18) SELECT COUNT(*) AS n FROM adults')[0].n
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(2), result.ToInteger(), "WITH statements should be accepted as read-only")
	})

	t.Run("MutationRejected", func(t *testing.T) {
		rt, db := newSQLRuntime(t)

		_, err := rt.RunString(t.Context(), `sql.query('DELETE FROM users')`)
		require.Error(t, err, "A mutating statement should be rejected by query")
		assert.Contains(t, err.Error(), "not read-only", "Error should carry the read-only reason")

		var count int

		require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users`).Scan(t.Context(), &count), "Count query should succeed")
		assert.Equal(t, 3, count, "Data should be untouched")
	})

	t.Run("StackedStatementRejected", func(t *testing.T) {
		rt, db := newSQLRuntime(t)

		_, err := rt.RunString(t.Context(), `sql.query('SELECT 1; DELETE FROM users')`)
		require.Error(t, err, "A stacked mutating statement should be rejected by query")

		var count int

		require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users`).Scan(t.Context(), &count), "Count query should succeed")
		assert.Equal(t, 3, count, "Data should be untouched")
	})

	t.Run("UnparseableRejected", func(t *testing.T) {
		rt, _ := newSQLRuntime(t)

		_, err := rt.RunString(t.Context(), `sql.query('PRAGMA case_sensitive_like = true')`)
		require.Error(t, err, "SQL the guard cannot parse should fail closed")
		assert.Contains(t, err.Error(), "not read-only", "Error should carry the read-only reason")
	})

	t.Run("RowLimit", func(t *testing.T) {
		rt, _ := newSQLRuntime(t, jssql.WithMaxRows(2))

		_, err := rt.RunString(t.Context(), `sql.query('SELECT * FROM users')`)
		require.Error(t, err, "A result over the row limit should fail loudly")
		assert.Contains(t, err.Error(), "row limit", "Error should carry the row limit reason")

		result, err := rt.RunString(t.Context(), `sql.query('SELECT * FROM users LIMIT 2').length`)
		require.NoError(t, err, "A bounded query should pass")
		assert.Equal(t, int64(2), result.ToInteger(), "Bounded query should return the limited rows")
	})
}

// TestQueryOne tests single-row access.
func TestQueryOne(t *testing.T) {
	rt, _ := newSQLRuntime(t)

	t.Run("ReturnsFirstRow", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `sql.queryOne('SELECT name FROM users WHERE age = ?', 30).name`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "alice", result.String(), "Matching row should be returned as an object")
	})

	t.Run("ReturnsNullWhenEmpty", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `sql.queryOne('SELECT name FROM users WHERE age = ?', 99) === null`)
		require.NoError(t, err, "Script should execute successfully")
		assert.True(t, result.ToBoolean(), "No match should yield null")
	})
}

// TestExec tests the write path and its capability gate.
func TestExec(t *testing.T) {
	t.Run("DisabledByDefault", func(t *testing.T) {
		rt, db := newSQLRuntime(t)

		result, err := rt.RunString(t.Context(), `
			try {
				sql.exec('DELETE FROM users');
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Exec rejection should be catchable in scripts")
		assert.Contains(t, result.String(), "exec disabled", "Caught error should carry the capability reason")

		var count int

		require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users`).Scan(t.Context(), &count), "Count query should succeed")
		assert.Equal(t, 3, count, "Data should be untouched")
	})

	t.Run("InsertWithExec", func(t *testing.T) {
		rt, db := newSQLRuntime(t, jssql.WithExec())

		result, err := rt.RunString(t.Context(), `
			sql.exec('INSERT INTO users (name, age) VALUES (?, ?)', 'dave', 40).rowsAffected
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(1), result.ToInteger(), "Insert should report one affected row")

		var name string

		require.NoError(t, db.NewRaw(`SELECT name FROM users WHERE age = ?`, 40).Scan(t.Context(), &name), "Inserted row should be readable")
		assert.Equal(t, "dave", name, "Inserted values should bind from script arguments")
	})

	t.Run("UpdateReportsAffectedRows", func(t *testing.T) {
		rt, _ := newSQLRuntime(t, jssql.WithExec())

		result, err := rt.RunString(t.Context(), `
			sql.exec('UPDATE users SET age = age + 1 WHERE age >= ?', 25).rowsAffected
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(2), result.ToInteger(), "Update should report every affected row")
	})
}
