package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBPrimaryKeyConditionsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBPrimaryKeyConditionsTestSuite tests primary key condition methods.
type CBPrimaryKeyConditionsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestPKEquals tests PKEquals and OrPKEquals conditions.
func (suite *CBPrimaryKeyConditionsTestSuite) TestPKEquals() {
	suite.T().Logf("Testing PKEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicPKEquals", func() {
		var firstUser User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(1).
			Scan(suite.ctx, &firstUser)
		suite.Require().NoError(err, "Should get a user")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKEquals(firstUser.ID)
				}),
		)

		suite.Len(users, 1, "Should find exactly one user")
		suite.Equal(firstUser.ID, users[0].ID, "Should find the correct user")
	})

	suite.Run("OrPKEquals", func() {
		var allUsers []User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(2).
			Scan(suite.ctx, &allUsers)
		suite.Require().NoError(err, "Should get users")
		suite.Require().Len(allUsers, 2, "Should have two users")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKEquals(allUsers[0].ID).
						OrPKEquals(allUsers[1].ID)
				}).
				OrderBy("id"),
		)

		suite.Len(users, 2, "Should find two users")
		suite.Equal(allUsers[0].ID, users[0].ID, "First user ID should match")
		suite.Equal(allUsers[1].ID, users[1].ID, "Second user ID should match")
	})
}

// TestPKNotEquals tests PKNotEquals and OrPKNotEquals conditions.
func (suite *CBPrimaryKeyConditionsTestSuite) TestPKNotEquals() {
	suite.T().Logf("Testing PKNotEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicPKNotEquals", func() {
		var firstUser User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(1).
			Scan(suite.ctx, &firstUser)
		suite.Require().NoError(err, "Should get a user")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotEquals(firstUser.ID)
				}).
				OrderBy("id"),
		)

		suite.Len(users, 19, "Should find all users except the excluded one")

		for _, user := range users {
			suite.NotEqual(firstUser.ID, user.ID, "Should not be the excluded user")
		}
	})

	// NOT id1 OR NOT id2 => always true (every row satisfies at least one), returns all 20 users
	suite.Run("OrPKNotEquals", func() {
		var allUsers []User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(2).
			Scan(suite.ctx, &allUsers)
		suite.Require().NoError(err, "Should get users")
		suite.Require().Len(allUsers, 2, "Should have two users")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotEquals(allUsers[0].ID).
						OrPKNotEquals(allUsers[1].ID)
				}).
				OrderBy("id"),
		)

		suite.Len(users, 20, "NOT A OR NOT B should return all users")
	})
}

// TestPKIn tests PKIn and OrPKIn conditions.
func (suite *CBPrimaryKeyConditionsTestSuite) TestPKIn() {
	suite.T().Logf("Testing PKIn condition for %s", suite.ds.Kind)

	suite.Run("MultipleValues", func() {
		var allUsers []User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(2).
			Scan(suite.ctx, &allUsers)
		suite.Require().NoError(err, "Should get users")
		suite.Require().Len(allUsers, 2, "Should have two users")

		ids := []string{allUsers[0].ID, allUsers[1].ID}

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn(ids)
				}).
				OrderBy("id"),
		)

		suite.Len(users, 2, "Should find two users")
		suite.Equal(allUsers[0].ID, users[0].ID, "First user ID should match")
		suite.Equal(allUsers[1].ID, users[1].ID, "Second user ID should match")
	})

	// Single-element slice exercises the bun.Tuple single-value path
	suite.Run("SingleValue", func() {
		var firstUser User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(1).
			Scan(suite.ctx, &firstUser)
		suite.Require().NoError(err, "Should get a user")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn([]string{firstUser.ID})
				}),
		)

		suite.Len(users, 1, "Single-value IN should find exactly one user")
		suite.Equal(firstUser.ID, users[0].ID, "Should find the correct user")
	})

	suite.Run("OrPKIn", func() {
		var allUsers []User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(2).
			Scan(suite.ctx, &allUsers)
		suite.Require().NoError(err, "Should get users")
		suite.Require().Len(allUsers, 2, "Should have two users")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn([]string{allUsers[0].ID}).
						OrPKIn([]string{allUsers[1].ID})
				}).
				OrderBy("id"),
		)

		suite.Len(users, 2, "Should find two users via OR")
		suite.Equal(allUsers[0].ID, users[0].ID, "First user ID should match")
		suite.Equal(allUsers[1].ID, users[1].ID, "Second user ID should match")
	})
}

