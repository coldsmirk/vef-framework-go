package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &TableOpsTestSuite{BaseTestSuite: base}
	})
}

// TableOpsTestSuite verifies the shared table-source helpers (query_table_ops.go) render
// identical SQL across every query type, guarding against the TableExpr/TableSubQuery
// parenthesization divergence between SELECT and the write-query builders.
type TableOpsTestSuite struct {
	*BaseTestSuite
}

// TestTableExprNoParens asserts TableExpr emits the expression verbatim, without enclosing
// parentheses, so table-valued sources (e.g. functions, bare names) render correctly.
// Assertions stay dialect-agnostic: the raw expression "test_user" is emitted as-is and the
// SELECT/subquery keywords are not identifier-quoted, so no per-dialect quoting is involved.
func (suite *TableOpsTestSuite) TestTableExprNoParens() {
	suite.T().Logf("Testing TableExpr parenthesization for %s", suite.ds.Kind)

	suite.Run("BareExprWithAlias", func() {
		sql := suite.db.NewSelect().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("test_user")
			}, "u").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("u.id")
			}).
			String()

		suite.Contains(sql, "test_user AS ", "TableExpr must render a bare source followed by AS, without wrapping parens")
		suite.NotContains(sql, "(test_user)", "TableExpr must not wrap the expression in parentheses")
	})

	suite.Run("BareExprNoAlias", func() {
		sql := suite.db.NewSelect().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("test_user")
			}).
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("id")
			}).
			String()

		suite.Contains(sql, "test_user", "TableExpr must render the bare source")
		suite.NotContains(sql, "(test_user)", "TableExpr must not wrap the expression in parentheses")
	})

	suite.Run("SubQueryExprIsSingleWrapped", func() {
		// ExprBuilder.SubQuery already encloses the subquery in parentheses, so TableExpr
		// must not add a second pair (which would produce an invalid double-wrap).
		sql := suite.db.NewSelect().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*User)(nil)).Select("id")
				})
			}, "u").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("u.id")
			}).
			String()

		suite.Contains(sql, "(SELECT", "TableExpr with a SubQuery must keep the subquery's parentheses")
		suite.NotContains(sql, "((SELECT", "TableExpr must not double-wrap a SubQuery expression")
	})
}

// TestTableSubQueryParens asserts TableSubQuery wraps the subquery in exactly one pair of
// parentheses, which is the dedicated subquery-source contract.
func (suite *TableOpsTestSuite) TestTableSubQueryParens() {
	suite.T().Logf("Testing TableSubQuery parenthesization for %s", suite.ds.Kind)

	sql := suite.db.NewSelect().
		TableSubQuery(func(sq orm.SelectQuery) {
			sq.Model((*User)(nil)).Select("id")
		}, "u").
		SelectExpr(func(eb orm.ExprBuilder) any {
			return eb.Expr("u.id")
		}).
		String()

	suite.Contains(sql, "(SELECT", "TableSubQuery must wrap the subquery in parentheses")
	suite.NotContains(sql, "((SELECT", "TableSubQuery must wrap the subquery in exactly one pair of parentheses")
}

// TestTableSourceConsistency asserts the shared helpers produce the same table-source
// fragment regardless of the query type that consumes them. The expected fragment is
// derived from the dialect at runtime so the assertion holds across PostgreSQL, MySQL,
// and SQLite without hard-coding identifier quoting.
func (suite *TableOpsTestSuite) TestTableSourceConsistency() {
	suite.T().Logf("Testing table-source consistency across query types for %s", suite.ds.Kind)

	suite.Run("TableExpr", func() {
		tableExpr := func(eb orm.ExprBuilder) any {
			return eb.Expr("test_user")
		}

		selectSQL := suite.db.NewSelect().
			TableExpr(tableExpr, "u").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("u.id")
			}).
			String()

		updateSQL := suite.db.NewUpdate().
			Model((*Post)(nil)).
			TableExpr(tableExpr, "u").
			Set("status", "x").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", "x")
			}).
			String()

		// "test_user" is a raw expression (never identifier-quoted), so the fragment is
		// identical across dialects and must appear verbatim in both query types.
		suite.Contains(selectSQL, "test_user AS ", "SELECT must render the shared TableExpr fragment")
		suite.Contains(updateSQL, "test_user AS ", "UPDATE must render the same TableExpr fragment as SELECT")
		suite.NotContains(selectSQL, "(test_user)", "SELECT TableExpr must not wrap the expression in parens")
		suite.NotContains(updateSQL, "(test_user)", "UPDATE TableExpr must not wrap the expression in parens")
	})
}
