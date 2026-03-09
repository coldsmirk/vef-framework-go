package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBBasicComparisonTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBBasicComparisonTestSuite tests basic comparison and column comparison conditions.
type CBBasicComparisonTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestEquals tests Equals and OrEquals conditions.
func (suite *CBBasicComparisonTestSuite) TestEquals() {
	suite.T().Logf("Testing Equals condition for %s", suite.ds.Kind)

	suite.Run("BasicEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("name", "Alice Johnson")
				}),
		)

		suite.Len(users, 1, "Should find exactly one user")
		suite.Equal("Alice Johnson", users[0].Name, "Should find Alice Johnson")
	})

	suite.Run("OrEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("name", "Alice Johnson").
						OrEquals("name", "Bob Smith")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
		suite.Equal("Alice Johnson", users[0].Name, "Should find Alice first")
		suite.Equal("Bob Smith", users[1].Name, "Should find Bob second")
	})

	suite.Run("EqualsWithDifferentTypes", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 30)
				}),
		)

		suite.Len(users, 1, "Should find one user with age 30")
		suite.Equal(int16(30), users[0].Age, "Should match age 30")

		activeUsers := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("is_active", true)
				}),
		)

		suite.True(len(activeUsers) > 0, "Should find active users")

		for _, user := range activeUsers {
			suite.True(user.IsActive, "Should only return active users")
		}
	})
}

// TestNotEquals tests NotEquals and OrNotEquals conditions.
func (suite *CBBasicComparisonTestSuite) TestNotEquals() {
	suite.T().Logf("Testing NotEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEquals("name", "Alice Johnson")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice")

		for _, user := range users {
			suite.NotEqual("Alice Johnson", user.Name, "Should not be Alice Johnson")
		}
	})

	suite.Run("OrNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEquals("name", "Alice Johnson").
						OrNotEquals("age", 25)
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")

		for _, user := range users {
			suite.True(user.Name != "Alice Johnson" || user.Age != 25,
				"Should match NotEquals condition")
		}
	})
}

// TestGreaterThan tests GreaterThan and OrGreaterThan conditions.
func (suite *CBBasicComparisonTestSuite) TestGreaterThan() {
	suite.T().Logf("Testing GreaterThan condition for %s", suite.ds.Kind)

	suite.Run("BasicGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThan("age", 30)
				}),
		)

		suite.Len(users, 10, "Should find users with age > 30")

		for _, user := range users {
			suite.True(user.Age > 30, "Should have age greater than 30")
		}
	})

	suite.Run("OrGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThan("age", 30).
						OrGreaterThan("age", 24)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")

		for _, user := range users {
			suite.True(user.Age > 30 || user.Age > 24,
				"Should match GreaterThan condition")
		}
	})
}

// TestGreaterThanOrEqual tests GreaterThanOrEqual and OrGreaterThanOrEqual conditions.
func (suite *CBBasicComparisonTestSuite) TestGreaterThanOrEqual() {
	suite.T().Logf("Testing GreaterThanOrEqual condition for %s", suite.ds.Kind)

	suite.Run("BasicGreaterThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqual("age", 30)
				}).
				OrderBy("age"),
		)

		suite.Len(users, 11, "Should find users with age >= 30")

		for _, user := range users {
			suite.True(user.Age >= 30, "Should have age >= 30")
		}
	})

	suite.Run("OrGreaterThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqual("age", 35).
						OrGreaterThanOrEqual("age", 25)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")

		for _, user := range users {
			suite.True(user.Age >= 35 || user.Age >= 25,
				"Should match GreaterThanOrEqual condition")
		}
	})
}

// TestLessThan tests LessThan and OrLessThan conditions.
func (suite *CBBasicComparisonTestSuite) TestLessThan() {
	suite.T().Logf("Testing LessThan condition for %s", suite.ds.Kind)

	suite.Run("BasicLessThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThan("age", 30)
				}),
		)

		suite.Len(users, 9, "Should find users with age < 30")

		for _, user := range users {
			suite.True(user.Age < 30, "Should have age less than 30")
		}
	})

	suite.Run("OrLessThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThan("age", 26).
						OrLessThan("age", 31)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")

		for _, user := range users {
			suite.True(user.Age < 26 || user.Age < 31,
				"Should match LessThan condition")
		}
	})
}

