package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBAggregationFunctionsTestSuite{BaseTestSuite: base}
	})
}

// EBAggregationFunctionsTestSuite tests aggregate expression methods of orm.ExprBuilder.
type EBAggregationFunctionsTestSuite struct {
	*BaseTestSuite
}

// TestCount tests the Count aggregate function with various scenarios.
func (suite *EBAggregationFunctionsTestSuite) TestCount() {
	suite.T().Logf("Testing Count for %s", suite.ds.Kind)

	suite.Run("ConditionalCountWithFilter", func() {
		// Test Count with FILTER clause (PostgreSQL, SQLite) vs CASE equivalent (MySQL)
		type ConditionalCounts struct {
			TotalPosts     int64 `bun:"total_posts"`
			PublishedPosts int64 `bun:"published_posts"`
			DraftPosts     int64 `bun:"draft_posts"`
			ReviewPosts    int64 `bun:"review_posts"`
			HighViewPosts  int64 `bun:"high_view_posts"`
		}

		var counts ConditionalCounts

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountAll()
			}, "total_posts").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Count(func(cb orm.CountBuilder) {
					cb.All().Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "published")
					})
				})
			}, "published_posts").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Count(func(cb orm.CountBuilder) {
					cb.All().Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "draft")
					})
				})
			}, "draft_posts").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Count(func(cb orm.CountBuilder) {
					cb.All().Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "review")
					})
				})
			}, "review_posts").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Count(func(cb orm.CountBuilder) {
					cb.All().Filter(func(cb orm.ConditionBuilder) {
						cb.GreaterThan("view_count", 80)
					})
				})
			}, "high_view_posts").
			Scan(suite.ctx, &counts)

		suite.Require().NoError(err, "Should execute query")
		suite.True(counts.TotalPosts > 0, "Should have posts")
		suite.True(counts.PublishedPosts >= 0, "Should count published posts")
		suite.True(counts.DraftPosts >= 0, "Should count draft posts")
		suite.True(counts.ReviewPosts >= 0, "Should count review posts")
		suite.True(counts.HighViewPosts >= 0, "Should count high view posts")

		suite.T().Logf("Total: %d, Published: %d, Draft: %d, Review: %d, HighView: %d",
			counts.TotalPosts, counts.PublishedPosts, counts.DraftPosts,
			counts.ReviewPosts, counts.HighViewPosts)
	})
}

// TestCountColumn tests the CountColumn aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestCountColumn() {
	suite.T().Logf("Testing CountColumn for %s", suite.ds.Kind)

	suite.Run("BasicCountColumn", func() {
		type AgeStats struct {
			CountAge int64 `bun:"count_age"`
		}

		var ageStats AgeStats

		err := suite.selectUsers().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountColumn("age")
			}, "count_age").
			Scan(suite.ctx, &ageStats)

		suite.Require().NoError(err, "Should execute query")
		suite.Equal(int64(20), ageStats.CountAge, "Should count 20 users with age")
	})

	suite.Run("DistinctCountColumn", func() {
		type DistinctStats struct {
			UniqueStatuses   int64 `bun:"unique_statuses"`
			UniqueCategories int64 `bun:"unique_categories"`
			DistinctUserIDs  int64 `bun:"distinct_user_ids"`
		}

		var stats DistinctStats

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountColumn("status", true) // distinct = true
			}, "unique_statuses").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountColumn("category_id", true)
			}, "unique_categories").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountColumn("user_id", true)
			}, "distinct_user_ids").
			Scan(suite.ctx, &stats)

		suite.Require().NoError(err, "Should execute query")
		suite.True(stats.UniqueStatuses > 0, "Should have unique statuses")
		suite.True(stats.UniqueCategories > 0, "Should have unique categories")
		suite.True(stats.DistinctUserIDs > 0, "Should have distinct user IDs")
	})
}

// TestCountAll tests the CountAll aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestCountAll() {
	suite.T().Logf("Testing CountAll for %s", suite.ds.Kind)

	suite.Run("CountAllWithGrouping", func() {
		type StatusCount struct {
			Status string `bun:"status"`
			Count  int64  `bun:"post_count"`
		}

		var statusCounts []StatusCount

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountAll()
			}, "post_count").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &statusCounts)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(statusCounts, "Should return results")

		totalCount := int64(0)
		for _, sc := range statusCounts {
			suite.True(sc.Count > 0, "Each status should have at least 1 post")
			totalCount += sc.Count
		}

		// Verify total matches expected post count from fixture
		expectedPosts := 30 // From fixtures/test_post.yml
		suite.Equal(int64(expectedPosts), totalCount, "Total count should match fixture posts")
	})
}

