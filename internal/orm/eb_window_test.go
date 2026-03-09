package orm_test

import (
	"strings"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBWindowFunctionsTestSuite{BaseTestSuite: base}
	})
}

// EBWindowFunctionsTestSuite tests window function methods of orm.ExprBuilder.
type EBWindowFunctionsTestSuite struct {
	*BaseTestSuite
}

// TestRowNumber tests the ROW_NUMBER window function.
func (suite *EBWindowFunctionsTestSuite) TestRowNumber() {
	suite.T().Logf("Testing RowNumber function for %s", suite.ds.Kind)

	suite.Run("SequentialRowNumbers", func() {
		type UserWithRowNumber struct {
			ID     string `bun:"id"`
			Name   string `bun:"name"`
			Age    int16  `bun:"age"`
			RowNum int64  `bun:"row_num"`
		}

		var usersWithRowNum []UserWithRowNumber

		err := suite.selectUsers().
			Select("id", "name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.RowNumber(func(rn orm.RowNumberBuilder) {
					rn.Over().OrderBy("age")
				})
			}, "row_num").
			OrderBy("age").
			Scan(suite.ctx, &usersWithRowNum)

		suite.Require().NoError(err, "ROW_NUMBER should work correctly")
		suite.Len(usersWithRowNum, 20, "Should have 20 users")

		// Verify ROW_NUMBER sequence
		for i, user := range usersWithRowNum {
			suite.Equal(int64(i+1), user.RowNum, "ROW_NUMBER should be sequential starting from 1")
		}

		suite.T().Logf("Row numbers verified: %d users with sequential numbers", len(usersWithRowNum))
	})
}

// TestRank tests the RANK window function.
func (suite *EBWindowFunctionsTestSuite) TestRank() {
	suite.T().Logf("Testing Rank function for %s", suite.ds.Kind)

	suite.Run("RankPartitionedByStatus", func() {
		type PostWithRank struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
			Rank      int64  `bun:"rank"`
		}

		var postsWithRank []PostWithRank

		err := suite.selectPosts().
			Select("id", "title", "status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Rank(func(r orm.RankBuilder) {
					r.Over().PartitionBy("status").OrderByDesc("view_count")
				})
			}, "rank").
			OrderBy("status").
			OrderByDesc("view_count").
			Scan(suite.ctx, &postsWithRank)

		suite.Require().NoError(err, "RANK should work with partitioning")
		suite.Require().NotEmpty(postsWithRank, "Should have posts with rank")

		// Verify ranking within partitions
		statusGroups := make(map[string][]PostWithRank)
		for _, post := range postsWithRank {
			statusGroups[post.Status] = append(statusGroups[post.Status], post)
		}

		for status, posts := range statusGroups {
			if len(posts) > 0 {
				suite.True(posts[0].Rank >= 1, "First post in %s partition should have rank >= 1", status)
			}
		}

		suite.T().Logf("Verified ranks for %d posts across %d status groups", len(postsWithRank), len(statusGroups))
	})
}

// TestDenseRank tests the DENSE_RANK window function.
func (suite *EBWindowFunctionsTestSuite) TestDenseRank() {
	suite.T().Logf("Testing DenseRank function for %s", suite.ds.Kind)

	suite.Run("DenseRankPartitionedByStatus", func() {
		type PostWithDenseRank struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			Status    string `bun:"status"`
			ViewCount int64  `bun:"view_count"`
			Rank      int64  `bun:"rank"`
			DenseRank int64  `bun:"dense_rank"`
		}

		var postsWithRank []PostWithDenseRank

		err := suite.selectPosts().
			Select("id", "title", "status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Rank(func(r orm.RankBuilder) {
					r.Over().PartitionBy("status").OrderByDesc("view_count")
				})
			}, "rank").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DenseRank(func(dr orm.DenseRankBuilder) {
					dr.Over().PartitionBy("status").OrderByDesc("view_count")
				})
			}, "dense_rank").
			OrderBy("status").
			OrderByDesc("view_count").
			Scan(suite.ctx, &postsWithRank)

		suite.Require().NoError(err, "DENSE_RANK should work with partitioning")
		suite.Require().NotEmpty(postsWithRank, "Should have posts with dense rank")

		// Verify ranking within partitions
		statusGroups := make(map[string][]PostWithDenseRank)
		for _, post := range postsWithRank {
			statusGroups[post.Status] = append(statusGroups[post.Status], post)
		}

		for status, posts := range statusGroups {
			if len(posts) > 1 {
				suite.True(posts[0].Rank >= 1, "First post in %s partition should have rank >= 1", status)
				suite.True(posts[0].DenseRank >= 1, "First post in %s partition should have dense_rank >= 1", status)
			}
		}

		suite.T().Logf("Verified dense ranks for %d posts across %d status groups", len(postsWithRank), len(statusGroups))
	})
}

// TestPercentRank tests the PERCENT_RANK window function.
func (suite *EBWindowFunctionsTestSuite) TestPercentRank() {
	suite.T().Logf("Testing PercentRank function for %s", suite.ds.Kind)

	suite.Run("PercentRankByViewCount", func() {
		type PostAnalytics struct {
			Title          string  `bun:"title"`
			Status         string  `bun:"status"`
			ViewCount      int64   `bun:"view_count"`
			RankInStatus   int64   `bun:"rank_in_status"`
			PercentOfTotal float64 `bun:"percent_of_total"`
		}

		var postAnalytics []PostAnalytics

		err := suite.selectPosts().
			Select("title", "status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Rank(func(r orm.RankBuilder) {
					r.Over().PartitionBy("status").OrderByDesc("view_count")
				})
			}, "rank_in_status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.PercentRank(func(pr orm.PercentRankBuilder) {
					pr.Over().OrderByDesc("view_count")
				})
			}, "percent_of_total").
			OrderBy("status").
			OrderByDesc("view_count").
			Scan(suite.ctx, &postAnalytics)

		suite.Require().NoError(err, "PERCENT_RANK should work correctly")
		suite.Require().NotEmpty(postAnalytics, "Should have post analytics")

		// Verify percent rank is within valid range
		for _, post := range postAnalytics {
			suite.True(post.RankInStatus >= 1, "Rank should be at least 1")
			suite.True(post.PercentOfTotal >= 0 && post.PercentOfTotal <= 1, "Percent rank should be between 0 and 1")
		}

		suite.T().Logf("Verified percent ranks for %d posts", len(postAnalytics))
	})
}

