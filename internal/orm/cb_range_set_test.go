package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBRangeSetOperationsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBRangeSetOperationsTestSuite tests range and set operation condition methods.
type CBRangeSetOperationsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestBetween tests Between and OrBetween conditions.
func (suite *CBRangeSetOperationsTestSuite) TestBetween() {
	suite.T().Logf("Testing Between condition for %s", suite.ds.Kind)

	suite.Run("BasicBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Between("age", 25, 30)
				}).
				OrderBy("age"),
		)

		suite.Len(users, 6, "Should find users with age between 25 and 30")

		for _, user := range users {
			suite.True(user.Age >= 25 && user.Age <= 30, "Age should be between 25 and 30")
		}
	})

	suite.Run("OrBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Between("age", 25, 26).
						OrBetween("age", 34, 36)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.True((user.Age >= 25 && user.Age <= 26) || (user.Age >= 34 && user.Age <= 36),
				"Age should match Between conditions")
		}
	})
}

// TestNotBetween tests NotBetween and OrNotBetween conditions.
func (suite *CBRangeSetOperationsTestSuite) TestNotBetween() {
	suite.T().Logf("Testing NotBetween condition for %s", suite.ds.Kind)

	suite.Run("BasicNotBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotBetween("age", 26, 34)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.True(user.Age < 26 || user.Age > 34, "Age should not be between 26 and 34")
		}
	})

	suite.Run("OrNotBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotBetween("age", 26, 29).
						OrNotBetween("age", 31, 34)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestBetweenExpr tests BetweenExpr and OrBetweenExpr conditions.
func (suite *CBRangeSetOperationsTestSuite) TestBetweenExpr() {
	suite.T().Logf("Testing BetweenExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicBetweenExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.BetweenExpr("age",
						func(eb orm.ExprBuilder) any {
							return eb.Expr("?", 25)
						},
						func(eb orm.ExprBuilder) any {
							return eb.Expr("?", 30)
						},
					)
				}).
				OrderBy("age"),
		)

		suite.Len(users, 6, "Should find users with age between 25 and 30")

		for _, user := range users {
			suite.True(user.Age >= 25 && user.Age <= 30, "Age should be between 25 and 30")
		}
	})

	suite.Run("OrBetweenExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.BetweenExpr("age",
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 25) },
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 26) },
					).OrBetweenExpr("age",
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 34) },
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 36) },
					)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestNotBetweenExpr tests NotBetweenExpr and OrNotBetweenExpr conditions.
func (suite *CBRangeSetOperationsTestSuite) TestNotBetweenExpr() {
	suite.T().Logf("Testing NotBetweenExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicNotBetweenExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotBetweenExpr("age",
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 26) },
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 34) },
					)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.True(user.Age < 26 || user.Age > 34, "Age should not be between 26 and 34")
		}
	})

	suite.Run("OrNotBetweenExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotBetweenExpr("age",
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 26) },
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 29) },
					).OrNotBetweenExpr("age",
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 31) },
						func(eb orm.ExprBuilder) any { return eb.Expr("?", 34) },
					)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestIn tests In and OrIn conditions.
func (suite *CBRangeSetOperationsTestSuite) TestIn() {
	suite.T().Logf("Testing In condition for %s", suite.ds.Kind)

	suite.Run("BasicInWithStrings", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.In("email", []string{"alice@example.com", "bob@example.com"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
		emails := []string{users[0].Email, users[1].Email}
		suite.Contains(emails, "alice@example.com", "Should include alice@example.com in result set")
		suite.Contains(emails, "bob@example.com", "Should include bob@example.com in result set")
	})

	suite.Run("BasicInWithIntegers", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.In("age", []int16{25, 35})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")
		suite.Equal(int16(25), users[0].Age, "First user age should be 25")
		suite.Equal(int16(35), users[1].Age, "Second user age should be 35")
	})

	suite.Run("OrIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.In("age", []int16{25}).
						OrIn("age", []int16{35})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestNotIn tests NotIn and OrNotIn conditions.
func (suite *CBRangeSetOperationsTestSuite) TestNotIn() {
	suite.T().Logf("Testing NotIn condition for %s", suite.ds.Kind)

	suite.Run("BasicNotIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotIn("email", []string{"charlie@example.com"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Charlie")

		for _, user := range users {
			suite.NotEqual("charlie@example.com", user.Email, "User email should not be charlie's")
		}
	})

	suite.Run("OrNotIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotIn("age", []int16{25}).
						OrNotIn("age", []int16{30})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestInExpr tests InExpr and OrInExpr conditions.
func (suite *CBRangeSetOperationsTestSuite) TestInExpr() {
	suite.T().Logf("Testing InExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicInExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.InExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?, ?", 25, 35)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrInExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.InExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 25)
					}).OrInExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 35)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestNotInExpr tests NotInExpr and OrNotInExpr conditions.
func (suite *CBRangeSetOperationsTestSuite) TestNotInExpr() {
	suite.T().Logf("Testing NotInExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicNotInExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotInExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("(?)", 30)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.NotEqual(int16(30), user.Age, "User age should not be 30")
		}
	})

	suite.Run("OrNotInExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotInExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("(?)", 25)
					}).OrNotInExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("(?)", 30)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}