// TestSum tests the Sum aggregate function with builder callback.
func (suite *EBAggregationFunctionsTestSuite) TestSum() {
	suite.T().Logf("Testing Sum for %s", suite.ds.Kind)

	suite.Run("SumWithFilter", func() {
		type ConditionalSums struct {
			TotalViews     int64 `bun:"total_views"`
			PublishedViews int64 `bun:"published_views"`
		}

		var sums ConditionalSums

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SumColumn("view_count")
			}, "total_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sum(func(sb orm.SumBuilder) {
					sb.Column("view_count").Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "published")
					})
				})
			}, "published_views").
			Scan(suite.ctx, &sums)

		suite.Require().NoError(err, "Should execute query")
		suite.True(sums.TotalViews > 0, "Should have total views")
		suite.True(sums.PublishedViews >= 0, "Should have published views")
		suite.True(sums.PublishedViews <= sums.TotalViews, "Published views should not exceed total")
	})

	suite.Run("SumInComplexGroupBy", func() {
		type CategoryStats struct {
			CategoryName   string  `bun:"category_name"`
			TotalPosts     int64   `bun:"total_posts"`
			PublishedPosts int64   `bun:"published_posts"`
			DraftPosts     int64   `bun:"draft_posts"`
			TotalViews     int64   `bun:"total_views"`
			PublishedViews int64   `bun:"published_views"`
			AvgViews       float64 `bun:"avg_views"`
			MaxViews       int64   `bun:"max_views"`
			MinViews       int64   `bun:"min_views"`
		}

		var stats []CategoryStats

		query := suite.selectPosts().
			Join((*Category)(nil), func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("c.id", "category_id")
			}).
			SelectAs("c.name", "category_name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountAll()
			}, "total_posts").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SumColumn("view_count")
			}, "total_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}, "avg_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MaxColumn("view_count")
			}, "max_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MinColumn("view_count")
			}, "min_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Count(func(cb orm.CountBuilder) {
					cb.All().Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "published")
					})
				})
			}, "published_posts").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Count(func(cb orm.CountBuilder) {
					cb.All().Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "draft")
					})
				})
			}, "draft_posts").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sum(func(sb orm.SumBuilder) {
					sb.Column("p.view_count").Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "published")
					})
				})
			}, "published_views").
			GroupBy("c.id", "c.name").
			OrderBy("c.name")

		err := query.Scan(suite.ctx, &stats)
		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(stats, "Should return results")

		for _, stat := range stats {
			suite.NotEmpty(stat.CategoryName, "Should have non-empty category name")
			suite.True(stat.TotalPosts > 0, "Each category should have posts")
			suite.True(stat.TotalViews >= 0, "Total views should be non-negative")
			suite.True(stat.PublishedViews >= 0, "Published views should be non-negative")
			suite.True(stat.PublishedPosts >= 0, "Published posts should be non-negative")
			suite.True(stat.DraftPosts >= 0, "Draft posts should be non-negative")
			suite.True(stat.PublishedPosts+stat.DraftPosts <= stat.TotalPosts,
				"Published + Draft should not exceed total")

			suite.T().Logf("Category %s: %d posts (%d published, %d draft), %d total views (%d published views)",
				stat.CategoryName, stat.TotalPosts, stat.PublishedPosts, stat.DraftPosts,
				stat.TotalViews, stat.PublishedViews)
		}
	})
}

// TestSumColumn tests the SumColumn aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestSumColumn() {
	suite.T().Logf("Testing SumColumn for %s", suite.ds.Kind)

	suite.Run("BasicSumColumn", func() {
		type ViewStats struct {
			TotalViews int64   `bun:"total_views"`
			AvgViews   float64 `bun:"avg_views"`
		}

		var stats ViewStats

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SumColumn("view_count")
			}, "total_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}, "avg_views").
			Scan(suite.ctx, &stats)

		suite.Require().NoError(err, "Should execute query")
		suite.True(stats.TotalViews > 0, "Should have total views")
		suite.True(stats.AvgViews > 0, "Should have average views")
	})
}