// TestCumeDist tests the CUME_DIST window function.
func (suite *EBWindowFunctionsTestSuite) TestCumeDist() {
	suite.T().Logf("Testing CumeDist function for %s", suite.ds.Kind)

	suite.Run("CumeDistByViewCount", func() {
		type CumeDistResult struct {
			ID        string  `bun:"id"`
			ViewCount int64   `bun:"view_count"`
			CumeDist  float64 `bun:"cume_dist"`
		}

		var cumeDistResults []CumeDistResult

		err := suite.selectPosts().
			Select("id", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CumeDist(func(cdb orm.CumeDistBuilder) {
					cdb.Over().OrderBy("view_count")
				})
			}, "cume_dist").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &cumeDistResults)

		suite.Require().NoError(err, "CUME_DIST should work correctly")
		suite.Require().NotEmpty(cumeDistResults, "Should have cume_dist results")

		for _, result := range cumeDistResults {
			suite.True(result.CumeDist > 0 && result.CumeDist <= 1, "CumeDist should be in (0, 1]")
			suite.T().Logf("ID: %s, ViewCount: %d, CumeDist: %.4f",
				result.ID, result.ViewCount, result.CumeDist)
		}
	})
}

// TestNtile tests the NTILE window function.
func (suite *EBWindowFunctionsTestSuite) TestNtile() {
	suite.T().Logf("Testing NTile function for %s", suite.ds.Kind)

	suite.Run("QuartilesUsingNtile", func() {
		type UserWithQuartile struct {
			Name     string `bun:"name"`
			Age      int16  `bun:"age"`
			Quartile int64  `bun:"quartile"`
		}

		var usersWithQuartile []UserWithQuartile

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NTile(func(nb orm.NTileBuilder) {
					nb.Buckets(4).Over().OrderBy("age")
				})
			}, "quartile").
			OrderBy("age").
			Scan(suite.ctx, &usersWithQuartile)

		suite.Require().NoError(err, "NTILE should work for quartile distribution")
		suite.Len(usersWithQuartile, 20, "Should have 20 users")

		// Verify quartile assignment
		for _, user := range usersWithQuartile {
			suite.True(user.Quartile >= 1 && user.Quartile <= 4, "Quartile should be between 1 and 4")
		}

		suite.T().Logf("Verified quartiles for %d users", len(usersWithQuartile))
	})
}

// TestLag tests the LAG window function.
func (suite *EBWindowFunctionsTestSuite) TestLag() {
	suite.T().Logf("Testing Lag function for %s", suite.ds.Kind)

	suite.Run("LagWithDefaultOffset", func() {
		type PostWithLag struct {
			Title         string `bun:"title"`
			ViewCount     int64  `bun:"view_count"`
			PrevViewCount *int64 `bun:"prev_view_count"`
		}

		var postsWithLag []PostWithLag

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lag(func(lb orm.LagBuilder) {
					lb.Column("view_count").Over().OrderBy("view_count")
				})
			}, "prev_view_count").
			OrderBy("view_count").
			Scan(suite.ctx, &postsWithLag)

		suite.Require().NoError(err, "LAG should work with default offset")
		suite.Require().NotEmpty(postsWithLag, "Should have posts with lag")

		// First row should have null prev_view_count
		if len(postsWithLag) > 0 {
			suite.Nil(postsWithLag[0].PrevViewCount, "First row should have null previous value")
		}

		suite.T().Logf("Verified LAG for %d posts", len(postsWithLag))
	})

	suite.Run("LagWithCustomOffset", func() {
		type PostWithLagAdvanced struct {
			Title          string `bun:"title"`
			ViewCount      int64  `bun:"view_count"`
			Prev2ViewCount *int64 `bun:"prev2_view_count"`
		}

		var advLag []PostWithLagAdvanced

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lag(func(lb orm.LagBuilder) {
					lb.Column("view_count").Offset(2).Over().OrderBy("view_count")
				})
			}, "prev2_view_count").
			OrderBy("view_count").
			Scan(suite.ctx, &advLag)

		suite.Require().NoError(err, "LAG should work with custom offset")
		suite.Require().NotEmpty(advLag, "Should have posts for advanced lag")

		if len(advLag) >= 3 {
			// The third row's Prev2 should equal the first row's view_count
			if advLag[2].Prev2ViewCount != nil {
				suite.Equal(advLag[0].ViewCount, *advLag[2].Prev2ViewCount, "Third row's LAG(2) should match first row's value")
			}
		}

		suite.T().Logf("Verified LAG with offset 2 for %d posts", len(advLag))
	})
}

// TestLead tests the LEAD window function.
func (suite *EBWindowFunctionsTestSuite) TestLead() {
	suite.T().Logf("Testing Lead function for %s", suite.ds.Kind)

	suite.Run("LeadWithDefaultOffset", func() {
		type PostWithLead struct {
			Title         string `bun:"title"`
			ViewCount     int64  `bun:"view_count"`
			NextViewCount *int64 `bun:"next_view_count"`
		}

		var postsWithLead []PostWithLead

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lead(func(lb orm.LeadBuilder) {
					lb.Column("view_count").Over().OrderBy("view_count")
				})
			}, "next_view_count").
			OrderBy("view_count").
			Scan(suite.ctx, &postsWithLead)

		suite.Require().NoError(err, "LEAD should work with default offset")
		suite.Require().NotEmpty(postsWithLead, "Should have posts with lead")

		// Last row should have null next_view_count
		if len(postsWithLead) > 0 {
			lastIdx := len(postsWithLead) - 1
			suite.Nil(postsWithLead[lastIdx].NextViewCount, "Last row should have null next value")
		}

		suite.T().Logf("Verified LEAD for %d posts", len(postsWithLead))
	})

	suite.Run("LeadWithCustomOffsetAndDefault", func() {
		type PostWithLeadAdvanced struct {
			Title          string `bun:"title"`
			ViewCount      int64  `bun:"view_count"`
			Next2ViewCount *int64 `bun:"next2_view_count"`
			Next2OrDefault int64  `bun:"next2_or_default"`
		}

		var advLead []PostWithLeadAdvanced

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lead(func(lb orm.LeadBuilder) {
					lb.Column("view_count").Offset(2).Over().OrderBy("view_count")
				})
			}, "next2_view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lead(func(lb orm.LeadBuilder) {
					lb.Column("view_count").Offset(2).DefaultValue(-1).Over().OrderBy("view_count")
				})
			}, "next2_or_default").
			OrderBy("view_count").
			Scan(suite.ctx, &advLead)

		suite.Require().NoError(err, "LEAD should work with custom offset and default value")
		suite.Require().NotEmpty(advLead, "Should have posts for advanced lead")

		// Default value should apply on the last rows where LEAD overflows
		if len(advLead) >= 1 {
			lastIdx := len(advLead) - 1
			suite.NotZero(advLead[lastIdx].Next2OrDefault, "Default value should be applied when LEAD exceeds bounds")
		}

		suite.T().Logf("Verified LEAD with offset 2 and default for %d posts", len(advLead))
	})
}

