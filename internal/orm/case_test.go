package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/schema"
)

// TestCaseAppendQueryEmpty verifies that a CASE expression with no WHEN clause is
// rejected at query-build time rather than rendering the invalid "CASE END".
func TestCaseAppendQueryEmpty(t *testing.T) {
	gen := newTestQueryGen()

	t.Run("NoClauses", func(t *testing.T) {
		c := new(caseExpr)
		_, err := c.AppendQuery(gen, nil)
		require.ErrorIs(t, err, ErrCaseMissingWhen, "Empty CASE must fail with ErrCaseMissingWhen")
	})

	t.Run("ElseOnlyStillRejected", func(t *testing.T) {
		c := &caseExpr{hasElse: true, elseExpr: schema.SafeQuery("?", []any{0})}
		_, err := c.AppendQuery(gen, nil)
		require.ErrorIs(t, err, ErrCaseMissingWhen, "CASE with only ELSE must fail with ErrCaseMissingWhen")
	})

	t.Run("WithClauseRenders", func(t *testing.T) {
		c := &caseExpr{
			clauses: []caseClause{{
				whenExpr: schema.SafeQuery("1 = 1", nil),
				thenExpr: schema.SafeQuery("?", []any{1}),
			}},
		}
		b, err := c.AppendQuery(gen, nil)
		require.NoError(t, err, "Non-empty CASE should render")
		assert.Equal(t, "CASE WHEN 1 = 1 THEN 1 END", string(b), "Should render a complete CASE expression")
	})
}