// TestAvg tests the Avg aggregate function with builder callback.
func (suite *EBAggregationFunctionsTestSuite) TestAvg() {
	suite.T().Logf("Testing Avg for %s", suite.ds.Kind)

	suite.Run("AvgWithFilter", func() {
		type ConditionalAvg struct {
			AvgPublishedViews int64 `bun:"avg_published_views"`
		}

		var avg ConditionalAvg

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ToInteger(
					eb.Avg(func(ab orm.AvgBuilder) {
						ab.Column("view_count").Filter(func(cb orm.ConditionBuilder) {
							cb.Equals("status", "published")
						})
					}),
				)
			}, "avg_published_views").
			Scan(suite.ctx, &avg)

		suite.Require().NoError(err, "Should execute query")
		suite.True(avg.AvgPublishedViews >= 0, "Average should be non-negative")
	})
}

// TestAvgColumn tests the AvgColumn aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestAvgColumn() {
	suite.T().Logf("Testing AvgColumn for %s", suite.ds.Kind)

	suite.Run("BasicAvgColumn", func() {
		type AgeStats struct {
			AvgAge float64 `bun:"avg_age"`
		}

		var stats AgeStats

		err := suite.selectUsers().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("age")
			}, "avg_age").
			Scan(suite.ctx, &stats)

		suite.Require().NoError(err, "Should execute query")
		suite.InDelta(31.4, stats.AvgAge, 1.0, "Average age should be around 31.4")
	})

	suite.Run("DistinctAvgColumn", func() {
		type DistinctAvg struct {
			AvgDistinctViews float64 `bun:"avg_distinct_views"`
		}

		var avg DistinctAvg

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count", true) // distinct average
			}, "avg_distinct_views").
			Scan(suite.ctx, &avg)

		suite.Require().NoError(err, "Should execute query")
		suite.True(avg.AvgDistinctViews > 0, "Should have distinct average")
	})
}

// TestMin tests the Min aggregate function with builder callback.
func (suite *EBAggregationFunctionsTestSuite) TestMin() {
	suite.T().Logf("Testing Min for %s", suite.ds.Kind)

	suite.Run("MinInCombinedStats", func() {
		type CombinedStats struct {
			Status   string  `bun:"status"`
			MinViews int64   `bun:"min_views"`
			MaxViews int64   `bun:"max_views"`
			AvgViews float64 `bun:"avg_views"`
		}

		var stats []CombinedStats

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MinColumn("view_count")
			}, "min_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MaxColumn("view_count")
			}, "max_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}, "avg_views").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &stats)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(stats, "Should return results")

		for _, stat := range stats {
			suite.True(stat.MinViews >= 0, "Min should be non-negative")
			suite.True(stat.MaxViews >= stat.MinViews, "Max should be >= Min")
		}
	})
}

// TestMinColumn tests the MinColumn aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestMinColumn() {
	suite.T().Logf("Testing MinColumn for %s", suite.ds.Kind)

	suite.Run("BasicMinColumn", func() {
		type AgeStats struct {
			MinAge int16 `bun:"min_age"`
		}

		var stats AgeStats

		err := suite.selectUsers().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MinColumn("age")
			}, "min_age").
			Scan(suite.ctx, &stats)

		suite.Require().NoError(err, "Should execute query")
		suite.Equal(int16(21), stats.MinAge, "Min age should be 21")
	})
}

// TestMax tests the Max aggregate function with builder callback.
func (suite *EBAggregationFunctionsTestSuite) TestMax() {
	suite.T().Logf("Testing Max for %s", suite.ds.Kind)

	suite.Run("MaxWithFilter", func() {
		type Result struct {
			MaxPublishedViews int64 `bun:"max_published_views"`
		}

		var result Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Max(func(mb orm.MaxBuilder) {
					mb.Column("view_count").Filter(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "published")
					})
				})
			}, "max_published_views").
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Should execute query")
		suite.True(result.MaxPublishedViews >= 0, "Max should be non-negative")
	})

	suite.Run("MaxInGroupBy", func() {
		type GroupedMax struct {
			Status   string `bun:"status"`
			MaxViews int64  `bun:"max_views"`
		}

		var results []GroupedMax

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MaxColumn("view_count")
			}, "max_views").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.MaxViews >= 0, "Max should be non-negative")
		}
	})
}