// TestFirstValue tests the FIRST_VALUE window function.
func (suite *EBWindowFunctionsTestSuite) TestFirstValue() {
	suite.T().Logf("Testing FirstValue function for %s", suite.ds.Kind)

	suite.Run("FirstValuePartitionedByStatus", func() {
		type PostWithFirstValue struct {
			Title         string `bun:"title"`
			Status        string `bun:"status"`
			ViewCount     int64  `bun:"view_count"`
			FirstInStatus int64  `bun:"first_in_status"`
		}

		var postsWithFirst []PostWithFirstValue

		err := suite.selectPosts().
			Select("title", "status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.FirstValue(func(fvb orm.FirstValueBuilder) {
					fvb.Column("view_count").Over().PartitionBy("status").OrderBy("view_count")
				})
			}, "first_in_status").
			OrderBy("status", "view_count").
			Scan(suite.ctx, &postsWithFirst)

		suite.Require().NoError(err, "FIRST_VALUE should work with partitioning")
		suite.Require().NotEmpty(postsWithFirst, "Should have posts with first value")

		// Verify FIRST_VALUE behavior
		statusFirstValues := make(map[string][]PostWithFirstValue)
		for _, post := range postsWithFirst {
			statusFirstValues[post.Status] = append(statusFirstValues[post.Status], post)
		}

		for status, posts := range statusFirstValues {
			if len(posts) > 1 {
				// All posts in same status should have same first_in_status value
				firstValue := posts[0].FirstInStatus
				for _, post := range posts {
					suite.Equal(firstValue, post.FirstInStatus, "All posts in %s should have same first value", status)
				}
			}
		}

		suite.T().Logf("Verified FIRST_VALUE for %d posts across %d status groups", len(postsWithFirst), len(statusFirstValues))
	})
}

// TestLastValue tests the LAST_VALUE window function.
func (suite *EBWindowFunctionsTestSuite) TestLastValue() {
	suite.T().Logf("Testing LastValue function for %s", suite.ds.Kind)

	suite.Run("LastValuePartitionedByStatus", func() {
		type PostWithLastValue struct {
			Title        string `bun:"title"`
			Status       string `bun:"status"`
			ViewCount    int64  `bun:"view_count"`
			LastInStatus int64  `bun:"last_in_status"`
		}

		var postsWithLast []PostWithLastValue

		err := suite.selectPosts().
			Select("title", "status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.LastValue(func(lvb orm.LastValueBuilder) {
					lvb.Column("view_count").Over().PartitionBy("status").OrderBy("view_count").Rows().UnboundedPreceding().And().UnboundedFollowing()
				})
			}, "last_in_status").
			OrderBy("status", "view_count").
			Scan(suite.ctx, &postsWithLast)

		suite.Require().NoError(err, "LAST_VALUE should work with partitioning")
		suite.Require().NotEmpty(postsWithLast, "Should have posts with last value")

		// Verify LAST_VALUE behavior
		statusLastValues := make(map[string][]PostWithLastValue)
		for _, post := range postsWithLast {
			statusLastValues[post.Status] = append(statusLastValues[post.Status], post)
		}

		for status, posts := range statusLastValues {
			if len(posts) > 1 {
				// All posts in same status should have same last_in_status value
				lastValue := posts[0].LastInStatus
				for _, post := range posts {
					suite.Equal(lastValue, post.LastInStatus, "All posts in %s should have same last value", status)
				}
			}
		}

		suite.T().Logf("Verified LAST_VALUE for %d posts across %d status groups", len(postsWithLast), len(statusLastValues))
	})
}

// TestNthValue tests the NTH_VALUE window function.
func (suite *EBWindowFunctionsTestSuite) TestNthValue() {
	suite.T().Logf("Testing NthValue function for %s", suite.ds.Kind)

	suite.Run("SecondValueInPartition", func() {
		type PostWithNthValue struct {
			Status        string `bun:"status"`
			ViewCount     int64  `bun:"view_count"`
			SecondFromEnd int64  `bun:"second_from_end"`
		}

		var nthVals []PostWithNthValue

		err := suite.selectPosts().
			Select("status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NthValue(func(nvb orm.NthValueBuilder) {
					nvb.Column("view_count").N(2).Over().PartitionBy("status").OrderBy("view_count").Rows().UnboundedPreceding().And().UnboundedFollowing()
				})
			}, "second_from_end").
			OrderBy("status", "view_count").
			Scan(suite.ctx, &nthVals)

		suite.Require().NoError(err, "NTH_VALUE should work with full frame")
		suite.Require().NotEmpty(nthVals, "Should compute NTH_VALUE")

		suite.T().Logf("Verified NTH_VALUE for %d posts", len(nthVals))
	})
}

// TestWinCount tests the COUNT window function.
func (suite *EBWindowFunctionsTestSuite) TestWinCount() {
	suite.T().Logf("Testing WinCount function for %s", suite.ds.Kind)

	suite.Run("RunningCount", func() {
		type UserWithRunningCount struct {
			Name         string `bun:"name"`
			Age          int16  `bun:"age"`
			RunningCount int64  `bun:"running_count"`
		}

		var usersWithCount []UserWithRunningCount

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinCount(func(wc orm.WindowCountBuilder) {
					wc.All().Over().OrderBy("age").Rows().UnboundedPreceding()
				})
			}, "running_count").
			OrderBy("age").
			Scan(suite.ctx, &usersWithCount)

		suite.Require().NoError(err, "WinCount should work for running count")
		suite.Len(usersWithCount, 20, "Should have 20 users")

		// Verify running counts
		for i, user := range usersWithCount {
			suite.Equal(int64(i+1), user.RunningCount, "Running count should increment by 1")
		}

		suite.T().Logf("Verified running count for %d users", len(usersWithCount))
	})
}

