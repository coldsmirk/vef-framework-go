package orm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
)

// ConflictModel is a minimal model for rendering INSERT ... ON CONFLICT SQL.
type ConflictModel struct {
	ID   string `bun:"id,pk"`
	Name string `bun:"name"`
}

// TestConflictTargetWhere verifies that the partial-index predicate (target
// WHERE) is rendered only with a column-inference target. PostgreSQL rejects a
// WHERE clause after ON CONSTRAINT, so it must be dropped there. The SQLite
// dialect exercises the same buildPostgresSQLite branch without a live database.
func TestConflictTargetWhere(t *testing.T) {
	rawDB, err := database.Open(config.DataSourceConfig{Kind: config.SQLite})
	require.NoError(t, err, "Database.Open should succeed")

	t.Cleanup(func() { _ = rawDB.Close() })

	db, err := Open(rawDB, config.SQLite)
	require.NoError(t, err, "ORM open should succeed")

	bunDB, ok := db.(*BunDB)
	require.True(t, ok, "Open should return *BunDB for white-box rendering")

	renderConflict := func(t *testing.T, build func(ConflictBuilder)) string {
		t.Helper()

		q := NewInsertQuery(bunDB)
		q.Model(&ConflictModel{ID: "row-1", Name: "n"}).OnConflict(build)

		return q.query.String()
	}

	t.Run("ColumnTargetKeepsWhere", func(t *testing.T) {
		sql := renderConflict(t, func(cb ConflictBuilder) {
			cb.Columns("id").
				Where(func(cond ConditionBuilder) {
					cond.IsNotNull("id")
				}).
				DoUpdate().
				Set("name")
		})

		require.Contains(t, sql, "ON CONFLICT", "Column-inference target should emit ON CONFLICT")
		require.Contains(t, sql, "WHERE", "Partial-index predicate must be kept for a column target")
	})

	t.Run("ConstraintTargetDropsWhere", func(t *testing.T) {
		sql := renderConflict(t, func(cb ConflictBuilder) {
			cb.Constraint("conflict_model_pkey").
				Where(func(cond ConditionBuilder) {
					cond.IsNotNull("id")
				}).
				DoUpdate().
				Set("name")
		})

		require.Contains(t, sql, "ON CONFLICT ON CONSTRAINT", "Named constraint should emit ON CONFLICT ON CONSTRAINT")
		require.NotContains(t, sql, "WHERE", "Partial-index predicate is invalid after ON CONSTRAINT and must be dropped")
	})

	t.Run("ConstraintTargetUpperHasNoWhereKeyword", func(t *testing.T) {
		sql := strings.ToUpper(renderConflict(t, func(cb ConflictBuilder) {
			cb.Constraint("conflict_model_pkey").DoNothing()
		}))

		require.Contains(t, sql, "ON CONFLICT ON CONSTRAINT", "DoNothing on a named constraint should emit the constraint target")
		require.Contains(t, sql, "DO NOTHING", "DoNothing should emit DO NOTHING")
	})
}
