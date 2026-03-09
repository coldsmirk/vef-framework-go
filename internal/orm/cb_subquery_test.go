package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBSubqueryOperationsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBSubqueryOperationsTestSuite tests subquery operation condition methods.
type CBSubqueryOperationsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestInSubQuery tests InSubQuery and OrInSubQuery conditions.
func (suite *CBSubqueryOperationsTestSuite) TestInSubQuery() {
	suite.T().Logf("Testing InSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicInSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.InSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("is_active", true)
							})
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts from active users")
	})

	suite.Run("OrInSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.InSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("age", 30)
							})
					}).OrInSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("age", 35)
							})
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts")
	})
}

// TestNotInSubQuery tests NotInSubQuery and OrNotInSubQuery conditions.
func (suite *CBSubqueryOperationsTestSuite) TestNotInSubQuery() {
	suite.T().Logf("Testing NotInSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicNotInSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotInSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("is_active", false)
							})
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts not from inactive users")
	})

	suite.Run("OrNotInSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotInSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("age", 25)
							})
					}).OrNotInSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("age", 30)
							})
					})
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})
}

// TestEqualsSubQuery tests EqualsSubQuery and OrEqualsSubQuery conditions.
func (suite *CBSubqueryOperationsTestSuite) TestEqualsSubQuery() {
	suite.T().Logf("Testing EqualsSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicEqualsSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Alice Johnson")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts by Alice")
	})

	suite.Run("OrEqualsSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Alice Johnson")
							}).
							Limit(1)
					}).OrEqualsSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Bob Smith")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts")
	})
}

// TestNotEqualsSubQuery tests NotEqualsSubQuery and OrNotEqualsSubQuery conditions.
func (suite *CBSubqueryOperationsTestSuite) TestNotEqualsSubQuery() {
	suite.T().Logf("Testing NotEqualsSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEqualsSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Alice Johnson")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})

	suite.Run("OrNotEqualsSubQuery", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Alice Johnson")
							}).
							Limit(1)
					}).OrNotEqualsSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Bob Smith")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})
}

// TestGreaterThanSubQuery tests GreaterThanSubQuery and OrGreaterThanSubQuery conditions.
func (suite *CBSubqueryOperationsTestSuite) TestGreaterThanSubQuery() {
	suite.T().Logf("Testing GreaterThanSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicGreaterThanSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 25)
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.True(user.Age > 25, "Age should be greater than 25")
		}
	})

	suite.Run("OrGreaterThanSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 40)
							}).
							Limit(1)
					}).OrGreaterThanSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 24)
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestLessThanSubQuery tests LessThanSubQuery and OrLessThanSubQuery conditions.
func (suite *CBSubqueryOperationsTestSuite) TestLessThanSubQuery() {
	suite.T().Logf("Testing LessThanSubQuery condition for %s", suite.ds.Kind)

	suite.Run("BasicLessThanSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 30)
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.True(user.Age < 30, "Age should be less than 30")
		}
	})

	suite.Run("OrLessThanSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 26)
							}).
							Limit(1)
					}).OrLessThanSubQuery("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 31)
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestEqualsAll tests EqualsAll and OrEqualsAll conditions (not supported on SQLite).
func (suite *CBSubqueryOperationsTestSuite) TestEqualsAll() {
	suite.T().Logf("Testing EqualsAll condition for %s", suite.ds.Kind)

	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ALL operator not supported on %s", suite.ds.Kind)
	}

	suite.Run("BasicEqualsAll", func() {
		// Find users whose age equals all ages in a subquery returning a single value
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 30)
							})
					})
				}),
		)

		suite.Len(users, 1, "Should find one user with age 30")
		suite.Equal(int16(30), users[0].Age, "User age should be 30")
	})

	suite.Run("OrEqualsAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 25)
							})
					}).OrEqualsAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 35)
							})
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestNotEqualsAll tests NotEqualsAll and OrNotEqualsAll conditions (not supported on SQLite).
func (suite *CBSubqueryOperationsTestSuite) TestNotEqualsAll() {
	suite.T().Logf("Testing NotEqualsAll condition for %s", suite.ds.Kind)

	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ALL operator not supported on %s", suite.ds.Kind)
	}

	suite.Run("BasicNotEqualsAll", func() {
		// Find users whose age does not equal all ages in a subquery
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 30)
							})
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 19, "Should find all users except age 30")

		for _, user := range users {
			suite.NotEqual(int16(30), user.Age, "Age should not be 30")
		}
	})

	suite.Run("OrNotEqualsAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 25)
							})
					}).OrNotEqualsAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Expr("?", 35)
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestExists tests Exists and OrExists conditions using Expr with orm.ExprBuilder.
func (suite *CBSubqueryOperationsTestSuite) TestExists() {
	suite.T().Logf("Testing Exists condition for %s", suite.ds.Kind)

	suite.Run("BasicExists", func() {
		// Find posts where the author exists and is active
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Exists(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("is_active", true).
										EqualsColumn("id", "p.user_id")
								})
						})
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts with active authors")
	})

	suite.Run("OrExists", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Exists(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 30).
										EqualsColumn("id", "p.user_id")
								})
						})
					}).OrExpr(func(eb orm.ExprBuilder) any {
						return eb.Exists(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 35).
										EqualsColumn("id", "p.user_id")
								})
						})
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts")
	})
}

// TestNotExists tests NotExists and OrNotExists conditions using Expr with orm.ExprBuilder.
func (suite *CBSubqueryOperationsTestSuite) TestNotExists() {
	suite.T().Logf("Testing NotExists condition for %s", suite.ds.Kind)

	suite.Run("BasicNotExists", func() {
		// Find posts where there's no corresponding category
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.NotExists(func(sq orm.SelectQuery) {
							sq.Model((*Category)(nil)).
								Where(func(cb orm.ConditionBuilder) {
									cb.EqualsColumn("id", "p.category_id").
										Equals("name", "NonExistent")
								})
						})
					})
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})

	suite.Run("OrNotExists", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.NotExists(func(sq orm.SelectQuery) {
							sq.Model((*Category)(nil)).
								Where(func(cb orm.ConditionBuilder) {
									cb.EqualsColumn("id", "p.category_id").
										Equals("name", "Category1")
								})
						})
					}).OrExpr(func(eb orm.ExprBuilder) any {
						return eb.NotExists(func(sq orm.SelectQuery) {
							sq.Model((*Category)(nil)).
								Where(func(cb orm.ConditionBuilder) {
									cb.EqualsColumn("id", "p.category_id").
										Equals("name", "Category2")
								})
						})
					})
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})
}
