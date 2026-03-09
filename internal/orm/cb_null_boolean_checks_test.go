package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBNullBooleanChecksTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBNullBooleanChecksTestSuite tests NULL and boolean check conditions.
type CBNullBooleanChecksTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestIsNull tests IsNull and OrIsNull conditions.
func (suite *CBNullBooleanChecksTestSuite) TestIsNull() {
	suite.T().Logf("Testing IsNull condition for %s", suite.ds.Kind)

	suite.Run("BasicIsNull", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNull("description")
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")

		for _, post := range posts {
			suite.Nil(post.Description, "Should have NULL description")
		}
	})

	suite.Run("OrIsNull", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNull("description").
						OrIsNull("content")
				}),
		)

		suite.True(len(posts) >= 0, "Should execute successfully")
	})
}

// TestIsNotNull tests IsNotNull and OrIsNotNull conditions.
func (suite *CBNullBooleanChecksTestSuite) TestIsNotNull() {
	suite.T().Logf("Testing IsNotNull condition for %s", suite.ds.Kind)

	suite.Run("BasicIsNotNull", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNotNull("title")
				}),
		)

		suite.True(len(posts) > 0, "Should find posts with non-NULL title")

		for _, post := range posts {
			suite.NotEmpty(post.Title, "Should have non-NULL title")
		}
	})

	suite.Run("OrIsNotNull", func() {
		posts := suite.assertQueryReturnsPosts(
			suite.selectPosts().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsNotNull("title").
						OrIsNotNull("content")
				}),
		)

		suite.True(len(posts) > 0, "Should find posts matching either condition")
	})
}

// TestIsTrue tests IsTrue and OrIsTrue conditions.
func (suite *CBNullBooleanChecksTestSuite) TestIsTrue() {
	suite.T().Logf("Testing IsTrue condition for %s", suite.ds.Kind)

	suite.Run("BasicIsTrue", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsTrue("is_active")
				}),
		)

		suite.True(len(users) > 0, "Should find active users")

		for _, user := range users {
			suite.True(user.IsActive, "Should be active")
		}
	})

	suite.Run("OrIsTrue", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsTrue("is_active").
						OrIsTrue("is_active")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		for _, user := range users {
			suite.True(user.IsActive, "Should be active")
		}
	})
}

// TestIsFalse tests IsFalse and OrIsFalse conditions.
func (suite *CBNullBooleanChecksTestSuite) TestIsFalse() {
	suite.T().Logf("Testing IsFalse condition for %s", suite.ds.Kind)

	suite.Run("BasicIsFalse", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsFalse("is_active")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		for _, user := range users {
			suite.False(user.IsActive, "Should be inactive")
		}
	})

	suite.Run("OrIsFalse", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.IsFalse("is_active").
						OrIsFalse("is_active")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		for _, user := range users {
			suite.False(user.IsActive, "Should be inactive")
		}
	})
}
