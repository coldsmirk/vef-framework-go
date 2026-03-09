package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBComparisonExpressionsTestSuite{BaseTestSuite: base}
	})
}

// EBComparisonExpressionsTestSuite tests comparison expression methods of orm.ExprBuilder.
type EBComparisonExpressionsTestSuite struct {
	*BaseTestSuite
}

// TestEquals tests the Equals comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestEquals() {
	suite.T().Logf("Testing Equals for %s", suite.ds.Kind)

	suite.Run("SimpleStringEquals", func() {
		type Result struct {
			ID     string `bun:"id"`
			Title  string `bun:"title"`
			Status string `bun:"status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "status").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Equals(eb.Column("status"), "published")
				})
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.Equal("published", result.Status, "Status should be 'published'")
		}
	})

	suite.Run("IntegerEquals", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Equals(eb.Column("view_count"), 42)
				})
			}).
			OrderBy("id").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")

		for _, result := range results {
			suite.Equal(int64(42), result.ViewCount, "ViewCount should be 42")
		}
	})

	suite.Run("EqualsInSelect", func() {
		type Result struct {
			ID          string `bun:"id"`
			Status      string `bun:"status"`
			IsPublished bool   `bun:"is_published"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Equals(eb.Column("status"), "published")
			}, "is_published").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.Status == "published" {
				suite.True(result.IsPublished, "IsPublished should be true for published posts")
			} else {
				suite.False(result.IsPublished, "IsPublished should be false for non-published posts")
			}
		}
	})
}

// TestNotEquals tests the NotEquals comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestNotEquals() {
	suite.T().Logf("Testing NotEquals for %s", suite.ds.Kind)

	suite.Run("SimpleStringNotEquals", func() {
		type Result struct {
			ID     string `bun:"id"`
			Title  string `bun:"title"`
			Status string `bun:"status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "status").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotEquals(eb.Column("status"), "draft")
				})
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.NotEqual("draft", result.Status, "Status should not be 'draft'")
		}
	})

	suite.Run("IntegerNotEquals", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotEquals(eb.Column("view_count"), 0)
				})
			}).
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.NotEqual(int64(0), result.ViewCount, "ViewCount should not be 0")
		}
	})

	suite.Run("NotEqualsInSelect", func() {
		type Result struct {
			ID       string `bun:"id"`
			Status   string `bun:"status"`
			NotDraft bool   `bun:"not_draft"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NotEquals(eb.Column("status"), "draft")
			}, "not_draft").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.Status != "draft" {
				suite.True(result.NotDraft, "NotDraft should be true for non-draft posts")
			} else {
				suite.False(result.NotDraft, "NotDraft should be false for draft posts")
			}
		}
	})
}

// TestGreaterThan tests the GreaterThan comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestGreaterThan() {
	suite.T().Logf("Testing GreaterThan for %s", suite.ds.Kind)

	suite.Run("SimpleGreaterThan", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.GreaterThan(eb.Column("view_count"), 50)
				})
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ViewCount > 50, "ViewCount should be > 50")
		}
	})

	suite.Run("GreaterThanInSelect", func() {
		type Result struct {
			ID         string `bun:"id"`
			ViewCount  int64  `bun:"view_count"`
			IsHighView bool   `bun:"is_high_view"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.GreaterThan(eb.Column("view_count"), 80)
			}, "is_high_view").
			OrderBy("view_count").
			Limit(10).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.ViewCount > 80 {
				suite.True(result.IsHighView, "IsHighView should be true when ViewCount > 80")
			} else {
				suite.False(result.IsHighView, "IsHighView should be false when ViewCount <= 80")
			}
		}
	})
}

// TestGreaterThanOrEqual tests the GreaterThanOrEqual comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestGreaterThanOrEqual() {
	suite.T().Logf("Testing GreaterThanOrEqual for %s", suite.ds.Kind)

	suite.Run("SimpleGreaterThanOrEqual", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.GreaterThanOrEqual(eb.Column("view_count"), 30)
				})
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ViewCount >= 30, "ViewCount should be >= 30")
		}
	})

	suite.Run("BoundaryGreaterThanOrEqual", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.GreaterThanOrEqual(eb.Column("view_count"), 42)
				})
			}).
			OrderBy("view_count").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")

		for _, result := range results {
			suite.True(result.ViewCount >= 42, "ViewCount should be >= 42")
		}
	})
}

// TestLessThan tests the LessThan comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestLessThan() {
	suite.T().Logf("Testing LessThan for %s", suite.ds.Kind)

	suite.Run("SimpleLessThan", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.LessThan(eb.Column("view_count"), 70)
				})
			}).
			OrderByDesc("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ViewCount < 70, "ViewCount should be < 70")
		}
	})

	suite.Run("LessThanInSelect", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
			IsLowView bool   `bun:"is_low_view"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.LessThan(eb.Column("view_count"), 30)
			}, "is_low_view").
			OrderBy("view_count").
			Limit(10).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.ViewCount < 30 {
				suite.True(result.IsLowView, "IsLowView should be true when ViewCount < 30")
			} else {
				suite.False(result.IsLowView, "IsLowView should be false when ViewCount >= 30")
			}
		}
	})
}