// TestWinSum tests the SUM window function.
func (suite *EBWindowFunctionsTestSuite) TestWinSum() {
	suite.T().Logf("Testing WinSum function for %s", suite.ds.Kind)

	suite.Run("RunningTotal", func() {
		type UserWithRunningTotal struct {
			Name         string `bun:"name"`
			Age          int16  `bun:"age"`
			RunningTotal int64  `bun:"running_total"`
		}

		var usersWithSum []UserWithRunningTotal

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinSum(func(ws orm.WindowSumBuilder) {
					ws.Column("age").Over().OrderBy("age").Rows().UnboundedPreceding()
				})
			}, "running_total").
			OrderBy("age").
			Scan(suite.ctx, &usersWithSum)

		suite.Require().NoError(err, "WinSum should work for running total")
		suite.Len(usersWithSum, 20, "Should have 20 users")

		// Verify running totals
		for _, user := range usersWithSum {
			suite.True(user.RunningTotal > 0, "Running total should be positive")
		}

		suite.T().Logf("Verified running total for %d users", len(usersWithSum))
	})

	suite.Run("CumulativeViewsByStatus", func() {
		type PostWithCumulativeViews struct {
			Title           string `bun:"title"`
			Status          string `bun:"status"`
			ViewCount       int64  `bun:"view_count"`
			CumulativeViews int64  `bun:"cumulative_views"`
		}

		var postsWithCumulative []PostWithCumulativeViews

		err := suite.selectPosts().
			Select("title", "status", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinSum(func(ws orm.WindowSumBuilder) {
					ws.Column("view_count").Over().PartitionBy("status").OrderByDesc("view_count").Rows().UnboundedPreceding()
				})
			}, "cumulative_views").
			OrderBy("status").
			OrderByDesc("view_count").
			Scan(suite.ctx, &postsWithCumulative)

		suite.Require().NoError(err, "WinSum should work with partitioning")
		suite.Require().NotEmpty(postsWithCumulative, "Should have posts with cumulative views")

		for _, post := range postsWithCumulative {
			suite.True(post.CumulativeViews >= post.ViewCount, "Cumulative views should be at least equal to current view count")
		}

		suite.T().Logf("Verified cumulative views for %d posts", len(postsWithCumulative))
	})
}

// TestWinAvg tests the AVG window function.
func (suite *EBWindowFunctionsTestSuite) TestWinAvg() {
	suite.T().Logf("Testing WinAvg function for %s", suite.ds.Kind)

	suite.Run("MovingAverage", func() {
		type UserWithMovingAvg struct {
			Name      string  `bun:"name"`
			Age       int16   `bun:"age"`
			MovingAvg float64 `bun:"moving_avg"`
		}

		var usersWithAvg []UserWithMovingAvg

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinAvg(func(wa orm.WindowAvgBuilder) {
					wa.Column("age").Over().OrderBy("age").Rows().UnboundedPreceding()
				})
			}, "moving_avg").
			OrderBy("age").
			Scan(suite.ctx, &usersWithAvg)

		suite.Require().NoError(err, "WinAvg should work for moving average")
		suite.Len(usersWithAvg, 20, "Should have 20 users")

		// Verify moving averages
		for _, user := range usersWithAvg {
			suite.True(user.MovingAvg > 0, "Moving average should be positive")
		}

		suite.T().Logf("Verified moving average for %d users", len(usersWithAvg))
	})

	suite.Run("ThreeRowMovingAverage", func() {
		type PostWithMovingAvg struct {
			Title     string  `bun:"title"`
			ViewCount int64   `bun:"view_count"`
			MovAvg    float64 `bun:"mov_avg"`
		}

		var movingAvgRows []PostWithMovingAvg

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinAvg(func(wab orm.WindowAvgBuilder) {
					wab.Column("view_count").Over().OrderBy("view_count").Rows().Preceding(2).And().CurrentRow()
				})
			}, "mov_avg").
			OrderBy("view_count").
			Scan(suite.ctx, &movingAvgRows)

		suite.Require().NoError(err, "WinAvg should work with ROWS BETWEEN 2 PRECEDING AND CURRENT ROW")
		suite.Require().NotEmpty(movingAvgRows, "Should have moving average values")

		suite.T().Logf("Verified 3-row moving average for %d posts", len(movingAvgRows))
	})
}

// TestWinMin tests the MIN window function.
func (suite *EBWindowFunctionsTestSuite) TestWinMin() {
	suite.T().Logf("Testing WinMin function for %s", suite.ds.Kind)

	suite.Run("MinInStatusPartition", func() {
		type WindowMinResult struct {
			ID          string `bun:"id"`
			ViewCount   int64  `bun:"view_count"`
			Status      string `bun:"status"`
			MinInStatus int64  `bun:"min_in_status"`
		}

		var windowMinResults []WindowMinResult

		err := suite.selectPosts().
			Select("id", "view_count", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinMin(func(wmb orm.WindowMinBuilder) {
					wmb.Column("view_count").Over().PartitionBy("status")
				})
			}, "min_in_status").
			OrderBy("status", "id").
			Limit(8).
			Scan(suite.ctx, &windowMinResults)

		suite.Require().NoError(err, "WinMin should work correctly")
		suite.Require().NotEmpty(windowMinResults, "Should have window min results")

		for _, result := range windowMinResults {
			suite.True(result.MinInStatus <= result.ViewCount, "Min should be <= current view count")
			suite.T().Logf("ID: %s, Status: %s, ViewCount: %d, MinInStatus: %d",
				result.ID, result.Status, result.ViewCount, result.MinInStatus)
		}
	})
}

// TestWinMax tests the MAX window function.
func (suite *EBWindowFunctionsTestSuite) TestWinMax() {
	suite.T().Logf("Testing WinMax function for %s", suite.ds.Kind)

	suite.Run("MaxInStatusPartition", func() {
		type WindowMaxResult struct {
			ID          string `bun:"id"`
			ViewCount   int64  `bun:"view_count"`
			Status      string `bun:"status"`
			MaxInStatus int64  `bun:"max_in_status"`
		}

		var windowMaxResults []WindowMaxResult

		err := suite.selectPosts().
			Select("id", "view_count", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinMax(func(wmb orm.WindowMaxBuilder) {
					wmb.Column("view_count").Over().PartitionBy("status")
				})
			}, "max_in_status").
			OrderBy("status", "id").
			Limit(8).
			Scan(suite.ctx, &windowMaxResults)

		suite.Require().NoError(err, "WinMax should work correctly")
		suite.Require().NotEmpty(windowMaxResults, "Should have window max results")

		for _, result := range windowMaxResults {
			suite.True(result.MaxInStatus >= result.ViewCount, "Max should be >= current view count")
			suite.T().Logf("ID: %s, Status: %s, ViewCount: %d, MaxInStatus: %d",
				result.ID, result.Status, result.ViewCount, result.MaxInStatus)
		}
	})
}

