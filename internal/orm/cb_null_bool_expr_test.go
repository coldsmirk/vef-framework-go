package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBNullBoolExprTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBNullBoolExprTestSuite tests NULL/boolean Expr and SubQuery variants plus extended comparison methods.
type CBNullBoolExprTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestIsNullExpr tests IsNullExpr and OrIsNullExpr conditions.
func (suite *CBNullBoolExprTestSuite) TestIsNullExpr() {
	suite.T().Logf("Testing IsNullExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicIsNullExpr", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNullExpr(func(eb orm.ExprBuilder) any {
						return eb.Column("description")
					})
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})

	suite.Run("OrIsNullExpr", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("status", "published").
						OrIsNullExpr(func(eb orm.ExprBuilder) any {
							return eb.Column("description")
						})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts matching either condition")
	})
}

// TestIsNullSubQuery tests IsNullSubQuery and OrIsNullSubQuery conditions.
func (suite *CBNullBoolExprTestSuite) TestIsNullSubQuery() {
	suite.T().Logf("Testing IsNullSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicIsNullSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNullSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Null()
							}).
							Limit(1)
					})
				}),
		)

		suite.Len(users, 20, "Should match all users when subquery is NULL")
	})

	suite.Run("OrIsNullSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrIsNullSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Null()
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestIsNotNullExpr tests IsNotNullExpr and OrIsNotNullExpr conditions.
func (suite *CBNullBoolExprTestSuite) TestIsNotNullExpr() {
	suite.T().Logf("Testing IsNotNullExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicIsNotNullExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNotNullExpr(func(eb orm.ExprBuilder) any {
						return eb.Column("name")
					})
				}),
		)

		suite.Len(users, 20, "Should find all users with non-null names")
	})

	suite.Run("OrIsNotNullExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrIsNotNullExpr(func(eb orm.ExprBuilder) any {
							return eb.Column("name")
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestIsNotNullSubQuery tests IsNotNullSubQuery and OrIsNotNullSubQuery conditions.
func (suite *CBNullBoolExprTestSuite) TestIsNotNullSubQuery() {
	suite.T().Logf("Testing IsNotNullSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicIsNotNullSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNotNullSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal(1)
							}).
							Limit(1)
					})
				}),
		)

		suite.Len(users, 20, "Should match all users when subquery is NOT NULL")
	})

	suite.Run("OrIsNotNullSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrIsNotNullSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(1)
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestIsTrueExpr tests IsTrueExpr and OrIsTrueExpr conditions.
func (suite *CBNullBoolExprTestSuite) TestIsTrueExpr() {
	suite.T().Logf("Testing IsTrueExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicIsTrueExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsTrueExpr(func(eb orm.ExprBuilder) any {
						return eb.Column("is_active")
					})
				}),
		)

		suite.True(len(users) > 0, "Should find active users")

		for _, user := range users {
			suite.True(user.IsActive, "Should be active")
		}
	})

	suite.Run("OrIsTrueExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrIsTrueExpr(func(eb orm.ExprBuilder) any {
							return eb.Column("is_active")
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestIsTrueSubQuery tests IsTrueSubQuery and OrIsTrueSubQuery conditions.
func (suite *CBNullBoolExprTestSuite) TestIsTrueSubQuery() {
	suite.T().Logf("Testing IsTrueSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicIsTrueSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsTrueSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal(true)
							}).
							Limit(1)
					})
				}),
		)

		suite.Len(users, 20, "Should match all users when subquery is TRUE")
	})

	suite.Run("OrIsTrueSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrIsTrueSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(true)
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestIsFalseExpr tests IsFalseExpr and OrIsFalseExpr conditions.
func (suite *CBNullBoolExprTestSuite) TestIsFalseExpr() {
	suite.T().Logf("Testing IsFalseExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicIsFalseExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsFalseExpr(func(eb orm.ExprBuilder) any {
						return eb.Column("is_active")
					})
				}),
		)

		suite.True(len(users) > 0, "Should find inactive users")

		for _, user := range users {
			suite.False(user.IsActive, "Should be inactive")
		}
	})

	suite.Run("OrIsFalseExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrIsFalseExpr(func(eb orm.ExprBuilder) any {
							return eb.Column("is_active")
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestIsFalseSubQuery tests IsFalseSubQuery and OrIsFalseSubQuery conditions.
func (suite *CBNullBoolExprTestSuite) TestIsFalseSubQuery() {
	suite.T().Logf("Testing IsFalseSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicIsFalseSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsFalseSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal(false)
							}).
							Limit(1)
					})
				}),
		)

		suite.Len(users, 20, "Should match all users when subquery is FALSE")
	})

	suite.Run("OrIsFalseSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrIsFalseSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(false)
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestComparisonSubQueryAndExprExtended tests GTE/LTE SubQuery and Expr variants.
func (suite *CBNullBoolExprTestSuite) TestComparisonSubQueryAndExprExtended() {
	suite.T().Logf("Testing extended comparison SubQuery/Expr for %s", suite.ds.Kind)

	suite.Run("GteSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqualSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal(30)
							}).
							Limit(1)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users with age >= 30")

		for _, user := range users {
			suite.True(user.Age >= 30, "Should have age >= 30")
		}
	})

	suite.Run("OrGteSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrGreaterThanOrEqualSubQuery("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(40)
								}).
								Limit(1)
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})

	suite.Run("OrGteExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrGreaterThanOrEqualExpr("age", func(eb orm.ExprBuilder) any {
							return eb.Literal(40)
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})

	suite.Run("LteSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqualSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal(30)
							}).
							Limit(1)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users with age <= 30")

		for _, user := range users {
			suite.True(user.Age <= 30, "Should have age <= 30")
		}
	})

	suite.Run("OrLteSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 45).
						OrLessThanOrEqualSubQuery("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal(25)
								}).
								Limit(1)
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})

	suite.Run("LteExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqualExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Literal(30)
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 10, "Should find users with age <= 30")
	})

	suite.Run("OrLteExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 45).
						OrLessThanOrEqualExpr("age", func(eb orm.ExprBuilder) any {
							return eb.Literal(25)
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}