// TestLessThanOrEqual tests the LessThanOrEqual comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestLessThanOrEqual() {
	suite.T().Logf("Testing LessThanOrEqual for %s", suite.ds.Kind)

	suite.Run("SimpleLessThanOrEqual", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.LessThanOrEqual(eb.Column("view_count"), 50)
				})
			}).
			OrderByDesc("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ViewCount <= 50, "ViewCount should be <= 50")
		}
	})

	suite.Run("BoundaryLessThanOrEqual", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.LessThanOrEqual(eb.Column("view_count"), 42)
				})
			}).
			OrderByDesc("view_count").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")

		for _, result := range results {
			suite.True(result.ViewCount <= 42, "ViewCount should be <= 42")
		}
	})
}

// TestBetween tests the Between comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestBetween() {
	suite.T().Logf("Testing Between for %s", suite.ds.Kind)

	suite.Run("SimpleBetweenIntegers", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Between(eb.Column("view_count"), 30, 70)
				})
			}).
			OrderBy("view_count").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ViewCount >= 30 && result.ViewCount <= 70, "ViewCount should be between 30 and 70")
		}
	})

	suite.Run("BetweenAtBoundaries", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Between(eb.Column("view_count"), 42, 42)
				})
			}).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")

		for _, result := range results {
			suite.Equal(int64(42), result.ViewCount, "ViewCount should be exactly 42")
		}
	})

	suite.Run("BetweenInSelect", func() {
		type Result struct {
			ID           string `bun:"id"`
			ViewCount    int64  `bun:"view_count"`
			IsMediumView bool   `bun:"is_medium_view"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Between(eb.Column("view_count"), 30, 80)
			}, "is_medium_view").
			OrderBy("view_count").
			Limit(10).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.ViewCount >= 30 && result.ViewCount <= 80 {
				suite.True(result.IsMediumView, "IsMediumView should be true when between 30 and 80")
			} else {
				suite.False(result.IsMediumView, "IsMediumView should be false when outside range")
			}
		}
	})
}

// TestNotBetween tests the NotBetween comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestNotBetween() {
	suite.T().Logf("Testing NotBetween for %s", suite.ds.Kind)

	suite.Run("SimpleNotBetween", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotBetween(eb.Column("view_count"), 30, 70)
				})
			}).
			OrderBy("view_count").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ViewCount < 30 || result.ViewCount > 70, "ViewCount should be outside 30-70 range")
		}
	})

	suite.Run("NotBetweenInSelect", func() {
		type Result struct {
			ID            string `bun:"id"`
			ViewCount     int64  `bun:"view_count"`
			IsExtremeView bool   `bun:"is_extreme_view"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NotBetween(eb.Column("view_count"), 30, 80)
			}, "is_extreme_view").
			OrderBy("view_count").
			Limit(10).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.ViewCount < 30 || result.ViewCount > 80 {
				suite.True(result.IsExtremeView, "IsExtremeView should be true when outside 30-80 range")
			} else {
				suite.False(result.IsExtremeView, "IsExtremeView should be false when in range")
			}
		}
	})
}

// TestIn tests the In comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestIn() {
	suite.T().Logf("Testing In for %s", suite.ds.Kind)

	suite.Run("SimpleInStrings", func() {
		type Result struct {
			ID     string `bun:"id"`
			Title  string `bun:"title"`
			Status string `bun:"status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "status").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.In(eb.Column("status"), "published", "review")
				})
			}).
			OrderBy("id").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Status == "published" || result.Status == "review", "Status should be in allowed list")
		}
	})

	suite.Run("InIntegers", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.In(eb.Column("view_count"), 23, 42, 85, 96)
				})
			}).
			OrderBy("view_count").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")

		for _, result := range results {
			suite.True(
				result.ViewCount == 23 || result.ViewCount == 42 || result.ViewCount == 85 || result.ViewCount == 96,
				"ViewCount should be in allowed list")
		}
	})

	suite.Run("InSingleValue", func() {
		type Result struct {
			ID     string `bun:"id"`
			Status string `bun:"status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.In(eb.Column("status"), "draft")
				})
			}).
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")

		for _, result := range results {
			suite.Equal("draft", result.Status, "Status should be 'draft'")
		}
	})

	suite.Run("InInSelect", func() {
		type Result struct {
			ID             string `bun:"id"`
			Status         string `bun:"status"`
			IsActiveStatus bool   `bun:"is_active_status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.In(eb.Column("status"), "published", "review")
			}, "is_active_status").
			OrderBy("id").
			Limit(10).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.Status == "published" || result.Status == "review" {
				suite.True(result.IsActiveStatus, "IsActiveStatus should be true for active statuses")
			} else {
				suite.False(result.IsActiveStatus, "IsActiveStatus should be false for non-active statuses")
			}
		}
	})
}

// TestNotIn tests the NotIn comparison operator.
func (suite *EBComparisonExpressionsTestSuite) TestNotIn() {
	suite.T().Logf("Testing NotIn for %s", suite.ds.Kind)

	suite.Run("SimpleNotInStrings", func() {
		type Result struct {
			ID     string `bun:"id"`
			Title  string `bun:"title"`
			Status string `bun:"status"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "status").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotIn(eb.Column("status"), "draft", "archived")
				})
			}).
			OrderBy("id").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Status != "draft" && result.Status != "archived", "Status should not be in excluded list")
		}
	})

	suite.Run("NotInIntegers", func() {
		type Result struct {
			ID        string `bun:"id"`
			ViewCount int64  `bun:"view_count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.NotIn(eb.Column("view_count"), 0, 10, 20)
				})
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")

		for _, result := range results {
			suite.True(
				result.ViewCount != 0 && result.ViewCount != 10 && result.ViewCount != 20,
				"ViewCount should not be in excluded list")
		}
	})

	suite.Run("NotInInSelect", func() {
		type Result struct {
			ID            string `bun:"id"`
			Status        string `bun:"status"`
			IsNotExcluded bool   `bun:"is_not_excluded"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NotIn(eb.Column("status"), "draft", "archived")
			}, "is_not_excluded").
			OrderBy("id").
			Limit(10).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			if result.Status != "draft" && result.Status != "archived" {
				suite.True(result.IsNotExcluded, "IsNotExcluded should be true for non-excluded statuses")
			} else {
				suite.False(result.IsNotExcluded, "IsNotExcluded should be false for excluded statuses")
			}
		}
	})
}