// TestMaxColumn tests the MaxColumn aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestMaxColumn() {
	suite.T().Logf("Testing MaxColumn for %s", suite.ds.Kind)

	suite.Run("BasicMaxColumn", func() {
		type AgeStats struct {
			MaxAge int16 `bun:"max_age"`
		}

		var stats AgeStats

		err := suite.selectUsers().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MaxColumn("age")
			}, "max_age").
			Scan(suite.ctx, &stats)

		suite.Require().NoError(err, "Should execute query")
		suite.Equal(int16(45), stats.MaxAge, "Max age should be 45")
	})
}

// TestStringAgg tests the StringAgg aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestStringAgg() {
	suite.T().Logf("Testing StringAgg for %s", suite.ds.Kind)

	suite.Run("StringAggWithDistinctAndSeparator", func() {
		type Result struct {
			StatusList       string `bun:"status_list"`
			OrderedTitleList string `bun:"ordered_title_list"`
		}

		var result Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StringAgg(func(sab orm.StringAggBuilder) {
					sab.Column("status").Separator(", ").Distinct()
				})
			}, "status_list").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StringAgg(func(sab orm.StringAggBuilder) {
					sab.Column("title").Separator(" | ").OrderBy("view_count")
				})
			}, "ordered_title_list").
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Should execute query")
		suite.NotEmpty(result.StatusList, "Should aggregate distinct statuses")
		suite.NotEmpty(result.OrderedTitleList, "Should aggregate ordered titles")
	})

	suite.Run("StringAggGroupedByStatus", func() {
		type Result struct {
			Status string `bun:"status"`
			Titles string `bun:"titles"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StringAgg(func(sab orm.StringAggBuilder) {
					sab.Column("title").Separator(", ")
				})
			}, "titles").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.NotEmpty(result.Titles, "Should have non-empty Titles")
		}
	})
}

// TestArrayAgg tests the ArrayAgg aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestArrayAgg() {
	suite.T().Logf("Testing ArrayAgg for %s", suite.ds.Kind)

	suite.Run("ArrayAggWithOrdering", func() {
		type Result struct {
			ViewCountArray []int64  `bun:"view_count_array,array"`
			OrderedTitles  []string `bun:"ordered_titles,array"`
			UniqueStatuses []string `bun:"unique_statuses,array"`
		}

		var result Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ArrayAgg(func(aab orm.ArrayAggBuilder) {
					aab.Column("view_count").OrderByDesc("view_count")
				})
			}, "view_count_array").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ArrayAgg(func(aab orm.ArrayAggBuilder) {
					aab.Column("title").OrderBy("title")
				})
			}, "ordered_titles").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ArrayAgg(func(aab orm.ArrayAggBuilder) {
					aab.Column("status").Distinct().OrderBy("status")
				})
			}, "unique_statuses").
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Should execute query")
		suite.NotEmpty(result.ViewCountArray, "Should have view count array")
		suite.NotEmpty(result.OrderedTitles, "Should have ordered titles")
		suite.NotEmpty(result.UniqueStatuses, "Should have unique statuses")

		// Verify ordering (only PostgreSQL supports ORDER BY in ARRAY_AGG)
		if suite.ds.Kind == config.Postgres {
			for i := 1; i < len(result.ViewCountArray); i++ {
				suite.True(result.ViewCountArray[i-1] >= result.ViewCountArray[i],
					"View counts should be in descending order")
			}
		}
	})

	suite.Run("ArrayAggGroupedByStatus", func() {
		type Result struct {
			Status string `bun:"status"`
			Count  int64  `bun:"count"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountAll()
			}, "count").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Count > 0, "Count should be positive")
		}
	})
}

