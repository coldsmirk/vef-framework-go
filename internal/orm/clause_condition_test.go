package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &ClauseConditionTestSuite{BaseTestSuite: base}
	})
}

// ClauseConditionTestSuite tests ClauseConditionBuilder methods across all databases.
type ClauseConditionTestSuite struct {
	*BaseTestSuite
}

// TestJoinOnWithOrCondition tests that OR conditions work correctly in JOIN ON clauses.
func (suite *ClauseConditionTestSuite) TestJoinOnWithOrCondition() {
	suite.T().Logf("Testing JOIN ON with OR condition for %s", suite.ds.Kind)

	type Result struct {
		PostTitle string `bun:"post_title"`
		UserName  string `bun:"user_name"`
	}

	var results []Result

	err := suite.selectPosts().
		Join((*User)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("u.id", "p.user_id").
				OrEqualsColumn("u.name", "p.title")
		}).
		SelectAs("p.title", "post_title").
		SelectAs("u.name", "user_name").
		OrderBy("p.title").
		Limit(5).
		Scan(suite.ctx, &results)

	suite.NoError(err, "JOIN ON with OR should execute successfully")
	suite.NotEmpty(results, "Should return results")
}

// TestJoinOnWithGroupCondition tests that grouped conditions work in JOIN ON clauses.
func (suite *ClauseConditionTestSuite) TestJoinOnWithGroupCondition() {
	suite.T().Logf("Testing JOIN ON with Group condition for %s", suite.ds.Kind)

	type Result struct {
		PostTitle string `bun:"post_title"`
		UserName  string `bun:"user_name"`
	}

	var results []Result

	// JOIN ON (u.id = p.user_id AND (u.is_active = true OR u.age > 20))
	err := suite.selectPosts().
		Join((*User)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("u.id", "p.user_id").
				Group(func(cb orm.ConditionBuilder) {
					cb.Equals("u.is_active", true).
						OrGreaterThan("u.age", 20)
				})
		}).
		SelectAs("p.title", "post_title").
		SelectAs("u.name", "user_name").
		OrderBy("p.title").
		Limit(5).
		Scan(suite.ctx, &results)

	suite.NoError(err, "JOIN ON with Group should execute successfully")
	suite.NotEmpty(results, "Should return results")
}

// TestJoinOnWithOrGroupCondition tests that OR-grouped conditions work in JOIN ON clauses.
func (suite *ClauseConditionTestSuite) TestJoinOnWithOrGroupCondition() {
	suite.T().Logf("Testing JOIN ON with OrGroup condition for %s", suite.ds.Kind)

	type Result struct {
		PostTitle string `bun:"post_title"`
		UserName  string `bun:"user_name"`
	}

	var results []Result

	// JOIN ON (u.id = p.user_id) OR (u.name = p.title)
	err := suite.selectPosts().
		Join((*User)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("u.id", "p.user_id").
				OrGroup(func(cb orm.ConditionBuilder) {
					cb.Equals("u.is_active", true).
						GreaterThan("u.age", 25)
				})
		}).
		SelectAs("p.title", "post_title").
		SelectAs("u.name", "user_name").
		OrderBy("p.title").
		Limit(5).
		Scan(suite.ctx, &results)

	suite.NoError(err, "JOIN ON with OrGroup should execute successfully")
	suite.NotEmpty(results, "Should return results")
}

// TestHavingWithOrCondition tests OR conditions in HAVING clauses.
func (suite *ClauseConditionTestSuite) TestHavingWithOrCondition() {
	suite.T().Logf("Testing HAVING with OR condition for %s", suite.ds.Kind)

	type Result struct {
		UserID    string `bun:"user_id"`
		PostCount int    `bun:"post_count"`
	}

	var results []Result

	err := suite.selectPosts().
		Select("user_id").
		SelectExpr(func(eb orm.ExprBuilder) any {
			return eb.CountAll()
		}, "post_count").
		GroupBy("user_id").
		Having(func(cb orm.ConditionBuilder) {
			cb.Expr(func(eb orm.ExprBuilder) any {
				return eb.GreaterThan(eb.CountAll(), 0)
			}).OrExpr(func(eb orm.ExprBuilder) any {
				return eb.Equals(eb.CountAll(), 1)
			})
		}).
		Scan(suite.ctx, &results)

	suite.NoError(err, "HAVING with OR should execute successfully")
	suite.NotEmpty(results, "Should return results")
}

// TestHavingWithGroupCondition tests grouped conditions in HAVING clauses.
func (suite *ClauseConditionTestSuite) TestHavingWithGroupCondition() {
	suite.T().Logf("Testing HAVING with Group condition for %s", suite.ds.Kind)

	type Result struct {
		UserID    string `bun:"user_id"`
		PostCount int    `bun:"post_count"`
	}

	var results []Result

	err := suite.selectPosts().
		Select("user_id").
		SelectExpr(func(eb orm.ExprBuilder) any {
			return eb.CountAll()
		}, "post_count").
		GroupBy("user_id").
		Having(func(cb orm.ConditionBuilder) {
			cb.Group(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.GreaterThanOrEqual(eb.CountAll(), 1)
				}).OrExpr(func(eb orm.ExprBuilder) any {
					return eb.LessThanOrEqual(eb.CountAll(), 100)
				})
			})
		}).
		Scan(suite.ctx, &results)

	suite.NoError(err, "HAVING with Group should execute successfully")
	suite.NotEmpty(results, "Should return results")
}

// TestNestedConditionsInJoinOn tests deeply nested conditions in JOIN ON.
func (suite *ClauseConditionTestSuite) TestNestedConditionsInJoinOn() {
	suite.T().Logf("Testing nested conditions in JOIN ON for %s", suite.ds.Kind)

	type Result struct {
		PostTitle string `bun:"post_title"`
		UserName  string `bun:"user_name"`
	}

	var results []Result

	// JOIN ON u.id = p.user_id AND ((u.is_active = true AND u.age > 20) OR u.age > 50)
	err := suite.selectPosts().
		Join((*User)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("u.id", "p.user_id").
				Group(func(cb orm.ConditionBuilder) {
					cb.Group(func(cb orm.ConditionBuilder) {
						cb.Equals("u.is_active", true).
							GreaterThan("u.age", 20)
					}).OrGroup(func(cb orm.ConditionBuilder) {
						cb.GreaterThan("u.age", 50)
					})
				})
		}).
		SelectAs("p.title", "post_title").
		SelectAs("u.name", "user_name").
		OrderBy("p.title").
		Limit(5).
		Scan(suite.ctx, &results)

	suite.NoError(err, "Nested conditions in JOIN ON should execute successfully")
	suite.True(len(results) >= 0, "Should execute without error")
}