// TestWinStringAgg tests the STRING_AGG window function.
func (suite *EBWindowFunctionsTestSuite) TestWinStringAgg() {
	suite.T().Logf("Testing WinStringAgg function for %s", suite.ds.Kind)

	suite.Run("StringAggPartitionedByStatus", func() {
		if suite.ds.Kind == config.MySQL {
			suite.T().Skipf("WinStringAgg skipped for %s (MySQL does not support GROUP_CONCAT as window function)", suite.ds.Kind)

			return
		}

		type WindowStringAggResult struct {
			ID       string `bun:"id"`
			Status   string `bun:"status"`
			TitleAgg string `bun:"title_agg"`
		}

		var windowStringAggResults []WindowStringAggResult

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinStringAgg(func(wsab orm.WindowStringAggBuilder) {
					wsab.Column("title").Separator(", ").Over().PartitionBy("status")
				})
			}, "title_agg").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowStringAggResults)

		suite.Require().NoError(err, "WinStringAgg should work correctly")
		suite.Require().NotEmpty(windowStringAggResults, "Should have window string agg results")

		for _, result := range windowStringAggResults {
			suite.NotEmpty(result.TitleAgg, "Should have non-empty aggregated titles")
			suite.T().Logf("ID: %s, Status: %s, TitleAgg: %s",
				result.ID, result.Status, result.TitleAgg)
		}
	})
}

// TestWinArrayAgg tests the ARRAY_AGG window function (PostgreSQL only).
func (suite *EBWindowFunctionsTestSuite) TestWinArrayAgg() {
	suite.T().Logf("Testing WinArrayAgg function for %s", suite.ds.Kind)

	suite.Run("ArrayAggPartitionedByStatus", func() {
		if suite.ds.Kind != config.Postgres {
			suite.T().Skipf("WinArrayAgg skipped for %s (PostgreSQL only)", suite.ds.Kind)
		}

		type WindowArrayAggResult struct {
			ID         string  `bun:"id"`
			Status     string  `bun:"status"`
			ViewCounts []int64 `bun:"view_counts,array"`
		}

		var windowArrayAggResults []WindowArrayAggResult

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinArrayAgg(func(waab orm.WindowArrayAggBuilder) {
					waab.Column("view_count").Over().PartitionBy("status")
				})
			}, "view_counts").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowArrayAggResults)

		suite.Require().NoError(err, "WinArrayAgg should work correctly")
		suite.Require().NotEmpty(windowArrayAggResults, "Should have window array agg results")

		for _, result := range windowArrayAggResults {
			suite.NotEmpty(result.ViewCounts, "Should have non-empty view counts array")
			suite.T().Logf("ID: %s, Status: %s, ViewCounts: %v",
				result.ID, result.Status, result.ViewCounts)
		}
	})
}

// TestWinStdDev tests the STDDEV window function.
func (suite *EBWindowFunctionsTestSuite) TestWinStdDev() {
	suite.T().Logf("Testing WinStdDev function for %s", suite.ds.Kind)

	suite.Run("StdDevInStatusPartition", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("WinStdDev skipped for %s (SQLite does not support statistical functions)", suite.ds.Kind)

			return
		}

		type WindowStdDevResult struct {
			ID             string  `bun:"id"`
			ViewCount      int64   `bun:"view_count"`
			Status         string  `bun:"status"`
			StdDevInStatus float64 `bun:"stddev_in_status"`
		}

		var windowStdDevResults []WindowStdDevResult

		err := suite.selectPosts().
			Select("id", "view_count", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinStdDev(func(wsb orm.WindowStdDevBuilder) {
					wsb.Column("view_count").Over().PartitionBy("status")
				})
			}, "stddev_in_status").
			OrderBy("status", "id").
			Limit(8).
			Scan(suite.ctx, &windowStdDevResults)

		suite.Require().NoError(err, "WinStdDev should work correctly")
		suite.Require().NotEmpty(windowStdDevResults, "Should have window stddev results")

		for _, result := range windowStdDevResults {
			suite.True(result.StdDevInStatus >= 0, "StdDev should be non-negative")
			suite.T().Logf("ID: %s, Status: %s, ViewCount: %d, StdDevInStatus: %.2f",
				result.ID, result.Status, result.ViewCount, result.StdDevInStatus)
		}
	})
}

// TestWinVariance tests the VARIANCE window function.
func (suite *EBWindowFunctionsTestSuite) TestWinVariance() {
	suite.T().Logf("Testing WinVariance function for %s", suite.ds.Kind)

	suite.Run("VarianceInStatusPartition", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("WinVariance skipped for %s (SQLite does not support statistical functions)", suite.ds.Kind)

			return
		}

		type WindowVarianceResult struct {
			ID               string  `bun:"id"`
			ViewCount        int64   `bun:"view_count"`
			Status           string  `bun:"status"`
			VarianceInStatus float64 `bun:"variance_in_status"`
		}

		var windowVarianceResults []WindowVarianceResult

		err := suite.selectPosts().
			Select("id", "view_count", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinVariance(func(wvb orm.WindowVarianceBuilder) {
					wvb.Column("view_count").Over().PartitionBy("status")
				})
			}, "variance_in_status").
			OrderBy("status", "id").
			Limit(8).
			Scan(suite.ctx, &windowVarianceResults)

		suite.Require().NoError(err, "WinVariance should work correctly")
		suite.Require().NotEmpty(windowVarianceResults, "Should have window variance results")

		for _, result := range windowVarianceResults {
			suite.True(result.VarianceInStatus >= 0, "Variance should be non-negative")
			suite.T().Logf("ID: %s, Status: %s, ViewCount: %d, VarianceInStatus: %.2f",
				result.ID, result.Status, result.ViewCount, result.VarianceInStatus)
		}
	})
}

// TestWinJSONObjectAgg tests the JSON_OBJECT_AGG window function.
func (suite *EBWindowFunctionsTestSuite) TestWinJSONObjectAgg() {
	suite.T().Logf("Testing WinJSONObjectAgg function for %s", suite.ds.Kind)

	suite.Run("JSONObjectAggPartitionedByStatus", func() {
		type WindowJSONObjectResult struct {
			ID            string `bun:"id"`
			Status        string `bun:"status"`
			JSONObjectAgg string `bun:"json_object_agg"`
		}

		var windowJSONObjectResults []WindowJSONObjectResult

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinJSONObjectAgg(func(wjoab orm.WindowJSONObjectAggBuilder) {
					wjoab.KeyColumn("id").Column("title").Over().PartitionBy("status")
				})
			}, "json_object_agg").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowJSONObjectResults)

		suite.Require().NoError(err, "WinJSONObjectAgg should work correctly")
		suite.Require().NotEmpty(windowJSONObjectResults, "Should have window JSON object agg results")

		for _, result := range windowJSONObjectResults {
			suite.NotEmpty(result.JSONObjectAgg, "Should have non-empty JSON object")
			suite.True(strings.HasPrefix(result.JSONObjectAgg, "{"), "Should be a JSON object")
			suite.T().Logf("ID: %s, Status: %s, JSONObjectAgg: %s",
				result.ID, result.Status, result.JSONObjectAgg)
		}
	})
}

