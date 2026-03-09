package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBNotStringTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBNotStringTestSuite tests negated string conditions and their Any/IgnoreCase variants.
type CBNotStringTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestNotContains tests NotContains and OrNotContains conditions.
func (suite *CBNotStringTestSuite) TestNotContains() {
	suite.T().Logf("Testing NotContains condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContains("name", "Alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice")

		for _, user := range users {
			suite.NotContains(user.Name, "Alice", "Should not contain Alice")
		}
	})

	suite.Run("OrNotContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContains("name", "Alice").
						OrNotContains("name", "Bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotContainsAny tests NotContainsAny and OrNotContainsAny conditions.
func (suite *CBNotStringTestSuite) TestNotContainsAny() {
	suite.T().Logf("Testing NotContainsAny condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContainsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users not containing Alice or Bob")
	})

	suite.Run("OrNotContainsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotContainsAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotContainsIgnoreCase tests NotContainsIgnoreCase and OrNotContainsIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestNotContainsIgnoreCase() {
	suite.T().Logf("Testing NotContainsIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContainsIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsIgnoreCase("name", "alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice case-insensitively")
	})

	suite.Run("OrNotContainsIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsIgnoreCase("name", "alice").
						OrNotContainsIgnoreCase("name", "bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotContainsAnyIgnoreCase tests NotContainsAnyIgnoreCase and OrNotContainsAnyIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestNotContainsAnyIgnoreCase() {
	suite.T().Logf("Testing NotContainsAnyIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicNotContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users not matching any value")
	})

	suite.Run("OrNotContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestContainsAnyIgnoreCase tests ContainsAnyIgnoreCase and OrContainsAnyIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestContainsAnyIgnoreCase() {
	suite.T().Logf("Testing ContainsAnyIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find Alice and Bob case-insensitively")
	})

	suite.Run("OrContainsAnyIgnoreCase", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrContainsAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotStartsWith tests NotStartsWith and OrNotStartsWith conditions.
func (suite *CBNotStringTestSuite) TestNotStartsWith() {
	suite.T().Logf("Testing NotStartsWith condition for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWith("name", "Alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice")
	})

	suite.Run("OrNotStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWith("name", "Alice").
						OrNotStartsWith("name", "Bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotStartsWithAny tests NotStartsWithAny and OrNotStartsWithAny conditions.
func (suite *CBNotStringTestSuite) TestNotStartsWithAny() {
	suite.T().Logf("Testing NotStartsWithAny condition for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users not starting with Alice or Bob")
	})

	suite.Run("OrNotStartsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotStartsWithAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotStartsWithIgnoreCase tests NotStartsWithIgnoreCase and OrNotStartsWithIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestNotStartsWithIgnoreCase() {
	suite.T().Logf("Testing NotStartsWithIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWithIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithIgnoreCase("name", "alice")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Alice")
	})

	suite.Run("OrNotStartsWithIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithIgnoreCase("name", "alice").
						OrNotStartsWithIgnoreCase("name", "bob")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotStartsWithAnyIgnoreCase tests NotStartsWithAnyIgnoreCase and OrNotStartsWithAnyIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestNotStartsWithAnyIgnoreCase() {
	suite.T().Logf("Testing NotStartsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicNotStartsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotStartsWithAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users not matching any prefix")
	})

	suite.Run("OrNotStartsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotStartsWithAnyIgnoreCase("name", []string{"alice"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestStartsWithAnyIgnoreCase tests StartsWithAnyIgnoreCase and OrStartsWithAnyIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestStartsWithAnyIgnoreCase() {
	suite.T().Logf("Testing StartsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicStartsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWithAnyIgnoreCase("name", []string{"alice", "bob"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find Alice and Bob")
	})

	suite.Run("OrStartsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrStartsWithAnyIgnoreCase("name", []string{"alice"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotEndsWith tests NotEndsWith and OrNotEndsWith conditions.
func (suite *CBNotStringTestSuite) TestNotEndsWith() {
	suite.T().Logf("Testing NotEndsWith condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWith("name", "Johnson")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Johnson")
	})

	suite.Run("OrNotEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWith("name", "Johnson").
						OrNotEndsWith("name", "Smith")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotEndsWithAny tests NotEndsWithAny and OrNotEndsWithAny conditions.
func (suite *CBNotStringTestSuite) TestNotEndsWithAny() {
	suite.T().Logf("Testing NotEndsWithAny condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithAny("name", []string{"Johnson", "Smith"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users not ending with Johnson or Smith")
	})

	suite.Run("OrNotEndsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotEndsWithAny("name", []string{"Johnson", "Smith"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotEndsWithIgnoreCase tests NotEndsWithIgnoreCase and OrNotEndsWithIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestNotEndsWithIgnoreCase() {
	suite.T().Logf("Testing NotEndsWithIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWithIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithIgnoreCase("name", "johnson")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 19, "Should find all users except Johnson")
	})

	suite.Run("OrNotEndsWithIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithIgnoreCase("name", "johnson").
						OrNotEndsWithIgnoreCase("name", "smith")
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestNotEndsWithAnyIgnoreCase tests NotEndsWithAnyIgnoreCase and OrNotEndsWithAnyIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestNotEndsWithAnyIgnoreCase() {
	suite.T().Logf("Testing NotEndsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicNotEndsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEndsWithAnyIgnoreCase("name", []string{"johnson", "smith"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users not matching any suffix")
	})

	suite.Run("OrNotEndsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotEndsWithAnyIgnoreCase("name", []string{"johnson"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}

// TestEndsWithAnyIgnoreCase tests EndsWithAnyIgnoreCase and OrEndsWithAnyIgnoreCase conditions.
func (suite *CBNotStringTestSuite) TestEndsWithAnyIgnoreCase() {
	suite.T().Logf("Testing EndsWithAnyIgnoreCase for %s", suite.ds.Kind)

	suite.Run("BasicEndsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWithAnyIgnoreCase("name", []string{"johnson", "smith"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find Johnson and Smith")
	})

	suite.Run("OrEndsWithAnyIC", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrEndsWithAnyIgnoreCase("name", []string{"johnson"})
				}).
				OrderBy("name"),
		)

		suite.True(len(users) > 0, "Should find users matching either condition")
	})
}
