package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

// TestAggregateBaseRendering covers the shared aggregate shaping: the function
// name, DISTINCT, and an in-aggregate ORDER BY.
func TestAggregateBaseRendering(t *testing.T) {
	f := newDialectFixture(t, sqlitedialect.New())

	t.Run("CountAll", func(t *testing.T) {
		got := f.render(t, f.eb.Count(func(b CountBuilder) {
			b.All()
		}))

		assert.Contains(t, got, "COUNT(*)", "Count().All() should render COUNT(*)")
	})

	t.Run("CountDistinctColumn", func(t *testing.T) {
		got := f.render(t, f.eb.Count(func(b CountBuilder) {
			b.Column("user_id").Distinct()
		}))

		assert.Contains(t, got, "COUNT(DISTINCT", "Distinct should render the DISTINCT keyword")
		assert.Contains(t, got, "user_id", "Should count the requested column")
	})
}

// TestAggregateFilterNative verifies the native FILTER clause on dialects that
// support it (SQLite renders FILTER (WHERE ...)).
func TestAggregateFilterNative(t *testing.T) {
	f := newDialectFixture(t, sqlitedialect.New())

	got := f.render(t, f.eb.Count(func(b CountBuilder) {
		b.All().Filter(func(cb ConditionBuilder) {
			cb.GreaterThan("view_count", 80)
		})
	}))

	assert.Contains(t, got, "FILTER (WHERE", "SQLite should use the native FILTER clause")
	assert.Contains(t, got, "view_count", "Filter predicate should reference the column")
}

// TestAggregateFilterCompat exercises the FILTER-emulation path used by dialects
// without native FILTER (MySQL). COUNT folds into SUM(CASE ...); other aggregates
// wrap their argument in CASE.
func TestAggregateFilterCompat(t *testing.T) {
	f := newDialectFixture(t, mysqldialect.New())

	t.Run("CountFoldsIntoSum", func(t *testing.T) {
		got := f.render(t, f.eb.Count(func(b CountBuilder) {
			b.All().Filter(func(cb ConditionBuilder) {
				cb.Equals("status", "published")
			})
		}))

		assert.Contains(t, got, "SUM(CASE WHEN", "COUNT FILTER should fold into SUM(CASE ...) on MySQL")
		assert.NotContains(t, got, "FILTER (WHERE", "MySQL has no native FILTER clause")
	})

	// Regression: the FILTER-emulation path must not silently drop an ORDER BY
	// or NULLS suffix that survives into it. jsonArrayAgg keeps its ORDER BY on
	// MySQL (the strategy does not clear it), so the compat rewrite must still
	// render ORDER BY inside the function call.
	t.Run("PreservesOrderByAndNulls", func(t *testing.T) {
		got := f.render(t, f.eb.JSONArrayAgg(func(b JSONArrayAggBuilder) {
			b.Column("name").
				OrderBy("age").
				Filter(func(cb ConditionBuilder) {
					cb.GreaterThan("view_count", 0)
				})
		}))

		assert.Contains(t, got, "JSON_ARRAYAGG(CASE WHEN", "Should fold the FILTER predicate into a CASE argument")
		assert.Contains(t, got, "ORDER BY", "FILTER emulation must not drop the in-aggregate ORDER BY")
		assert.Contains(t, got, "age", "ORDER BY should reference the requested column")
	})
}

// TestAggregateDialectMapping verifies dialect-specific function selection: SQLite
// emulates BIT_OR via MAX over a non-zero CASE, while PostgreSQL-style dialects
// keep the native name (asserted indirectly through MySQL's native BIT_OR).
func TestAggregateDialectMapping(t *testing.T) {
	t.Run("SQLiteBitOrEmulation", func(t *testing.T) {
		f := newDialectFixture(t, sqlitedialect.New())

		got := f.render(t, f.eb.BitOr(func(b BitOrBuilder) {
			b.Column("flags")
		}))

		assert.Contains(t, got, "MAX(", "SQLite should emulate BIT_OR via MAX")
		assert.Contains(t, got, "CASE WHEN", "SQLite BIT_OR emulation should wrap the argument in CASE")
	})

	t.Run("MySQLBitOrNative", func(t *testing.T) {
		f := newDialectFixture(t, mysqldialect.New())

		got := f.render(t, f.eb.BitOr(func(b BitOrBuilder) {
			b.Column("flags")
		}))

		assert.Contains(t, got, "BIT_OR(", "MySQL should use the native BIT_OR function")
	})
}
