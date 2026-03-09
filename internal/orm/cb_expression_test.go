package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBExpressionOperationsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBExpressionOperationsTestSuite tests expression-based condition methods.
type CBExpressionOperationsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestEqualsExpr tests EqualsExpr and OrEqualsExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestEqualsExpr() {
	suite.T().Logf("Testing EqualsExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.Equal(int16(30), users[0].Age, "User age should be 30")
	})

	suite.Run("OrEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 25)
					}).OrEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 35)
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestNotEqualsExpr tests NotEqualsExpr and OrNotEqualsExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestNotEqualsExpr() {
	suite.T().Logf("Testing NotEqualsExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 19, "Should find all users except age 30")

		for _, user := range users {
			suite.NotEqual(int16(30), user.Age, "User age should not be 30")
		}
	})

	suite.Run("OrNotEqualsExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 25)
					}).OrNotEqualsExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestGreaterThanExpr tests GreaterThanExpr and OrGreaterThanExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestGreaterThanExpr() {
	suite.T().Logf("Testing GreaterThanExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicGreaterThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}),
		)

		suite.Len(users, 10, "Should find users with age > 30")

		for _, user := range users {
			suite.True(user.Age > 30, "Age should be greater than 30")
		}
	})

	suite.Run("OrGreaterThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					}).OrGreaterThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 24)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestLessThanExpr tests LessThanExpr and OrLessThanExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestLessThanExpr() {
	suite.T().Logf("Testing LessThanExpr condition for %s", suite.ds.Kind)

	suite.Run("BasicLessThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 30)
					})
				}),
		)

		suite.Len(users, 9, "Should find users with age < 30")

		for _, user := range users {
			suite.True(user.Age < 30, "Age should be less than 30")
		}
	})

	suite.Run("OrLessThanExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 26)
					}).OrLessThanExpr("age", func(eb orm.ExprBuilder) any {
						return eb.Expr("?", 31)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestExpr tests Expr and OrExpr conditions.
func (suite *CBExpressionOperationsTestSuite) TestExpr() {
	suite.T().Logf("Testing Expr condition for %s", suite.ds.Kind)

	suite.Run("BasicExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age > ?", 30)
					})
				}),
		)

		suite.Len(users, 10, "Should find users with age > 30")

		for _, user := range users {
			suite.True(user.Age > 30, "Age should be greater than 30")
		}
	})

	suite.Run("OrExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age = ?", 25)
					}).OrExpr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age = ?", 35)
					})
				}).
				OrderBy("age"),
		)

		suite.Len(users, 2, "Should find two users")
	})

	suite.Run("ComplexExpr", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Expr(func(eb orm.ExprBuilder) any {
						return eb.Expr("age BETWEEN ? AND ?", 25, 35)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}
