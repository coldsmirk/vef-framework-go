package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBUtilityFunctionsTestSuite{BaseTestSuite: base}
	})
}

// EBUtilityFunctionsTestSuite tests utility expression methods of orm.ExprBuilder.
type EBUtilityFunctionsTestSuite struct {
	*BaseTestSuite
}

// TestDecode tests the Decode utility function.
func (suite *EBUtilityFunctionsTestSuite) TestDecode() {
	suite.T().Logf("Testing Decode utility function for %s", suite.ds.Kind)

	suite.Run("DecodeStatusDescriptionMapping", func() {
		type DecodeStatusResult struct {
			ID         string `bun:"id"`
			Title      string `bun:"title"`
			Status     string `bun:"status"`
			StatusDesc string `bun:"status_desc"`
		}

		var decodeStatusResults []DecodeStatusResult

		err := suite.selectPosts().
			Select("id", "title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.Column("status"),
					"published", "Published Article",
					"draft", "Draft Article",
					"review", "Under Review",
					"Unknown Status",
				)
			}, "status_desc").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &decodeStatusResults)

		suite.Require().NoError(err, "DECODE should work for status description mapping")
		suite.Require().NotEmpty(decodeStatusResults, "Should have decode status results")

		for _, result := range decodeStatusResults {
			suite.NotEmpty(result.StatusDesc, "Status description should not be empty")

			switch result.Status {
			case "published":
				suite.Equal("Published Article", result.StatusDesc, "Published status should map to 'Published Article'")
			case "draft":
				suite.Equal("Draft Article", result.StatusDesc, "Draft status should map to 'Draft Article'")
			case "review":
				suite.Equal("Under Review", result.StatusDesc, "Review status should map to 'Under Review'")
			default:
				suite.Equal("Unknown Status", result.StatusDesc, "Unknown status should map to 'Unknown Status'")
			}

			suite.T().Logf("Id: %s, Post %s: %s -> %s",
				result.ID, result.Title, result.Status, result.StatusDesc)
		}
	})

	suite.Run("DecodeStatusPriorityMapping", func() {
		type DecodePriorityResult struct {
			ID             string `bun:"id"`
			Title          string `bun:"title"`
			Status         string `bun:"status"`
			StatusPriority int64  `bun:"status_priority"`
		}

		var decodePriorityResults []DecodePriorityResult

		err := suite.selectPosts().
			Select("id", "title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.Column("status"),
					"published", 1,
					"review", 2,
					"draft", 3,
					99,
				)
			}, "status_priority").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &decodePriorityResults)

		suite.Require().NoError(err, "DECODE should work for priority mapping")
		suite.Require().NotEmpty(decodePriorityResults, "Should have decode priority results")

		for _, result := range decodePriorityResults {
			suite.True(result.StatusPriority > 0, "Status priority should be positive")

			switch result.Status {
			case "published":
				suite.Equal(int64(1), result.StatusPriority, "Published status should have priority 1")
			case "review":
				suite.Equal(int64(2), result.StatusPriority, "Review status should have priority 2")
			case "draft":
				suite.Equal(int64(3), result.StatusPriority, "Draft status should have priority 3")
			default:
				suite.Equal(int64(99), result.StatusPriority, "Unknown status should have priority 99")
			}

			suite.T().Logf("Id: %s, Post %s: %s -> Priority: %d",
				result.ID, result.Title, result.Status, result.StatusPriority)
		}
	})

	suite.Run("DecodeCombinedMapping", func() {
		type DecodeCombinedResult struct {
			ID             string `bun:"id"`
			Title          string `bun:"title"`
			Status         string `bun:"status"`
			StatusDesc     string `bun:"status_desc"`
			StatusPriority int64  `bun:"status_priority"`
		}

		var decodeCombinedResults []DecodeCombinedResult

		err := suite.selectPosts().
			Select("id", "title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.Column("status"),
					"published", "Published Article",
					"draft", "Draft Article",
					"review", "Under Review",
					"Unknown Status",
				)
			}, "status_desc").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.Column("status"),
					"published", 1,
					"review", 2,
					"draft", 3,
					99,
				)
			}, "status_priority").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &decodeCombinedResults)

		suite.Require().NoError(err, "Combined DECODE should work")
		suite.Require().NotEmpty(decodeCombinedResults, "Should have combined decode results")

		for _, result := range decodeCombinedResults {
			suite.NotEmpty(result.StatusDesc, "Status description should not be empty")
			suite.True(result.StatusPriority > 0, "Status priority should be positive")

			switch result.Status {
			case "published":
				suite.Equal("Published Article", result.StatusDesc, "Should map published status description")
				suite.Equal(int64(1), result.StatusPriority, "Should map published priority")
			case "draft":
				suite.Equal("Draft Article", result.StatusDesc, "Should map draft status description")
				suite.Equal(int64(3), result.StatusPriority, "Should map draft priority")
			case "review":
				suite.Equal("Under Review", result.StatusDesc, "Should map review status description")
				suite.Equal(int64(2), result.StatusPriority, "Should map review priority")
			default:
				suite.Equal("Unknown Status", result.StatusDesc, "Should map unknown status description")
				suite.Equal(int64(99), result.StatusPriority, "Should map unknown priority")
			}

			suite.T().Logf("Id: %s, Post %s: %s -> %s (Priority: %d)",
				result.ID, result.Title, result.Status, result.StatusDesc, result.StatusPriority)
		}
	})

	suite.Run("DecodeInvalidArguments", func() {
		type DecodeInvalidResult struct {
			ID     string  `bun:"id"`
			Title  string  `bun:"title"`
			Result *string `bun:"result"`
		}

		var results []DecodeInvalidResult

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(eb.Column("status"))
			}, "result").
			Limit(1).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DECODE with invalid arguments should return NULL")
		suite.Require().NotEmpty(results, "Should have results")

		for _, result := range results {
			suite.Nil(result.Result, "Result should be NULL for invalid DECODE arguments")
			suite.T().Logf("Id: %s, Title: %s, Result: NULL (as expected)", result.ID, result.Title)
		}
	})

	suite.Run("DecodeMinimalArguments", func() {
		type DecodeMinimalResult struct {
			ID     string  `bun:"id"`
			Title  string  `bun:"title"`
			Status string  `bun:"status"`
			Label  *string `bun:"label"`
		}

		var results []DecodeMinimalResult

		err := suite.selectPosts().
			Select("id", "title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.Column("status"),
					"published", "Live",
				)
			}, "label").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DECODE with minimal arguments should work")
		suite.Require().NotEmpty(results, "Should have results")

		for _, result := range results {
			if result.Status == "published" {
				suite.NotNil(result.Label, "Label should not be NULL for published status")
				suite.Equal("Live", *result.Label, "Published status should map to 'Live'")
			} else {
				suite.Nil(result.Label, "Label should be NULL for non-published status")
			}

			labelStr := "NULL"
			if result.Label != nil {
				labelStr = *result.Label
			}

			suite.T().Logf("Id: %s, Title: %s, Status: %s -> Label: %s",
				result.ID, result.Title, result.Status, labelStr)
		}
	})

	suite.Run("DecodeWithoutDefault", func() {
		type DecodeNoDefaultResult struct {
			ID     string  `bun:"id"`
			Title  string  `bun:"title"`
			Status string  `bun:"status"`
			Tag    *string `bun:"tag"`
		}

		var results []DecodeNoDefaultResult

		err := suite.selectPosts().
			Select("id", "title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.Column("status"),
					"published", "Public",
					"draft", "Private",
				)
			}, "tag").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DECODE without default should work")
		suite.Require().NotEmpty(results, "Should have results")

		for _, result := range results {
			switch result.Status {
			case "published":
				suite.NotNil(result.Tag, "Tag should not be NULL for published status")
				suite.Equal("Public", *result.Tag, "Published status should map to 'Public'")
			case "draft":
				suite.NotNil(result.Tag, "Tag should not be NULL for draft status")
				suite.Equal("Private", *result.Tag, "Draft status should map to 'Private'")
			default:
				suite.Nil(result.Tag, "Tag should be NULL for unmapped status")
			}

			tagStr := "NULL"
			if result.Tag != nil {
				tagStr = *result.Tag
			}

			suite.T().Logf("Id: %s, Title: %s, Status: %s -> Tag: %s",
				result.ID, result.Title, result.Status, tagStr)
		}
	})

	suite.Run("DecodeNullValueMapping", func() {
		type DecodeNullMappingResult struct {
			ID          string  `bun:"id"`
			Title       string  `bun:"title"`
			Description *string `bun:"description"`
			DescLabel   string  `bun:"desc_label"`
		}

		var results []DecodeNullMappingResult

		err := suite.selectPosts().
			Select("id", "title", "description").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.IsNull("description"),
					eb.Literal(true), "No Description",
					eb.Literal(false), "Has Description",
					"Unknown",
				)
			}, "desc_label").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DECODE with NULL value mapping should work")
		suite.Require().NotEmpty(results, "Should have results")

		for _, result := range results {
			if result.Description == nil {
				suite.Equal("No Description", result.DescLabel, "NULL description should map to 'No Description'")
			} else {
				suite.Equal("Has Description", result.DescLabel, "Non-NULL description should map to 'Has Description'")
			}

			descStr := "NULL"
			if result.Description != nil {
				descStr = *result.Description
			}

			suite.T().Logf("Id: %s, Title: %s, Description: %s -> Label: %s",
				result.ID, result.Title, descStr, result.DescLabel)
		}
	})

	suite.Run("DecodeNestedExpression", func() {
		type DecodeNestedResult struct {
			ID            string `bun:"id"`
			Title         string `bun:"title"`
			ViewCount     int64  `bun:"view_count"`
			ViewCategory  string `bun:"view_category"`
			CategoryLabel string `bun:"category_label"`
		}

		var results []DecodeNestedResult

		err := suite.selectPosts().
			Select("id", "title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Case(func(cb orm.CaseBuilder) {
					cb.When(func(cond orm.ConditionBuilder) {
						cond.GreaterThan("view_count", 80)
					}).
						Then("High").
						When(func(cond orm.ConditionBuilder) {
							cond.GreaterThan("view_count", 30)
						}).
						Then("Medium").
						Else("Low")
				})
			}, "view_category").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Decode(
					eb.Case(func(cb orm.CaseBuilder) {
						cb.When(func(cond orm.ConditionBuilder) {
							cond.GreaterThan("view_count", 80)
						}).
							Then("High").
							When(func(cond orm.ConditionBuilder) {
								cond.GreaterThan("view_count", 30)
							}).
							Then("Medium").
							Else("Low")
					}),
					"High", "Popular Content",
					"Medium", "Regular Content",
					"Low", "New Content",
					"Unknown",
				)
			}, "category_label").
			OrderBy("view_count").
			Limit(8).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DECODE with nested expression should work")
		suite.Require().NotEmpty(results, "Should have results")

		for _, result := range results {
			suite.NotEmpty(result.ViewCategory, "View category should not be empty")
			suite.NotEmpty(result.CategoryLabel, "Category label should not be empty")

			switch result.ViewCategory {
			case "High":
				suite.Equal("Popular Content", result.CategoryLabel, "High category should map to 'Popular Content'")
			case "Medium":
				suite.Equal("Regular Content", result.CategoryLabel, "Medium category should map to 'Regular Content'")
			case "Low":
				suite.Equal("New Content", result.CategoryLabel, "Low category should map to 'New Content'")
			default:
				suite.Equal("Unknown", result.CategoryLabel, "Unknown category should map to 'Unknown'")
			}

			suite.T().Logf("Id: %s, Title: %s, ViewCount: %d -> Category: %s (%s)",
				result.ID, result.Title, result.ViewCount, result.ViewCategory, result.CategoryLabel)
		}
	})
}