// TestLessThanOrEqual tests LessThanOrEqual and OrLessThanOrEqual conditions.
func (suite *CBBasicComparisonTestSuite) TestLessThanOrEqual() {
	suite.T().Logf("Testing LessThanOrEqual condition for %s", suite.ds.Kind)

	suite.Run("BasicLessThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqual("age", 30)
				}).
				OrderBy("age"),
		)

		suite.Len(users, 10, "Should find users with age <= 30")

		for _, user := range users {
			suite.True(user.Age <= 30, "Should have age <= 30")
		}
	})

	suite.Run("OrLessThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqual("age", 25).
						OrLessThanOrEqual("age", 35)
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")

		for _, user := range users {
			suite.True(user.Age <= 25 || user.Age <= 35,
				"Should match LessThanOrEqual condition")
		}
	})
}

// TestEqualsColumn tests EqualsColumn and OrEqualsColumn conditions.
func (suite *CBBasicComparisonTestSuite) TestEqualsColumn() {
	suite.T().Logf("Testing EqualsColumn condition for %s", suite.ds.Kind)

	suite.Run("BasicEqualsColumn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsColumn("created_at", "updated_at")
				}),
		)

		suite.Len(users, 0, "Should find no users since fixture data has different timestamps")
	})

	suite.Run("OrEqualsColumn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsColumn("created_at", "updated_at").
						OrEqualsColumn("created_by", "updated_by")
				}),
		)

		suite.True(len(users) > 0, "Should find users via OR fallback")
	})
}

// TestNotEqualsColumn tests NotEqualsColumn and OrNotEqualsColumn conditions.
func (suite *CBBasicComparisonTestSuite) TestNotEqualsColumn() {
	suite.T().Logf("Testing NotEqualsColumn condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEqualsColumn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsColumn("created_at", "updated_at")
				}),
		)

		suite.Len(users, 20, "Should find all users with different timestamps")
	})

	suite.Run("OrNotEqualsColumn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsColumn("name", "email").
						OrNotEqualsColumn("created_at", "updated_at")
				}),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestGreaterThanColumn tests GreaterThanColumn and OrGreaterThanColumn conditions.
func (suite *CBBasicComparisonTestSuite) TestGreaterThanColumn() {
	suite.T().Logf("Testing GreaterThanColumn condition for %s", suite.ds.Kind)

	suite.Run("BasicGreaterThanColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanColumn("view_count", "view_count")
				}),
		)

		suite.Len(posts, 0, "Should find no posts since column equals itself")
	})

	suite.Run("OrGreaterThanColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanColumn("view_count", "view_count").
						OrGreaterThanColumn("id", "user_id")
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})
}

// TestGreaterThanOrEqualColumn tests GreaterThanOrEqualColumn and OrGreaterThanOrEqualColumn conditions.
func (suite *CBBasicComparisonTestSuite) TestGreaterThanOrEqualColumn() {
	suite.T().Logf("Testing GreaterThanOrEqualColumn condition for %s", suite.ds.Kind)

	suite.Run("BasicGteColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqualColumn("view_count", "view_count")
				}),
		)

		suite.True(len(posts) > 0, "Should find all posts since column >= itself")
	})

	suite.Run("OrGteColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqualColumn("view_count", "view_count").
						OrGreaterThanOrEqualColumn("id", "user_id")
				}),
		)

		suite.True(len(posts) > 0, "Should find posts matching either condition")
	})
}

// TestLessThanColumn tests LessThanColumn and OrLessThanColumn conditions.
func (suite *CBBasicComparisonTestSuite) TestLessThanColumn() {
	suite.T().Logf("Testing LessThanColumn condition for %s", suite.ds.Kind)

	suite.Run("BasicLessThanColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanColumn("view_count", "view_count")
				}),
		)

		suite.Len(posts, 0, "Should find no posts since column equals itself")
	})

	suite.Run("OrLessThanColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanColumn("view_count", "view_count").
						OrLessThanColumn("user_id", "id")
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})
}

// TestLessThanOrEqualColumn tests LessThanOrEqualColumn and OrLessThanOrEqualColumn conditions.
func (suite *CBBasicComparisonTestSuite) TestLessThanOrEqualColumn() {
	suite.T().Logf("Testing LessThanOrEqualColumn condition for %s", suite.ds.Kind)

	suite.Run("BasicLteColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqualColumn("view_count", "view_count")
				}),
		)

		suite.True(len(posts) > 0, "Should find all posts since column <= itself")
	})

	suite.Run("OrLteColumn", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqualColumn("view_count", "view_count").
						OrLessThanOrEqualColumn("user_id", "id")
				}),
		)

		suite.True(len(posts) > 0, "Should find posts matching either condition")
	})
}