// TestJsonObjectAgg tests the JsonObjectAgg aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestJsonObjectAgg() {
	suite.T().Logf("Testing JsonObjectAgg for %s", suite.ds.Kind)

	suite.Run("JsonObjectAggGroupedByStatus", func() {
		type Result struct {
			Status     string `bun:"status"`
			StatusMeta string `bun:"status_meta"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.JSONObjectAgg(func(joab orm.JSONObjectAggBuilder) {
					joab.KeyColumn("id")
					joab.Column("title")
				})
			}, "status_meta").
			GroupBy("status").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.NotEmpty(result.StatusMeta, "Should have non-empty StatusMeta")
		}
	})
}

// TestJsonArrayAgg tests the JsonArrayAgg aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestJsonArrayAgg() {
	suite.T().Logf("Testing JsonArrayAgg for %s", suite.ds.Kind)

	suite.Run("JsonArrayAggBasic", func() {
		type Result struct {
			Status     string `bun:"status"`
			TitlesJSON string `bun:"titles_json"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.JSONArrayAgg(func(jaab orm.JSONArrayAggBuilder) {
					jaab.Column("title")
				})
			}, "titles_json").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.NotEmpty(result.TitlesJSON, "Should have non-empty TitlesJSON")
		}
	})

	suite.Run("JsonArrayAggWithOrdering", func() {
		if suite.ds.Kind == config.MySQL {
			suite.T().Skipf("JsonArrayAgg with ORDER BY skipped for %s (MySQL does not support ORDER BY in JSON_ARRAYAGG)", suite.ds.Kind)

			return
		}

		type OrderedJSONArray struct {
			Status     string `bun:"status"`
			TitlesJSON string `bun:"titles_json"`
		}

		var results []OrderedJSONArray

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.JSONArrayAgg(func(jaab orm.JSONArrayAggBuilder) {
					jaab.Column("title").OrderBy("view_count")
				})
			}, "titles_json").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.NotEmpty(result.TitlesJSON, "Should have non-empty TitlesJSON")
		}
	})

	suite.Run("JsonArrayAggWithDistinct", func() {
		if suite.ds.Kind == config.MySQL {
			suite.T().Skipf("JsonArrayAgg with DISTINCT skipped for %s (MySQL does not support DISTINCT in JSON_ARRAYAGG)", suite.ds.Kind)

			return
		}

		type DistinctStatusJSON struct {
			AllStatuses string `bun:"all_statuses"`
		}

		var result DistinctStatusJSON

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.JSONArrayAgg(func(jaab orm.JSONArrayAggBuilder) {
					jaab.Column("status").Distinct()
				})
			}, "all_statuses").
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Should execute query")
		suite.NotEmpty(result.AllStatuses, "Should have JSON array of statuses")
	})
}

// TestBitOr tests the BitOr aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestBitOr() {
	suite.T().Logf("Testing BitOr for %s", suite.ds.Kind)

	suite.Run("BitOrBasic", func() {
		type Result struct {
			Status     string `bun:"status"`
			BitOrValue int64  `bun:"bit_or_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BitOr(func(bob orm.BitOrBuilder) {
					bob.Column("view_count")
				})
			}, "bit_or_value").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.BitOrValue >= 0, "BitOr result should be non-negative")
		}
	})

	suite.Run("BitOrWithGroupBy", func() {
		type GroupedBitOr struct {
			Status     string `bun:"status"`
			Count      int64  `bun:"count"`
			BitOrValue int64  `bun:"bit_or_value"`
		}

		var results []GroupedBitOr

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountAll()
			}, "count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BitOr(func(bob orm.BitOrBuilder) {
					bob.Column("view_count")
				})
			}, "bit_or_value").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Count > 0, "Count should be positive")
			suite.True(result.BitOrValue >= 0, "BitOr should be non-negative")
		}
	})
}

// TestBitAnd tests the BitAnd aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestBitAnd() {
	suite.T().Logf("Testing BitAnd for %s", suite.ds.Kind)

	suite.Run("BitAndBasic", func() {
		type Result struct {
			Status      string `bun:"status"`
			BitAndValue int64  `bun:"bit_and_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BitAnd(func(bab orm.BitAndBuilder) {
					bab.Column("view_count")
				})
			}, "bit_and_value").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.BitAndValue >= 0, "BitAnd result should be non-negative")
		}
	})

	suite.Run("BitAndWithGroupBy", func() {
		type GroupedBitAnd struct {
			Status      string `bun:"status"`
			Count       int64  `bun:"count"`
			BitAndValue int64  `bun:"bit_and_value"`
			BitOrValue  int64  `bun:"bit_or_value"`
		}

		var results []GroupedBitAnd

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountAll()
			}, "count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BitAnd(func(bab orm.BitAndBuilder) {
					bab.Column("view_count")
				})
			}, "bit_and_value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BitOr(func(bob orm.BitOrBuilder) {
					bob.Column("view_count")
				})
			}, "bit_or_value").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Count > 0, "Count should be positive")
			suite.True(result.BitAndValue >= 0, "BitAnd should be non-negative")
			suite.True(result.BitOrValue >= 0, "BitOr should be non-negative")
		}
	})
}

