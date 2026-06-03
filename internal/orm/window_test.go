package orm

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/schema"
)

// dialectFixture pairs a model-less ExprBuilder with a matching QueryGen so that
// builder expressions can be rendered to SQL without a live database connection.
// The builder carries no model, so eb.Column renders bare quoted identifiers.
type dialectFixture struct {
	eb  ExprBuilder
	gen schema.QueryGen
}

// newDialectFixture builds a DB-free ExprBuilder/QueryGen pair for the given
// dialect. The underlying connection is an in-memory SQLite shim that is never
// dialed: only SQL generation is exercised, so the bun dialect (which drives
// identifier quoting and ExecByDialect routing) is the only behavioral input.
func newDialectFixture(t *testing.T, dialect schema.Dialect) dialectFixture {
	t.Helper()

	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	require.NoError(t, err, "Should open SQLite shim for SQL generation")
	t.Cleanup(func() {
		require.NoError(t, sqldb.Close(), "Should close SQLite shim")
	})

	db := newBunDB(bun.NewDB(sqldb, dialect))

	return dialectFixture{
		eb:  db.NewSelect().ExprBuilder(),
		gen: schema.NewQueryGen(dialect),
	}
}

func (f dialectFixture) render(t *testing.T, appender schema.QueryAppender) string {
	t.Helper()

	b, err := appender.AppendQuery(f.gen, nil)
	require.NoError(t, err, "AppendQuery should render without error")

	return string(b)
}

// TestWindowBaseRendering covers the OVER clause shaping shared by every window
// function: PARTITION BY, ORDER BY and ROWS/RANGE frame bounds.
func TestWindowBaseRendering(t *testing.T) {
	f := newDialectFixture(t, sqlitedialect.New())

	t.Run("EmptyOver", func(t *testing.T) {
		got := f.render(t, f.eb.RowNumber(func(b RowNumberBuilder) {
			b.Over()
		}))

		assert.Contains(t, got, "ROW_NUMBER()", "Should render the function call")
		assert.Contains(t, got, "OVER ()", "Empty Over should render an empty window")
	})

	t.Run("PartitionByAndOrderBy", func(t *testing.T) {
		got := f.render(t, f.eb.RowNumber(func(b RowNumberBuilder) {
			b.Over().PartitionBy("user_id").OrderByDesc("age")
		}))

		assert.Contains(t, got, "OVER (PARTITION BY", "Should open the window with PARTITION BY")
		assert.Contains(t, got, "user_id", "Should partition on the requested column")
		assert.Contains(t, got, "ORDER BY", "Should render the ORDER BY clause")
		assert.Contains(t, got, "DESC", "Should honor descending order")
	})

	t.Run("RowsFrameBetween", func(t *testing.T) {
		got := f.render(t, f.eb.WinSum(func(b WindowSumBuilder) {
			b.Column("amount").Over().
				OrderBy("created_at").
				Rows().UnboundedPreceding().And().CurrentRow()
		}))

		assert.Contains(t, got, "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW",
			"Should render the full BETWEEN frame for ROWS")
	})

	t.Run("RangeFrameSingleBound", func(t *testing.T) {
		got := f.render(t, f.eb.WinSum(func(b WindowSumBuilder) {
			b.Column("amount").Over().
				OrderBy("created_at").
				Range().Preceding(3)
		}))

		assert.Contains(t, got, "RANGE 3 PRECEDING",
			"A single start bound should render without BETWEEN")
	})
}

// TestWindowFrameMissingBound locks the guard that rejects a frame unit with no
// start bound, which would otherwise emit "ROWS " or "BETWEEN  AND ...".
func TestWindowFrameMissingBound(t *testing.T) {
	f := newDialectFixture(t, sqlitedialect.New())

	t.Run("UnitWithoutAnyBound", func(t *testing.T) {
		appender := f.eb.WinSum(func(b WindowSumBuilder) {
			b.Column("amount").Over().OrderBy("created_at").Rows()
		})

		_, err := appender.AppendQuery(f.gen, nil)
		require.ErrorIs(t, err, ErrWindowFrameMissingBound,
			"Stopping at Rows() with no bound must error, not emit trailing garbage")
	})

	t.Run("BetweenWithoutStartBound", func(t *testing.T) {
		appender := f.eb.WinSum(func(b WindowSumBuilder) {
			b.Column("amount").Over().OrderBy("created_at").Rows().And().CurrentRow()
		})

		_, err := appender.AppendQuery(f.gen, nil)
		require.ErrorIs(t, err, ErrWindowFrameMissingBound,
			"BETWEEN with an unset start bound must error rather than emit 'BETWEEN  AND ...'")
	})
}

// TestWindowOffsetIdempotentRender guards the LAG/LEAD render path against state
// mutation: AppendQuery derives its args locally on each call, so rendering the
// same expression twice must produce identical SQL.
func TestWindowOffsetIdempotentRender(t *testing.T) {
	f := newDialectFixture(t, sqlitedialect.New())

	t.Run("LagColumnOffsetDefault", func(t *testing.T) {
		appender := f.eb.Lag(func(b LagBuilder) {
			b.Column("amount").Offset(2).DefaultValue(0).Over().OrderBy("created_at")
		})

		first := f.render(t, appender)
		second := f.render(t, appender)

		assert.Equal(t, first, second, "Repeated rendering must be side-effect free")
		assert.Contains(t, first, "LAG(", "Should render the LAG function")
		assert.Contains(t, first, "2", "Should carry the offset argument")
		assert.Contains(t, first, "OVER (", "Should render the OVER clause")
	})

	t.Run("LeadColumn", func(t *testing.T) {
		appender := f.eb.Lead(func(b LeadBuilder) {
			b.Column("amount").Over().OrderBy("created_at")
		})

		first := f.render(t, appender)
		second := f.render(t, appender)

		assert.Equal(t, first, second, "Repeated rendering must be side-effect free")
		assert.Contains(t, first, "LEAD(", "Should render the LEAD function")
	})
}

// TestWindowNthValueRendering covers NTH_VALUE, which also derives its args at
// render time (column plus the N position).
func TestWindowNthValueRendering(t *testing.T) {
	f := newDialectFixture(t, sqlitedialect.New())

	appender := f.eb.NthValue(func(b NthValueBuilder) {
		b.Column("amount").N(2).Over().OrderBy("created_at")
	})

	first := f.render(t, appender)
	second := f.render(t, appender)

	assert.Equal(t, first, second, "Repeated rendering must be side-effect free")
	assert.Contains(t, first, "NTH_VALUE(", "Should render the NTH_VALUE function")
	assert.Contains(t, first, "2", "Should carry the N position argument")
}

// TestWindowNullSuffixDialectGating verifies the FROM/NULLS suffix is gated by
// dialect: non-Oracle/SQLServer dialects take the Default branch and emit no
// suffix even when RespectNulls / FromFirst are configured.
func TestWindowNullSuffixDialectGating(t *testing.T) {
	for _, tc := range []struct {
		name    string
		dialect schema.Dialect
	}{
		{"SQLite", sqlitedialect.New()},
		{"MySQL", mysqldialect.New()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDialectFixture(t, tc.dialect)

			got := f.render(t, f.eb.NthValue(func(b NthValueBuilder) {
				b.Column("amount").N(1).FromFirst().Over().OrderBy("created_at")
			}))

			assert.NotContains(t, got, "FROM FIRST",
				"FROM FIRST is Oracle/SQLServer only and must not leak into "+tc.name)
			assert.NotContains(t, got, "RESPECT NULLS",
				"NULLS suffix must not leak into "+tc.name)
		})
	}
}
