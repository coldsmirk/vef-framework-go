package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBStringOperationsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBStringOperationsTestSuite tests string operation conditions and their case-insensitive variants.
type CBStringOperationsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestContains tests Contains and OrContains conditions.
func (suite *CBStringOperationsTestSuite) TestContains() {
	suite.T().Logf("Testing Contains condition for %s", suite.ds.Kind)

	suite.Run("BasicContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Contains("name", "Alice")
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.Contains(users[0].Name, "Alice", "Should contain Alice in name")
	})

	suite.Run("OrContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Contains("name", "Alice").
						OrContains("name", "Bob")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestStartsWith tests StartsWith and OrStartsWith conditions.
func (suite *CBStringOperationsTestSuite) TestStartsWith() {
	suite.T().Logf("Testing StartsWith condition for %s", suite.ds.Kind)

	suite.Run("BasicStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWith("name", "Alice")
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.True(len(users[0].Name) >= 5 && users[0].Name[:5] == "Alice",
			"Should start with Alice")
	})

	suite.Run("OrStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWith("name", "Alice").
						OrStartsWith("name", "Bob")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestEndsWith tests EndsWith and OrEndsWith conditions.
func (suite *CBStringOperationsTestSuite) TestEndsWith() {
	suite.T().Logf("Testing EndsWith condition for %s", suite.ds.Kind)

	suite.Run("BasicEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWith("name", "Johnson")
				}),
		)

		suite.Len(users, 1, "Should find one user")
		suite.True(len(users[0].Name) >= 7 && users[0].Name[len(users[0].Name)-7:] == "Johnson",
			"Should end with Johnson")
	})

	suite.Run("OrEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWith("name", "Johnson").
						OrEndsWith("name", "Smith")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestContainsIgnoreCase tests ContainsIgnoreCase and OrContainsIgnoreCase conditions.
func (suite *CBStringOperationsTestSuite) TestContainsIgnoreCase() {
	suite.T().Logf("Testing ContainsIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicIContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ContainsIgnoreCase("name", "alice")
				}),
		)

		suite.Len(users, 1, "Should find one user case-insensitively")
	})

	suite.Run("OrIContains", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ContainsIgnoreCase("name", "alice").
						OrContainsIgnoreCase("name", "bob")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users case-insensitively")
	})
}

// TestStartsWithIgnoreCase tests StartsWithIgnoreCase and OrStartsWithIgnoreCase conditions.
func (suite *CBStringOperationsTestSuite) TestStartsWithIgnoreCase() {
	suite.T().Logf("Testing StartsWithIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicIStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWithIgnoreCase("name", "alice")
				}),
		)

		suite.Len(users, 1, "Should find one user case-insensitively")
	})

	suite.Run("OrIStartsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWithIgnoreCase("name", "alice").
						OrStartsWithIgnoreCase("name", "bob")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users case-insensitively")
	})
}

// TestEndsWithIgnoreCase tests EndsWithIgnoreCase and OrEndsWithIgnoreCase conditions.
func (suite *CBStringOperationsTestSuite) TestEndsWithIgnoreCase() {
	suite.T().Logf("Testing EndsWithIgnoreCase condition for %s", suite.ds.Kind)

	suite.Run("BasicIEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWithIgnoreCase("name", "johnson")
				}),
		)

		suite.Len(users, 1, "Should find one user case-insensitively")
	})

	suite.Run("OrIEndsWith", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWithIgnoreCase("name", "johnson").
						OrEndsWithIgnoreCase("name", "smith")
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users case-insensitively")
	})
}

// TestContainsAny tests ContainsAny and OrContainsAny conditions.
func (suite *CBStringOperationsTestSuite) TestContainsAny() {
	suite.T().Logf("Testing ContainsAny condition for %s", suite.ds.Kind)

	suite.Run("BasicContainsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ContainsAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})

	suite.Run("OrContainsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.ContainsAny("name", []string{"Alice"}).
						OrContainsAny("name", []string{"Charlie"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestStartsWithAny tests StartsWithAny and OrStartsWithAny conditions.
func (suite *CBStringOperationsTestSuite) TestStartsWithAny() {
	suite.T().Logf("Testing StartsWithAny condition for %s", suite.ds.Kind)

	suite.Run("BasicStartsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWithAny("name", []string{"Alice", "Bob"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})

	suite.Run("OrStartsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.StartsWithAny("name", []string{"Alice"}).
						OrStartsWithAny("name", []string{"Charlie"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}

// TestEndsWithAny tests EndsWithAny and OrEndsWithAny conditions.
func (suite *CBStringOperationsTestSuite) TestEndsWithAny() {
	suite.T().Logf("Testing EndsWithAny condition for %s", suite.ds.Kind)

	suite.Run("BasicEndsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWithAny("name", []string{"Johnson", "Smith"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})

	suite.Run("OrEndsWithAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EndsWithAny("name", []string{"Johnson"}).
						OrEndsWithAny("name", []string{"Brown"})
				}).
				OrderBy("name"),
		)

		suite.Len(users, 2, "Should find two users")
	})
}
