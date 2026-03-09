package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBComprehensiveTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBComprehensiveTestSuite tests complex condition scenarios and comprehensive patterns.
type CBComprehensiveTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestApplyIf tests ApplyIf method for conditional application of conditions.
func (suite *CBComprehensiveTestSuite) TestApplyIf() {
	suite.T().Logf("Testing ApplyIf method for %s", suite.ds.Kind)

	suite.Run("BasicApplyIf", func() {
		applyCondition := true

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ApplyIf(applyCondition, func(cb orm.ConditionBuilder) {
						cb.Equals("is_active", true)
					})
				}),
		)

		suite.True(len(users) > 0, "Should find active users")

		for _, user := range users {
			suite.True(user.IsActive, "User should be active")
		}
	})

	suite.Run("ApplyNotApplied", func() {
		applyCondition := false

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ApplyIf(applyCondition, func(cb orm.ConditionBuilder) {
						cb.Equals("is_active", false)
					})
				}),
		)

		suite.Len(users, 20, "Should find all users (condition not applied)")
	})

	suite.Run("MultipleApply", func() {
		applyAge := true
		applyActive := false

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ApplyIf(applyAge, func(cb orm.ConditionBuilder) {
						cb.GreaterThan("age", 25)
					}).ApplyIf(applyActive, func(cb orm.ConditionBuilder) {
						cb.Equals("is_active", true)
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.True(user.Age > 25, "Age should be greater than 25")
		}
	})
}

// TestComplexConditionCombinations tests complex real-world condition scenarios.
func (suite *CBComprehensiveTestSuite) TestComplexConditionCombinations() {
	suite.T().Logf("Testing complex condition combinations for %s", suite.ds.Kind)

	suite.Run("SearchWithMultipleFilters", func() {
		// Simulate a search with multiple optional filters
		searchName := "Alice"
		minAge := 20
		maxAge := 40
		isActive := true

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ApplyIf(searchName != "", func(cb orm.ConditionBuilder) {
						cb.Contains("name", searchName)
					}).ApplyIf(minAge > 0, func(cb orm.ConditionBuilder) {
						cb.GreaterThanOrEqual("age", minAge)
					}).ApplyIf(maxAge > 0, func(cb orm.ConditionBuilder) {
						cb.LessThanOrEqual("age", maxAge)
					}).Equals("is_active", isActive)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("ComplexOrConditionsWithGrouping", func() {
		// (name LIKE '%Alice%' OR name LIKE '%Bob%') AND (age > 20 AND age < 40) AND is_active = true
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.OrGroup(func(cb orm.ConditionBuilder) {
						cb.Contains("name", "Alice").
							OrContains("name", "Bob")
					}).Group(func(cb orm.ConditionBuilder) {
						cb.GreaterThan("age", 20).
							LessThan("age", 40)
					}).Equals("is_active", true)
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("SubqueryWithComplexConditions", func() {
		// Find posts by active users with age > 25
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.InSubQuery("user_id", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("id").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("is_active", true).
									GreaterThan("age", 25)
							})
					})
				}),
		)

		suite.True(len(posts) > 0, "Should find posts")
	})
}

// TestEdgeCases tests edge cases and boundary conditions.
func (suite *CBComprehensiveTestSuite) TestEdgeCases() {
	suite.T().Logf("Testing edge cases for %s", suite.ds.Kind)

	suite.Run("EmptyInList", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.In("age", []int16{})
				}),
		)

		suite.Len(users, 0, "Should find no users with empty IN list")
	})

	suite.Run("NullComparisons", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNull("description").
						OrIsNotNull("description")
				}),
		)

		suite.True(len(posts) > 0, "Should find all posts")
	})

	suite.Run("ChainedOrConditions", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 25).
						OrEquals("age", 30).
						OrEquals("age", 35)
				}).
				OrderBy("age"),
		)

		suite.Len(users, 3, "Should find three users")
	})

	suite.Run("MixedAndOrConditions", func() {
		// age = 25 OR (age = 30 AND is_active = true)
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 25).
						OrGroup(func(cb orm.ConditionBuilder) {
							cb.Equals("age", 30).
								Equals("is_active", true)
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestPerformanceScenarios tests performance-related scenarios.
func (suite *CBComprehensiveTestSuite) TestPerformanceScenarios() {
	suite.T().Logf("Testing performance scenarios for %s", suite.ds.Kind)

	suite.Run("ManyConditions", func() {
		// Test with many conditions to ensure no performance degradation
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNotNull("id").
						IsNotNull("name").
						IsNotNull("email").
						IsNotNull("age").
						IsNotNull("is_active").
						IsNotNull("created_at").
						IsNotNull("updated_at").
						IsNotNull("created_by").
						IsNotNull("updated_by")
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("DeeplyNestedConditions", func() {
		// Test deeply nested conditions
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Group(func(cb orm.ConditionBuilder) {
						cb.Group(func(cb orm.ConditionBuilder) {
							cb.Group(func(cb orm.ConditionBuilder) {
								cb.Equals("is_active", true)
							})
						})
					})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}