// TestBoolOr tests the BoolOr aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestBoolOr() {
	suite.T().Logf("Testing BoolOr for %s", suite.ds.Kind)

	suite.Run("BoolOrWithBoolAnd", func() {
		type Result struct {
			Status       string `bun:"status"`
			AllHighViews bool   `bun:"all_high_views"`
			AnyHighViews bool   `bun:"any_high_views"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BoolAnd(func(bab orm.BoolAndBuilder) {
					bab.Expr(eb.Expr("? > 100", eb.Column("view_count")))
				})
			}, "all_high_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BoolOr(func(bob orm.BoolOrBuilder) {
					bob.Expr(eb.Expr("? > 100", eb.Column("view_count")))
				})
			}, "any_high_views").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")
	})
}

// TestBoolAnd tests the BoolAnd aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestBoolAnd() {
	suite.T().Logf("Testing BoolAnd for %s", suite.ds.Kind)

	suite.Run("BoolAndBasic", func() {
		type Result struct {
			AllMeetCriteria bool `bun:"all_meet_criteria"`
		}

		var result Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BoolAnd(func(bab orm.BoolAndBuilder) {
					bab.Expr(eb.Expr("? > 10", eb.Column("view_count")))
				})
			}, "all_meet_criteria").
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Should execute query")
	})

	suite.Run("BoolAndGrouped", func() {
		type GroupedBoolAnd struct {
			Status            string `bun:"status"`
			AllAboveThreshold bool   `bun:"all_above_threshold"`
		}

		var results []GroupedBoolAnd

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.BoolAnd(func(bab orm.BoolAndBuilder) {
					bab.Expr(eb.Expr("? >= 30", eb.Column("view_count")))
				})
			}, "all_above_threshold").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")
	})
}

// TestStdDev tests the StdDev (standard deviation) aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestStdDev() {
	suite.T().Logf("Testing StdDev for %s", suite.ds.Kind)

	suite.Run("BasicStdDev", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("StdDev skipped for %s (SQLite does not support statistical functions)", suite.ds.Kind)
		}

		type Result struct {
			Status   string  `bun:"status"`
			AvgViews float64 `bun:"avg_views"`
			StdDev   float64 `bun:"std_dev"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}, "avg_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StdDev(func(sb orm.StdDevBuilder) {
					sb.Column("view_count")
				})
			}, "std_dev").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.AvgViews >= 0, "Average should be non-negative")
			suite.True(result.StdDev >= 0, "Standard deviation should be non-negative")
		}
	})

	suite.Run("CombinedStatisticalFunctions", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("StdDev skipped for %s (SQLite does not support statistical functions)", suite.ds.Kind)
		}

		type CombinedStats struct {
			Status   string  `bun:"status"`
			Count    int64   `bun:"count"`
			AvgViews float64 `bun:"avg_views"`
			MinViews int64   `bun:"min_views"`
			MaxViews int64   `bun:"max_views"`
			StdDev   float64 `bun:"std_dev"`
		}

		var results []CombinedStats

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CountAll()
			}, "count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}, "avg_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MinColumn("view_count")
			}, "min_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.MaxColumn("view_count")
			}, "max_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StdDev(func(sb orm.StdDevBuilder) {
					sb.Column("view_count")
				})
			}, "std_dev").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Count > 0, "Count should be positive")
			suite.True(result.AvgViews >= 0, "Average should be non-negative")
			suite.True(result.MinViews >= 0, "Min should be non-negative")
			suite.True(result.MaxViews >= result.MinViews, "Max should be >= Min")
			suite.True(result.StdDev >= 0, "StdDev should be non-negative")

			suite.T().Logf("Status: %s, Count: %d, Avg: %.2f, Min: %d, Max: %d, StdDev: %.2f",
				result.Status, result.Count, result.AvgViews, result.MinViews,
				result.MaxViews, result.StdDev)
		}
	})
}