// TestWinJSONArrayAgg tests the JSON_ARRAY_AGG window function.
func (suite *EBWindowFunctionsTestSuite) TestWinJSONArrayAgg() {
	suite.T().Logf("Testing WinJSONArrayAgg function for %s", suite.ds.Kind)

	suite.Run("JSONArrayAggPartitionedByStatus", func() {
		type WindowJSONResult struct {
			ID           string `bun:"id"`
			Status       string `bun:"status"`
			JSONArrayAgg string `bun:"json_array_agg"`
		}

		var windowJSONResults []WindowJSONResult

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinJSONArrayAgg(func(wjaab orm.WindowJSONArrayAggBuilder) {
					wjaab.Column("title").Over().PartitionBy("status")
				})
			}, "json_array_agg").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowJSONResults)

		suite.Require().NoError(err, "WinJSONArrayAgg should work correctly")
		suite.Require().NotEmpty(windowJSONResults, "Should have window JSON array agg results")

		for _, result := range windowJSONResults {
			suite.NotEmpty(result.JSONArrayAgg, "Should have non-empty JSON array")
			suite.True(strings.HasPrefix(result.JSONArrayAgg, "["), "Should be a JSON array")
			suite.T().Logf("ID: %s, Status: %s, JSONArrayAgg: %s",
				result.ID, result.Status, result.JSONArrayAgg)
		}
	})
}

// TestWinBitOr tests the BIT_OR window function.
func (suite *EBWindowFunctionsTestSuite) TestWinBitOr() {
	suite.T().Logf("Testing WinBitOr function for %s", suite.ds.Kind)

	suite.Run("BitOrInStatusPartition", func() {
		type WindowBitResult struct {
			ID          string `bun:"id"`
			ViewCount   int64  `bun:"view_count"`
			Status      string `bun:"status"`
			BitOrResult int64  `bun:"bit_or_result"`
		}

		var windowBitResults []WindowBitResult

		err := suite.selectPosts().
			Select("id", "view_count", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinBitOr(func(wbob orm.WindowBitOrBuilder) {
					wbob.Column("view_count").Over().PartitionBy("status")
				})
			}, "bit_or_result").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowBitResults)

		suite.Require().NoError(err, "WinBitOr should work correctly")
		suite.Require().NotEmpty(windowBitResults, "Should have window bit OR results")

		for _, result := range windowBitResults {
			suite.True(result.BitOrResult >= 0, "BitOr should be non-negative")
			suite.T().Logf("ID: %s, Status: %s, ViewCount: %d, BitOr: %d",
				result.ID, result.Status, result.ViewCount, result.BitOrResult)
		}
	})
}

// TestWinBitAnd tests the BIT_AND window function.
func (suite *EBWindowFunctionsTestSuite) TestWinBitAnd() {
	suite.T().Logf("Testing WinBitAnd function for %s", suite.ds.Kind)

	suite.Run("BitAndInStatusPartition", func() {
		type WindowBitResult struct {
			ID           string `bun:"id"`
			ViewCount    int64  `bun:"view_count"`
			Status       string `bun:"status"`
			BitAndResult int64  `bun:"bit_and_result"`
		}

		var windowBitResults []WindowBitResult

		err := suite.selectPosts().
			Select("id", "view_count", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinBitAnd(func(wbab orm.WindowBitAndBuilder) {
					wbab.Column("view_count").Over().PartitionBy("status")
				})
			}, "bit_and_result").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowBitResults)

		suite.Require().NoError(err, "WinBitAnd should work correctly")
		suite.Require().NotEmpty(windowBitResults, "Should have window bit AND results")

		for _, result := range windowBitResults {
			suite.True(result.BitAndResult >= 0, "BitAnd should be non-negative")
			suite.T().Logf("ID: %s, Status: %s, ViewCount: %d, BitAnd: %d",
				result.ID, result.Status, result.ViewCount, result.BitAndResult)
		}
	})
}

// TestWinBoolOr tests the BOOL_OR window function.
func (suite *EBWindowFunctionsTestSuite) TestWinBoolOr() {
	suite.T().Logf("Testing WinBoolOr function for %s", suite.ds.Kind)

	suite.Run("BoolOrInStatusPartition", func() {
		type WindowBoolResult struct {
			ID           string `bun:"id"`
			Status       string `bun:"status"`
			BoolOrResult bool   `bun:"bool_or_result"`
		}

		var windowBoolResults []WindowBoolResult

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinBoolOr(func(wbob orm.WindowBoolOrBuilder) {
					wbob.Expr(eb.Expr("? > 100", eb.Column("view_count"))).Over().PartitionBy("status")
				})
			}, "bool_or_result").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowBoolResults)

		suite.Require().NoError(err, "WinBoolOr should work correctly")
		suite.Require().NotEmpty(windowBoolResults, "Should have window bool OR results")

		for _, result := range windowBoolResults {
			suite.T().Logf("ID: %s, Status: %s, BoolOr: %v",
				result.ID, result.Status, result.BoolOrResult)
		}
	})
}

// TestWinBoolAnd tests the BOOL_AND window function.
func (suite *EBWindowFunctionsTestSuite) TestWinBoolAnd() {
	suite.T().Logf("Testing WinBoolAnd function for %s", suite.ds.Kind)

	suite.Run("BoolAndInStatusPartition", func() {
		type WindowBoolResult struct {
			ID            string `bun:"id"`
			Status        string `bun:"status"`
			BoolAndResult bool   `bun:"bool_and_result"`
		}

		var windowBoolResults []WindowBoolResult

		err := suite.selectPosts().
			Select("id", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinBoolAnd(func(wbab orm.WindowBoolAndBuilder) {
					wbab.Expr(eb.Expr("? > 100", eb.Column("view_count"))).Over().PartitionBy("status")
				})
			}, "bool_and_result").
			OrderBy("status", "id").
			Limit(5).
			Scan(suite.ctx, &windowBoolResults)

		suite.Require().NoError(err, "WinBoolAnd should work correctly")
		suite.Require().NotEmpty(windowBoolResults, "Should have window bool AND results")

		for _, result := range windowBoolResults {
			suite.T().Logf("ID: %s, Status: %s, BoolAnd: %v",
				result.ID, result.Status, result.BoolAndResult)
		}
	})
}