// TestCombinedComparisons tests using multiple comparison operators together.
func (suite *EBComparisonExpressionsTestSuite) TestCombinedComparisons() {
	suite.T().Logf("Testing CombinedComparisons for %s", suite.ds.Kind)

	suite.Run("CombinedComparisonsInSelect", func() {
		type Result struct {
			ID           string `bun:"id"`
			ViewCount    int64  `bun:"view_count"`
			IsHighView   bool   `bun:"is_high_view"`
			IsMediumView bool   `bun:"is_medium_view"`
			IsLowView    bool   `bun:"is_low_view"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.GreaterThan(eb.Column("view_count"), 80)
			}, "is_high_view").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Between(eb.Column("view_count"), 30, 80)
			}, "is_medium_view").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.LessThan(eb.Column("view_count"), 30)
			}, "is_low_view").
			OrderBy("view_count").
			Limit(10).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			// Verify categorization logic
			if result.ViewCount > 80 {
				suite.True(result.IsHighView, "Should be high view")
				suite.False(result.IsMediumView, "Should not be medium view")
				suite.False(result.IsLowView, "Should not be low view")
			} else if result.ViewCount >= 30 && result.ViewCount <= 80 {
				suite.False(result.IsHighView, "Should not be high view")
				suite.True(result.IsMediumView, "Should be medium view")
				suite.False(result.IsLowView, "Should not be low view")
			} else {
				suite.False(result.IsHighView, "Should not be high view")
				suite.False(result.IsMediumView, "Should not be medium view")
				suite.True(result.IsLowView, "Should be low view")
			}
		}
	})
}

// TestIsTrue tests the IsTrue comparison operator for boolean expressions.
func (suite *EBComparisonExpressionsTestSuite) TestIsTrue() {
	suite.T().Logf("Testing IsTrue for %s", suite.ds.Kind)

	suite.Run("IsTrueOnBooleanColumn", func() {
		type Result struct {
			ID       string `bun:"id"`
			Name     string `bun:"name"`
			IsActive bool   `bun:"is_active"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "is_active").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.IsTrue(eb.Column("is_active"))
				})
			}).
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.IsActive, "IsActive should be true")
		}
	})

	suite.Run("IsTrueOnExpression", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			ViewCount int64  `bun:"view_count"`
			IsPopular bool   `bun:"is_popular"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.IsTrue(eb.GreaterThan(eb.Column("view_count"), 50))
			}, "is_popular").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.IsTrue(eb.GreaterThan(eb.Column("view_count"), 50))
				})
			}).
			OrderByDesc("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ViewCount > 50, "ViewCount should be > 50")
		}
	})
}

// TestIsFalse tests the IsFalse comparison operator for boolean expressions.
func (suite *EBComparisonExpressionsTestSuite) TestIsFalse() {
	suite.T().Logf("Testing IsFalse for %s", suite.ds.Kind)

	suite.Run("IsFalseOnBooleanColumn", func() {
		type Result struct {
			ID       string `bun:"id"`
			Name     string `bun:"name"`
			IsActive bool   `bun:"is_active"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("id", "name", "is_active").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.IsFalse(eb.Column("is_active"))
				})
			}).
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.False(result.IsActive, "IsActive should be false")
		}
	})

	suite.Run("IsFalseOnExpression", func() {
		type Result struct {
			ID           string `bun:"id"`
			Title        string `bun:"title"`
			ViewCount    int64  `bun:"view_count"`
			IsNotPopular bool   `bun:"is_not_popular"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.IsFalse(eb.GreaterThan(eb.Column("view_count"), 50))
			}, "is_not_popular").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.IsFalse(eb.GreaterThan(eb.Column("view_count"), 50))
				})
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.False(result.ViewCount > 50, "ViewCount should be <= 50")
		}
	})
}