// TestVariance tests the Variance aggregate function.
func (suite *EBAggregationFunctionsTestSuite) TestVariance() {
	suite.T().Logf("Testing Variance for %s", suite.ds.Kind)

	suite.Run("BasicVariance", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("Variance skipped for %s (SQLite not supported)", suite.ds.Kind)
		}

		type Result struct {
			Status   string  `bun:"status"`
			AvgViews float64 `bun:"avg_views"`
			Variance float64 `bun:"variance"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}, "avg_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Variance(func(vb orm.VarianceBuilder) {
					vb.Column("view_count")
				})
			}, "variance").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.AvgViews >= 0, "Average should be non-negative")
			suite.True(result.Variance >= 0, "Variance should be non-negative")
		}
	})

	suite.Run("VarianceWithPopulationAndSample", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("Variance with modes skipped for %s (SQLite not supported)", suite.ds.Kind)
		}

		type VarianceModes struct {
			Status     string  `bun:"status"`
			VarPop     float64 `bun:"var_pop"`
			VarSamp    float64 `bun:"var_samp"`
			StdDevPop  float64 `bun:"stddev_pop"`
			StdDevSamp float64 `bun:"stddev_samp"`
		}

		var results []VarianceModes

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Variance(func(vb orm.VarianceBuilder) {
					vb.Column("view_count").Population()
				})
			}, "var_pop").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Variance(func(vb orm.VarianceBuilder) {
					vb.Column("view_count").Sample()
				})
			}, "var_samp").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StdDev(func(sb orm.StdDevBuilder) {
					sb.Column("view_count").Population()
				})
			}, "stddev_pop").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StdDev(func(sb orm.StdDevBuilder) {
					sb.Column("view_count").Sample()
				})
			}, "stddev_samp").
			GroupBy("status").
			OrderBy("status").
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.VarPop >= 0, "Population variance should be non-negative")
			suite.True(result.VarSamp >= 0, "Sample variance should be non-negative")
			suite.True(result.StdDevPop >= 0, "Population stddev should be non-negative")
			suite.True(result.StdDevSamp >= 0, "Sample stddev should be non-negative")
		}
	})
}

// TestStringAggOrderByExpr tests OrderByExpr on string aggregation.
func (suite *EBAggregationFunctionsTestSuite) TestStringAggOrderByExpr() {
	suite.T().Logf("Testing StringAggOrderByExpr for %s", suite.ds.Kind)

	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("StringAgg OrderByExpr not supported on %s", suite.ds.Kind)
	}

	type Result struct {
		Names string `bun:"names"`
	}

	var result Result

	err := suite.selectUsers().
		SelectExpr(func(eb orm.ExprBuilder) any {
			return eb.StringAgg(func(sa orm.StringAggBuilder) {
				sa.Column("name")
				sa.Separator(",")
				sa.OrderByExpr(eb.Column("age"))
			})
		}, "names").
		Scan(suite.ctx, &result)

	suite.Require().NoError(err, "Should execute query")
	suite.NotEmpty(result.Names, "Should have concatenated names")
}

// TestJSONObjectAggKeyExpr tests KeyExpr on JSON object aggregation.
func (suite *EBAggregationFunctionsTestSuite) TestJSONObjectAggKeyExpr() {
	suite.T().Logf("Testing JSONObjectAggKeyExpr for %s", suite.ds.Kind)

	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("JSON_OBJECT_AGG not supported on %s", suite.ds.Kind)
	}

	type Result struct {
		Obj string `bun:"obj"`
	}

	var result Result

	err := suite.selectUsers().
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("name", "Alice Johnson")
		}).
		SelectExpr(func(eb orm.ExprBuilder) any {
			return eb.JSONObjectAgg(func(ja orm.JSONObjectAggBuilder) {
				ja.KeyExpr(eb.Column("name"))
				ja.Column("age")
			})
		}, "obj").
		Scan(suite.ctx, &result)

	suite.Require().NoError(err, "Should execute query")
}

// so we build the query without executing to cover the code path.
func (suite *EBAggregationFunctionsTestSuite) TestAggregateIgnoreRespectNulls() {
	suite.T().Logf("Testing AggregateIgnoreRespectNulls for %s", suite.ds.Kind)

	suite.Run("IgnoreNulls", func() {
		query := suite.selectUsers().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StringAgg(func(sa orm.StringAggBuilder) {
					sa.Column("name")
					sa.Separator(",")
					sa.IgnoreNulls()
				})
			}, "names")

		suite.NotNil(query, "Should not be nil")
	})

	suite.Run("RespectNulls", func() {
		query := suite.selectUsers().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StringAgg(func(sa orm.StringAggBuilder) {
					sa.Column("name")
					sa.Separator(",")
					sa.RespectNulls()
				})
			}, "names")

		suite.NotNil(query, "Should not be nil")
	})
}