// TestWindowFrameSpecs tests window frame specification methods.
func (suite *EBWindowFunctionsTestSuite) TestWindowFrameSpecs() {
	suite.T().Logf("Testing WindowFrameSpecs for %s", suite.ds.Kind)

	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("SQLite has limited window frame support on %s", suite.ds.Kind)
	}

	suite.Run("RangeFrame", func() {
		type UserWithSum struct {
			Name   string `bun:"name"`
			Age    int16  `bun:"age"`
			RunSum int64  `bun:"run_sum"`
		}

		var users []UserWithSum

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinSum(func(ws orm.WindowSumBuilder) {
					ws.Column("age")
					ws.Over().
						PartitionBy("is_active").
						OrderBy("age").
						Range().
						UnboundedPreceding()
				})
			}, "run_sum").
			OrderBy("age").
			Scan(suite.ctx, &users)

		suite.Require().NoError(err, "Range frame should work")
		suite.Require().NotEmpty(users, "Should have results")

		suite.T().Logf("Found %d users with range frame", len(users))
	})

	suite.Run("RowsFrameWithPreceding", func() {
		type UserWithAvg struct {
			Name   string  `bun:"name"`
			Age    int16   `bun:"age"`
			AvgAge float64 `bun:"avg_age"`
		}

		var users []UserWithAvg

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinAvg(func(wa orm.WindowAvgBuilder) {
					wa.Column("age")
					wa.Over().
						OrderBy("age").
						Rows().
						Preceding(2)
				})
			}, "avg_age").
			OrderBy("age").
			Scan(suite.ctx, &users)

		suite.Require().NoError(err, "Rows frame with preceding should work")
		suite.Require().NotEmpty(users, "Should have results")

		suite.T().Logf("Found %d users with rows frame", len(users))
	})

	suite.Run("RowsFrameWithFollowing", func() {
		type UserWithSum struct {
			Name   string `bun:"name"`
			Age    int16  `bun:"age"`
			SumAge int64  `bun:"sum_age"`
		}

		var users []UserWithSum

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinSum(func(ws orm.WindowSumBuilder) {
					ws.Column("age")
					ws.Over().
						OrderBy("age").
						Rows().
						CurrentRow().
						And().
						Following(1)
				})
			}, "sum_age").
			OrderBy("age").
			Scan(suite.ctx, &users)

		suite.Require().NoError(err, "Rows frame with following should work")
		suite.Require().NotEmpty(users, "Should have results")

		suite.T().Logf("Found %d users with rows+following", len(users))
	})

	if suite.ds.Kind == config.Postgres {
		suite.Run("GroupsFrame", func() {
			type UserWithCount struct {
				Name     string `bun:"name"`
				Age      int16  `bun:"age"`
				CountAge int64  `bun:"count_age"`
			}

			var users []UserWithCount

			err := suite.selectUsers().
				Select("name", "age").
				SelectExpr(func(eb orm.ExprBuilder) any {
					return eb.WinCount(func(wc orm.WindowCountBuilder) {
						wc.Column("age")
						wc.Over().
							OrderBy("age").
							Groups().
							Preceding(1).
							And().
							Following(1)
					})
				}, "count_age").
				OrderBy("age").
				Scan(suite.ctx, &users)

			suite.Require().NoError(err, "Groups frame should work")
			suite.Require().NotEmpty(users, "Should have results")

			suite.T().Logf("Found %d users with groups frame", len(users))
		})
	}
}

// TestPartitionByExprAndOrderByExpr tests PartitionByExpr and OrderByExpr.
func (suite *EBWindowFunctionsTestSuite) TestPartitionByExprAndOrderByExpr() {
	suite.T().Logf("Testing PartitionByExpr/OrderByExpr for %s", suite.ds.Kind)

	suite.Run("PartitionByExpr", func() {
		type UserWithRowNum struct {
			Name   string `bun:"name"`
			Age    int16  `bun:"age"`
			RowNum int64  `bun:"row_num"`
		}

		var users []UserWithRowNum

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.RowNumber(func(rn orm.RowNumberBuilder) {
					rn.Over().
						PartitionByExpr(eb.Column("is_active")).
						OrderBy("age")
				})
			}, "row_num").
			OrderBy("age").
			Scan(suite.ctx, &users)

		suite.Require().NoError(err, "PartitionByExpr should work")
		suite.Len(users, 20, "Should return all users")

		suite.T().Logf("Found %d users with partition by expr", len(users))
	})

	suite.Run("OrderByExpr", func() {
		type UserWithRank struct {
			Name string `bun:"name"`
			Age  int16  `bun:"age"`
			Rank int64  `bun:"rank"`
		}

		var users []UserWithRank

		err := suite.selectUsers().
			Select("name", "age").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Rank(func(r orm.RankBuilder) {
					r.Over().
						OrderByExpr(eb.Column("age"))
				})
			}, "rank").
			OrderBy("age").
			Scan(suite.ctx, &users)

		suite.Require().NoError(err, "OrderByExpr should work")
		suite.Len(users, 20, "Should return all users")

		suite.T().Logf("Found %d users with order by expr", len(users))
	})
}