// TestPKNotIn tests PKNotIn and OrPKNotIn conditions.
func (suite *CBPrimaryKeyConditionsTestSuite) TestPKNotIn() {
	suite.T().Logf("Testing PKNotIn condition for %s", suite.ds.Kind)

	suite.Run("MultipleValues", func() {
		var allUsers []User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(2).
			Scan(suite.ctx, &allUsers)
		suite.Require().NoError(err, "Should get users")
		suite.Require().Len(allUsers, 2, "Should have two users")

		excludedIDs := []string{allUsers[0].ID, allUsers[1].ID}

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn(excludedIDs)
				}).
				OrderBy("id"),
		)

		suite.Len(users, 18, "Should find all users except the two excluded")

		for _, user := range users {
			suite.NotEqual(allUsers[0].ID, user.ID, "Should not contain first excluded user")
			suite.NotEqual(allUsers[1].ID, user.ID, "Should not contain second excluded user")
		}
	})

	// Single-element slice exercises the bun.Tuple single-value path
	suite.Run("SingleValue", func() {
		var firstUser User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(1).
			Scan(suite.ctx, &firstUser)
		suite.Require().NoError(err, "Should get a user")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn([]string{firstUser.ID})
				}).
				OrderBy("id"),
		)

		suite.Len(users, 19, "Single-value NOT IN should exclude exactly one user")

		for _, user := range users {
			suite.NotEqual(firstUser.ID, user.ID, "Should not be the excluded user")
		}
	})

	// NOT IN {id1} OR NOT IN {id2} => always true (every row satisfies at least one), returns all 20 users
	suite.Run("OrPKNotIn", func() {
		var allUsers []User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(2).
			Scan(suite.ctx, &allUsers)
		suite.Require().NoError(err, "Should get users")
		suite.Require().Len(allUsers, 2, "Should have two users")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn([]string{allUsers[0].ID}).
						OrPKNotIn([]string{allUsers[1].ID})
				}).
				OrderBy("id"),
		)

		suite.Len(users, 20, "NOT IN A OR NOT IN B should return all users")
	})
}

// TestPKWithAlias tests PK conditions with explicit table alias parameter.
func (suite *CBPrimaryKeyConditionsTestSuite) TestPKWithAlias() {
	suite.T().Logf("Testing PK conditions with alias for %s", suite.ds.Kind)

	suite.Run("PKEqualsWithAlias", func() {
		var firstUser User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(1).
			Scan(suite.ctx, &firstUser)
		suite.Require().NoError(err, "Should get a user")

		// "u" is the default alias for test_user table
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKEquals(firstUser.ID, "u")
				}),
		)

		suite.Len(users, 1, "PKEquals with alias should find one user")
		suite.Equal(firstUser.ID, users[0].ID, "Should find the correct user")
	})

	suite.Run("PKInWithAlias", func() {
		var allUsers []User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(2).
			Scan(suite.ctx, &allUsers)
		suite.Require().NoError(err, "Should get users")
		suite.Require().Len(allUsers, 2, "Should have two users")

		ids := []string{allUsers[0].ID, allUsers[1].ID}

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn(ids, "u")
				}).
				OrderBy("id"),
		)

		suite.Len(users, 2, "PKIn with alias should find two users")
		suite.Equal(allUsers[0].ID, users[0].ID, "First user ID should match")
		suite.Equal(allUsers[1].ID, users[1].ID, "Second user ID should match")
	})

	suite.Run("PKNotInWithAlias", func() {
		var firstUser User

		err := suite.selectUsers().
			OrderBy("id").
			Limit(1).
			Scan(suite.ctx, &firstUser)
		suite.Require().NoError(err, "Should get a user")

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn([]string{firstUser.ID}, "u")
				}).
				OrderBy("id"),
		)

		suite.Len(users, 19, "PKNotIn with alias should exclude one user")

		for _, user := range users {
			suite.NotEqual(firstUser.ID, user.ID, "Should not be the excluded user")
		}
	})
}
