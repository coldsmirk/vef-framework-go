package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBCompositePKTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBCompositePKTestSuite tests primary key condition methods with composite primary keys.
type CBCompositePKTestSuite struct {
	*ConditionBuilderTestSuite
}

// totalFavorites is the total number of user_favorite fixture records.
const totalFavorites = 12

// TestCompositePKEquals tests PKEquals and OrPKEquals with composite primary keys.
func (suite *CBCompositePKTestSuite) TestCompositePKEquals() {
	suite.T().Logf("Testing composite PKEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicPKEquals", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKEquals([]any{"usr001", "post001"})
				}),
		)

		suite.Len(favorites, 1, "Should find exactly one favorite")
		suite.Equal("usr001", favorites[0].UserID, "UserID should match")
		suite.Equal("post001", favorites[0].PostID, "PostID should match")
	})

	suite.Run("OrPKEquals", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKEquals([]any{"usr001", "post001"}).
						OrPKEquals([]any{"usr002", "post004"})
				}).
				OrderBy("user_id").
				OrderBy("post_id"),
		)

		suite.Len(favorites, 2, "Should find two favorites")
		suite.Equal("usr001", favorites[0].UserID, "First favorite UserID should match")
		suite.Equal("post001", favorites[0].PostID, "First favorite PostID should match")
		suite.Equal("usr002", favorites[1].UserID, "Second favorite UserID should match")
		suite.Equal("post004", favorites[1].PostID, "Second favorite PostID should match")
	})
}

// TestCompositePKNotEquals tests PKNotEquals and OrPKNotEquals with composite primary keys.
func (suite *CBCompositePKTestSuite) TestCompositePKNotEquals() {
	suite.T().Logf("Testing composite PKNotEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicPKNotEquals", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotEquals([]any{"usr001", "post001"})
				}),
		)

		suite.Len(favorites, totalFavorites-1, "Should find all favorites except the excluded one")

		for _, fav := range favorites {
			if fav.UserID == "usr001" {
				suite.NotEqual("post001", fav.PostID, "Should not contain the excluded favorite")
			}
		}
	})

	// NOT (pk1) OR NOT (pk2) => always true, returns all records
	suite.Run("OrPKNotEquals", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotEquals([]any{"usr001", "post001"}).
						OrPKNotEquals([]any{"usr002", "post001"})
				}),
		)

		suite.Len(favorites, totalFavorites, "NOT pk1 OR NOT pk2 should return all favorites")
	})
}

// TestCompositePKIn tests PKIn and OrPKIn with composite primary keys.
func (suite *CBCompositePKTestSuite) TestCompositePKIn() {
	suite.T().Logf("Testing composite PKIn condition for %s", suite.ds.Kind)

	suite.Run("MultipleValues", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn([][]any{
						{"usr001", "post001"},
						{"usr002", "post004"},
						{"usr003", "post006"},
					})
				}).
				OrderBy("user_id"),
		)

		suite.Len(favorites, 3, "Should find three favorites")
		suite.Equal("usr001", favorites[0].UserID, "First favorite UserID should match")
		suite.Equal("usr002", favorites[1].UserID, "Second favorite UserID should match")
		suite.Equal("usr003", favorites[2].UserID, "Third favorite UserID should match")
	})

	suite.Run("SingleValue", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn([][]any{
						{"usr001", "post002"},
					})
				}),
		)

		suite.Len(favorites, 1, "Single-value IN should find exactly one favorite")
		suite.Equal("usr001", favorites[0].UserID, "UserID should match")
		suite.Equal("post002", favorites[0].PostID, "PostID should match")
	})

	suite.Run("OrPKIn", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn([][]any{
						{"usr001", "post001"},
					}).
						OrPKIn([][]any{
							{"usr004", "post007"},
						})
				}).
				OrderBy("user_id"),
		)

		suite.Len(favorites, 2, "Should find two favorites via OR")
		suite.Equal("usr001", favorites[0].UserID, "First favorite UserID should match")
		suite.Equal("usr004", favorites[1].UserID, "Second favorite UserID should match")
	})
}

// TestCompositePKNotIn tests PKNotIn and OrPKNotIn with composite primary keys.
func (suite *CBCompositePKTestSuite) TestCompositePKNotIn() {
	suite.T().Logf("Testing composite PKNotIn condition for %s", suite.ds.Kind)

	suite.Run("MultipleValues", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn([][]any{
						{"usr001", "post001"},
						{"usr001", "post002"},
					})
				}),
		)

		suite.Len(favorites, totalFavorites-2, "Should find all favorites except the two excluded")

		for _, fav := range favorites {
			if fav.UserID == "usr001" {
				suite.NotEqual("post001", fav.PostID, "Should not contain first excluded favorite")
				suite.NotEqual("post002", fav.PostID, "Should not contain second excluded favorite")
			}
		}
	})

	suite.Run("SingleValue", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn([][]any{
						{"usr005", "post008"},
					})
				}),
		)

		suite.Len(favorites, totalFavorites-1, "Single-value NOT IN should exclude exactly one favorite")

		for _, fav := range favorites {
			if fav.UserID == "usr005" {
				suite.NotEqual("post008", fav.PostID, "Should not contain the excluded favorite")
			}
		}
	})

	// NOT IN {pk1} OR NOT IN {pk2} => always true, returns all records
	suite.Run("OrPKNotIn", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn([][]any{
						{"usr001", "post001"},
					}).
						OrPKNotIn([][]any{
							{"usr002", "post001"},
						})
				}),
		)

		suite.Len(favorites, totalFavorites, "NOT IN pk1 OR NOT IN pk2 should return all favorites")
	})
}

// TestCompositePKWithAlias tests composite PK conditions with explicit table alias.
func (suite *CBCompositePKTestSuite) TestCompositePKWithAlias() {
	suite.T().Logf("Testing composite PK conditions with alias for %s", suite.ds.Kind)

	suite.Run("PKEqualsWithAlias", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					// "uf" is the alias for test_user_favorite table
					cb.PKEquals([]any{"usr001", "post003"}, "uf")
				}),
		)

		suite.Len(favorites, 1, "PKEquals with alias should find one favorite")
		suite.Equal("usr001", favorites[0].UserID, "UserID should match")
		suite.Equal("post003", favorites[0].PostID, "PostID should match")
	})

	suite.Run("PKInWithAlias", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKIn([][]any{
						{"usr003", "post002"},
						{"usr004", "post003"},
					}, "uf")
				}).
				OrderBy("user_id"),
		)

		suite.Len(favorites, 2, "PKIn with alias should find two favorites")
		suite.Equal("usr003", favorites[0].UserID, "First favorite UserID should match")
		suite.Equal("usr004", favorites[1].UserID, "Second favorite UserID should match")
	})

	suite.Run("PKNotInWithAlias", func() {
		favorites := suite.assertQueryReturnsUserFavorites(
			suite.selectUserFavorites().
				Where(func(cb orm.ConditionBuilder) {
					cb.PKNotIn([][]any{
						{"usr001", "post001"},
					}, "uf")
				}),
		)

		suite.Len(favorites, totalFavorites-1, "PKNotIn with alias should exclude one favorite")

		for _, fav := range favorites {
			if fav.UserID == "usr001" {
				suite.NotEqual("post001", fav.PostID, "Should not contain the excluded favorite")
			}
		}
	})
}