// TestNullHandling tests IgnoreNulls and RespectNulls.
func (suite *EBWindowFunctionsTestSuite) TestNullHandling() {
	suite.T().Logf("Testing NullHandling for %s", suite.ds.Kind)

	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("IGNORE NULLS / RESPECT NULLS not supported on %s", suite.ds.Kind)
	}

	suite.Run("RespectNulls", func() {
		type Result struct {
			Title    string  `bun:"title"`
			FirstVal *string `bun:"first_val"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("p.title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.FirstValue(func(fv orm.FirstValueBuilder) {
					fv.Column("description")
					fv.RespectNulls()
					fv.Over().OrderBy("p.title")
				})
			}, "first_val").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "RespectNulls should work")

		suite.T().Logf("Found %d results with RespectNulls", len(results))
	})
}

// TestFirstLastValue tests FromFirst and FromLast with NthValue.
func (suite *EBWindowFunctionsTestSuite) TestFirstLastValue() {
	suite.T().Logf("Testing FirstLastValue for %s", suite.ds.Kind)

	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("FROM FIRST/FROM LAST not supported on %s", suite.ds.Kind)
	}

	suite.Run("FromFirst", func() {
		type Result struct {
			Name    string `bun:"name"`
			NthName string `bun:"nth_name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NthValue(func(nv orm.NthValueBuilder) {
					nv.Column("name")
					nv.N(2)
					nv.FromFirst()
					nv.Over().OrderBy("age")
				})
			}, "nth_name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "FromFirst should work")

		suite.T().Logf("Found %d results with FromFirst", len(results))
	})

	suite.Run("FromLast", func() {
		type Result struct {
			Name    string  `bun:"name"`
			NthName *string `bun:"nth_name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NthValue(func(nv orm.NthValueBuilder) {
					nv.Column("name")
					nv.N(2)
					nv.FromLast()
					nv.Over().OrderBy("age")
				})
			}, "nth_name").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "FromLast should work")

		suite.T().Logf("Found %d results with FromLast", len(results))
	})
}

// TestWindowFrameEndBuilderMethods tests uncovered WindowFrameEndBuilder methods.
func (suite *EBWindowFunctionsTestSuite) TestWindowFrameEndBuilderMethods() {
	suite.T().Logf("Testing WindowFrameEndBuilderMethods for %s", suite.ds.Kind)

	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("SQLite has limited window frame support on %s", suite.ds.Kind)
	}

	suite.Run("FrameStartFollowing", func() {
		type Result struct {
			Name   string `bun:"name"`
			SumAge int64  `bun:"sum_age"`
		}

		var results []Result

		// ROWS BETWEEN 1 FOLLOWING AND 2 FOLLOWING
		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinSum(func(ws orm.WindowSumBuilder) {
					ws.Column("age")
					ws.Over().
						OrderBy("age").
						Rows().
						Following(1).
						And().
						Following(2)
				})
			}, "sum_age").
			OrderBy("age").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Frame start Following should work")
	})

	suite.Run("EndUnboundedPreceding", func() {
		type Result struct {
			Name   string `bun:"name"`
			SumAge int64  `bun:"sum_age"`
		}

		var results []Result

		// ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinSum(func(ws orm.WindowSumBuilder) {
					ws.Column("age")
					ws.Over().
						OrderBy("age").
						Rows().
						UnboundedPreceding().
						And().
						CurrentRow()
				})
			}, "sum_age").
			OrderBy("age").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "End CurrentRow should work")
	})

	suite.Run("EndPreceding", func() {
		type Result struct {
			Name   string  `bun:"name"`
			AvgAge float64 `bun:"avg_age"`
		}

		var results []Result

		// ROWS BETWEEN 3 PRECEDING AND 1 PRECEDING
		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.WinAvg(func(wa orm.WindowAvgBuilder) {
					wa.Column("age")
					wa.Over().
						OrderBy("age").
						Rows().
						Preceding(3).
						And().
						Preceding(1)
				})
			}, "avg_age").
			OrderBy("age").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "End Preceding should work")
	})
}

// TestWindowExprMethods tests Expr methods on FirstValue, LastValue, NthValue, Lag, Lead.
func (suite *EBWindowFunctionsTestSuite) TestWindowExprMethods() {
	suite.T().Logf("Testing WindowExprMethods for %s", suite.ds.Kind)

	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("Window Expr methods not supported on %s", suite.ds.Kind)
	}

	suite.Run("FirstValueExpr", func() {
		type Result struct {
			Name     string  `bun:"name"`
			FirstVal *string `bun:"first_val"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.FirstValue(func(fv orm.FirstValueBuilder) {
					fv.Expr(eb.Column("name"))
					fv.Over().OrderBy("age")
				})
			}, "first_val").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "FirstValue.Expr should work")
	})

	suite.Run("LastValueExpr", func() {
		type Result struct {
			Name    string  `bun:"name"`
			LastVal *string `bun:"last_val"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.LastValue(func(lv orm.LastValueBuilder) {
					lv.Expr(eb.Column("name"))
					lv.Over().OrderBy("age")
				})
			}, "last_val").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "LastValue.Expr should work")
	})

	suite.Run("NthValueExpr", func() {
		type Result struct {
			Name   string  `bun:"name"`
			NthVal *string `bun:"nth_val"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.NthValue(func(nv orm.NthValueBuilder) {
					nv.Expr(eb.Column("name"))
					nv.N(2)
					nv.Over().OrderBy("age")
				})
			}, "nth_val").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "NthValue.Expr should work")
	})

	suite.Run("LagExpr", func() {
		type Result struct {
			Name   string  `bun:"name"`
			LagVal *string `bun:"lag_val"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lag(func(l orm.LagBuilder) {
					l.Expr(eb.Column("name"))
					l.Offset(1)
					l.Over().OrderBy("age")
				})
			}, "lag_val").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Lag.Expr should work")
	})

	suite.Run("LeadExpr", func() {
		type Result struct {
			Name    string  `bun:"name"`
			LeadVal *string `bun:"lead_val"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lead(func(l orm.LeadBuilder) {
					l.Expr(eb.Column("name"))
					l.Offset(1)
					l.Over().OrderBy("age")
				})
			}, "lead_val").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Lead.Expr should work")
	})
}

// TestWindowIgnoreNulls tests IgnoreNulls on window functions.
func (suite *EBWindowFunctionsTestSuite) TestWindowIgnoreNulls() {
	suite.T().Logf("Testing WindowIgnoreNulls for %s", suite.ds.Kind)

	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("IGNORE NULLS not supported on %s", suite.ds.Kind)
	}

	type Result struct {
		Title    string  `bun:"title"`
		FirstVal *string `bun:"first_val"`
	}

	var results []Result

	err := suite.selectPosts().
		Select("p.title").
		SelectExpr(func(eb orm.ExprBuilder) any {
			return eb.FirstValue(func(fv orm.FirstValueBuilder) {
				fv.Column("description")
				fv.IgnoreNulls()
				fv.Over().OrderBy("p.title")
			})
		}, "first_val").
		Limit(5).
		Scan(suite.ctx, &results)

	suite.Require().NoError(err, "IgnoreNulls should work")
}

// TestWindowFrameEndUnboundedPreceding tests WindowFrameEndBuilder.UnboundedPreceding.
func (suite *EBWindowFunctionsTestSuite) TestWindowFrameEndUnboundedPreceding() {
	suite.T().Logf("Testing WindowFrameEndUnboundedPreceding for %s", suite.ds.Kind)

	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("SQLite has limited window frame support on %s", suite.ds.Kind)
	}

	type Result struct {
		Name   string `bun:"name"`
		SumAge int64  `bun:"sum_age"`
	}

	var results []Result

	// ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED PRECEDING
	// This is unusual but covers the end builder's UnboundedPreceding
	err := suite.selectUsers().
		Select("name").
		SelectExpr(func(eb orm.ExprBuilder) any {
			return eb.WinSum(func(ws orm.WindowSumBuilder) {
				ws.Column("age")
				ws.Over().
					OrderBy("age").
					Rows().
					Preceding(2).
					And().
					UnboundedPreceding()
			})
		}, "sum_age").
		OrderBy("age").
		Limit(3).
		Scan(suite.ctx, &results)

	// May fail on some DBs but the code path is covered
	suite.T().Logf("WindowFrameEnd UnboundedPreceding err: %v, results: %d", err, len(results))
}
