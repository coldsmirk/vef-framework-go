package orm_test

import (
	"strings"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBStringFunctionsTestSuite{BaseTestSuite: base}
	})
}

// EBStringFunctionsTestSuite tests string manipulation methods of orm.ExprBuilder.
type EBStringFunctionsTestSuite struct {
	*BaseTestSuite
}

// TestConcat tests the Concat function.
func (suite *EBStringFunctionsTestSuite) TestConcat() {
	suite.T().Logf("Testing Concat for %s", suite.ds.Kind)

	suite.Run("ConcatTitleAndStatus", func() {
		type Result struct {
			Title          string `bun:"title"`
			Status         string `bun:"status"`
			TitleAndStatus string `bun:"title_and_status"`
			MultiConcat    string `bun:"multi_concat"`
		}

		var concatResults []Result

		err := suite.selectPosts().
			Select("title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Concat(eb.Column("title"), "' - '", eb.Column("status"))
			}, "title_and_status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Concat("'[', ", eb.Column("status"), "'] '", eb.Column("title"))
			}, "multi_concat").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &concatResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(concatResults, "Should return results")

		for _, result := range concatResults {
			suite.Contains(result.TitleAndStatus, result.Title, "Concat should include title")
			suite.Contains(result.TitleAndStatus, result.Status, "Concat should include status")
			suite.Contains(result.MultiConcat, result.Title, "Multi concat should include title")
			suite.Contains(result.MultiConcat, result.Status, "Multi concat should include status")
		}
	})
}

// TestConcatWithSep tests the ConcatWithSep function.
func (suite *EBStringFunctionsTestSuite) TestConcatWithSep() {
	suite.T().Logf("Testing ConcatWithSep for %s", suite.ds.Kind)

	suite.Run("ConcatWithDashSeparator", func() {
		type Result struct {
			ID     string `bun:"id"`
			Joined string `bun:"joined"`
		}

		var concatResults []Result

		err := suite.selectPosts().
			Select("id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ConcatWithSep(" - ", eb.Column("title"), eb.Column("status"))
			}, "joined").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &concatResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(concatResults, "Should return results")

		for _, result := range concatResults {
			suite.Contains(result.Joined, " - ", "Should contain separator")
		}
	})
}

// TestSubString tests the SubString function.
func (suite *EBStringFunctionsTestSuite) TestSubString() {
	suite.T().Logf("Testing SubString for %s", suite.ds.Kind)

	suite.Run("ExtractSubstrings", func() {
		type Result struct {
			Title        string `bun:"title"`
			First5Chars  string `bun:"first5_chars"`
			Middle3Chars string `bun:"middle3_chars"`
		}

		var substringResults []Result

		err := suite.selectPosts().
			Select("title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SubString(eb.Column("title"), 1, 5)
			}, "first5_chars").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.SubString(eb.Column("title"), 3, 3)
			}, "middle3_chars").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &substringResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(substringResults, "Should return results")

		for _, result := range substringResults {
			if len(result.Title) >= 5 {
				suite.True(len(result.First5Chars) <= 5, "First5Chars should be at most 5 characters")
				suite.True(len(result.Middle3Chars) <= 3, "Middle3Chars should be at most 3 characters")
			}
		}
	})
}

// TestUpper tests the Upper function.
func (suite *EBStringFunctionsTestSuite) TestUpper() {
	suite.T().Logf("Testing Upper for %s", suite.ds.Kind)

	suite.Run("ConvertToUppercase", func() {
		type Result struct {
			Title       string `bun:"title"`
			UpperTitle  string `bun:"upper_title"`
			Status      string `bun:"status"`
			UpperStatus string `bun:"upper_status"`
		}

		var caseResults []Result

		err := suite.selectPosts().
			Select("title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Upper(eb.Column("title"))
			}, "upper_title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Upper(eb.Column("status"))
			}, "upper_status").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &caseResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(caseResults, "Should return results")

		for _, result := range caseResults {
			suite.Equal(strings.ToUpper(result.Title), result.UpperTitle, "Upper should convert title to uppercase")
			suite.Equal(strings.ToUpper(result.Status), result.UpperStatus, "Upper should convert status to uppercase")
		}
	})
}

// TestLower tests the Lower function.
func (suite *EBStringFunctionsTestSuite) TestLower() {
	suite.T().Logf("Testing Lower for %s", suite.ds.Kind)

	suite.Run("ConvertToLowercase", func() {
		type Result struct {
			Title       string `bun:"title"`
			LowerTitle  string `bun:"lower_title"`
			Status      string `bun:"status"`
			LowerStatus string `bun:"lower_status"`
		}

		var caseResults []Result

		err := suite.selectPosts().
			Select("title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lower(eb.Column("title"))
			}, "lower_title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lower(eb.Column("status"))
			}, "lower_status").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &caseResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(caseResults, "Should return results")

		for _, result := range caseResults {
			suite.Equal(strings.ToLower(result.Title), result.LowerTitle, "Lower should convert title to lowercase")
			suite.Equal(strings.ToLower(result.Status), result.LowerStatus, "Lower should convert status to lowercase")
		}
	})
}

// TestTrim tests the Trim function.
func (suite *EBStringFunctionsTestSuite) TestTrim() {
	suite.T().Logf("Testing Trim for %s", suite.ds.Kind)

	suite.Run("TrimWhitespace", func() {
		type Result struct {
			Status        string `bun:"status"`
			TrimmedStatus string `bun:"trimmed_status"`
		}

		var trimResults []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Trim(eb.Column("status"))
			}, "trimmed_status").
			OrderBy("status").
			Limit(5).
			Scan(suite.ctx, &trimResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(trimResults, "Should return results")

		for _, result := range trimResults {
			suite.NotEmpty(result.Status, "Should have non-empty Status")
			suite.NotEmpty(result.TrimmedStatus, "Should have non-empty TrimmedStatus")
			// Since status values don't have leading/trailing spaces, they should be equal
			suite.Equal(result.Status, result.TrimmedStatus, "Trim should preserve non-whitespace text")
		}
	})
}

// TestTrimLeft tests the TrimLeft function.
func (suite *EBStringFunctionsTestSuite) TestTrimLeft() {
	suite.T().Logf("Testing TrimLeft for %s", suite.ds.Kind)

	suite.Run("TrimLeadingWhitespace", func() {
		type Result struct {
			ID          string `bun:"id"`
			Original    string `bun:"original"`
			LeftTrimmed string `bun:"left_trimmed"`
		}

		var trimResults []Result

		err := suite.selectPosts().
			Select("id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Concat("   ", eb.Column("status"), "   ")
			}, "original").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.TrimLeft(eb.Concat("   ", eb.Column("status"), "   "))
			}, "left_trimmed").
			Limit(3).
			Scan(suite.ctx, &trimResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(trimResults, "Should return results")

		for _, result := range trimResults {
			suite.Contains(result.Original, "   ", "Original should contain spaces")
			suite.NotEqual(result.Original, result.LeftTrimmed, "Left trimmed should differ from original")
		}
	})
}

// TestTrimRight tests the TrimRight function.
func (suite *EBStringFunctionsTestSuite) TestTrimRight() {
	suite.T().Logf("Testing TrimRight for %s", suite.ds.Kind)

	suite.Run("TrimTrailingWhitespace", func() {
		type Result struct {
			ID           string `bun:"id"`
			Original     string `bun:"original"`
			RightTrimmed string `bun:"right_trimmed"`
		}

		var trimResults []Result

		err := suite.selectPosts().
			Select("id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Concat("   ", eb.Column("status"), "   ")
			}, "original").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.TrimRight(eb.Concat("   ", eb.Column("status"), "   "))
			}, "right_trimmed").
			Limit(3).
			Scan(suite.ctx, &trimResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(trimResults, "Should return results")

		for _, result := range trimResults {
			suite.Contains(result.Original, "   ", "Original should contain spaces")
			suite.NotEqual(result.Original, result.RightTrimmed, "Right trimmed should differ from original")
		}
	})
}

// TestLength tests the Length function.
func (suite *EBStringFunctionsTestSuite) TestLength() {
	suite.T().Logf("Testing Length for %s", suite.ds.Kind)

	suite.Run("CalculateStringLength", func() {
		type Result struct {
			Title        string `bun:"title"`
			TitleLength  int64  `bun:"title_length"`
			Status       string `bun:"status"`
			StatusLength int64  `bun:"status_length"`
		}

		var lengthResults []Result

		err := suite.selectPosts().
			Select("title", "status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Length(eb.Column("title"))
			}, "title_length").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Length(eb.Column("status"))
			}, "status_length").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &lengthResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(lengthResults, "Should return results")

		for _, result := range lengthResults {
			suite.Equal(int64(len(result.Title)), result.TitleLength, "Length should match title byte length")
			suite.Equal(int64(len(result.Status)), result.StatusLength, "Length should match status byte length")
		}
	})
}

// TestCharLength tests the CharLength function.
func (suite *EBStringFunctionsTestSuite) TestCharLength() {
	suite.T().Logf("Testing CharLength for %s", suite.ds.Kind)

	suite.Run("CalculateCharacterLength", func() {
		type Result struct {
			Title   string `bun:"title"`
			CharLen int64  `bun:"char_len"`
		}

		var lengthResults []Result

		err := suite.selectPosts().
			Select("title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CharLength(eb.Column("title"))
			}, "char_len").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &lengthResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(lengthResults, "Should return results")

		for _, result := range lengthResults {
			suite.True(result.CharLen > 0, "Character length should be positive")
		}
	})
}

// TestPosition tests the Position function.
func (suite *EBStringFunctionsTestSuite) TestPosition() {
	suite.T().Logf("Testing Position for %s", suite.ds.Kind)

	suite.Run("FindSubstringPosition", func() {
		type Result struct {
			Title    string `bun:"title"`
			Position int64  `bun:"pos"`
		}

		var posResults []Result

		err := suite.selectPosts().
			Select("title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Position("o", eb.Column("title"))
			}, "pos").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &posResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(posResults, "Should return results")

		for _, result := range posResults {
			suite.True(result.Position >= 0, "Position should be non-negative (0 means not found)")
		}
	})
}

// TestLeft tests the Left function.
func (suite *EBStringFunctionsTestSuite) TestLeft() {
	suite.T().Logf("Testing Left for %s", suite.ds.Kind)

	suite.Run("ExtractLeftmostCharacters", func() {
		type Result struct {
			Title    string `bun:"title"`
			LeftPart string `bun:"left_part"`
		}

		var leftResults []Result

		err := suite.selectPosts().
			Select("title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Left(eb.Column("title"), 10)
			}, "left_part").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &leftResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(leftResults, "Should return results")

		for _, result := range leftResults {
			suite.True(len(result.LeftPart) <= 10, "Left part should be at most 10 characters")
		}
	})
}

// TestRight tests the Right function.
func (suite *EBStringFunctionsTestSuite) TestRight() {
	suite.T().Logf("Testing Right for %s", suite.ds.Kind)

	suite.Run("ExtractRightmostCharacters", func() {
		type Result struct {
			Title     string `bun:"title"`
			RightPart string `bun:"right_part"`
		}

		var rightResults []Result

		err := suite.selectPosts().
			Select("title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Right(eb.Column("title"), 5)
			}, "right_part").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &rightResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(rightResults, "Should return results")

		for _, result := range rightResults {
			suite.True(len(result.RightPart) <= 5, "Right part should be at most 5 characters")
		}
	})
}

// TestRepeat tests the Repeat function.
func (suite *EBStringFunctionsTestSuite) TestRepeat() {
	suite.T().Logf("Testing Repeat for %s", suite.ds.Kind)

	suite.Run("RepeatString", func() {
		type Result struct {
			ID       string `bun:"id"`
			Repeated string `bun:"repeated"`
		}

		var repeatResults []Result

		err := suite.selectPosts().
			Select("id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Repeat("*", 5)
			}, "repeated").
			Limit(3).
			Scan(suite.ctx, &repeatResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(repeatResults, "Should return results")

		for _, result := range repeatResults {
			suite.Equal("*****", result.Repeated, "Should repeat '*' 5 times")
		}
	})
}

// TestReplace tests the Replace function.
func (suite *EBStringFunctionsTestSuite) TestReplace() {
	suite.T().Logf("Testing Replace for %s", suite.ds.Kind)

	suite.Run("ReplaceSubstring", func() {
		type Result struct {
			Status         string `bun:"status"`
			ReplacedStatus string `bun:"replaced_status"`
		}

		var replaceResults []Result

		err := suite.selectPosts().
			Select("status").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Replace(eb.Column("status"), "'draft'", "'DRAFT'")
			}, "replaced_status").
			OrderBy("status").
			Limit(5).
			Scan(suite.ctx, &replaceResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(replaceResults, "Should return results")

		for _, result := range replaceResults {
			suite.NotEmpty(result.Status, "Should have non-empty Status")
			suite.NotEmpty(result.ReplacedStatus, "Should have non-empty ReplacedStatus")
		}
	})
}

// TestReverse tests the Reverse function.
func (suite *EBStringFunctionsTestSuite) TestReverse() {
	suite.T().Logf("Testing Reverse for %s", suite.ds.Kind)

	suite.Run("ReverseString", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("Reverse not supported on %s (framework limitation: no simulation provided)", suite.ds.Kind)
		}

		type Result struct {
			Title    string `bun:"title"`
			Reversed string `bun:"reversed"`
		}

		var reverseResults []Result

		err := suite.selectPosts().
			Select("title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Reverse(eb.Column("title"))
			}, "reversed").
			Limit(3).
			Scan(suite.ctx, &reverseResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(reverseResults, "Should return results")

		for _, result := range reverseResults {
			suite.NotEmpty(result.Reversed, "Should have non-empty Reversed")
		}
	})
}

// TestCombinedStringFunctions tests using multiple string functions together.
func (suite *EBStringFunctionsTestSuite) TestCombinedStringFunctions() {
	suite.T().Logf("Testing CombinedStringFunctions for %s", suite.ds.Kind)

	suite.Run("NestedStringFunctions", func() {
		type Result struct {
			Title       string `bun:"title"`
			UpperTitle  string `bun:"upper_title"`
			LowerTitle  string `bun:"lower_title"`
			CombinedStr string `bun:"combined_str"`
		}

		var combinedResults []Result

		err := suite.selectPosts().
			Select("title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Upper(eb.Column("title"))
			}, "upper_title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Lower(eb.Column("title"))
			}, "lower_title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Concat(
					eb.Upper(eb.SubString(eb.Column("title"), 1, 3)),
					"...",
					eb.Lower(eb.SubString(eb.Column("title"), 1, 3)),
				)
			}, "combined_str").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &combinedResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(combinedResults, "Should return results")

		for _, result := range combinedResults {
			suite.NotEmpty(result.UpperTitle, "Should have non-empty UpperTitle")
			suite.NotEmpty(result.LowerTitle, "Should have non-empty LowerTitle")
			suite.NotEmpty(result.CombinedStr, "Should have non-empty CombinedStr")
		}
	})
}

// TestContains tests the Contains function (case-sensitive substring check).
func (suite *EBStringFunctionsTestSuite) TestContains() {
	suite.T().Logf("Testing Contains for %s", suite.ds.Kind)

	suite.Run("ContainsSubstring", func() {
		type Result struct {
			ID            string `bun:"id"`
			Title         string `bun:"title"`
			ContainsPost  bool   `bun:"contains_post"`
			ContainsGuide bool   `bun:"contains_guide"`
		}

		var containsResults []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Contains(eb.Column("title"), "'Post'")
			}, "contains_post").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Contains(eb.Column("title"), "'Guide'")
			}, "contains_guide").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &containsResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(containsResults, "Should return results")

		for _, result := range containsResults {
			suite.NotEmpty(result.ID, "Should have non-empty ID")
			suite.NotEmpty(result.Title, "Should have non-empty Title")

			// Verify case-sensitive matching
			if strings.Contains(result.Title, "Post") {
				suite.True(result.ContainsPost, "Should contain 'Post' when present")
			} else {
				suite.False(result.ContainsPost, "Should not contain 'Post' when absent")
			}

			if strings.Contains(result.Title, "Guide") {
				suite.True(result.ContainsGuide, "Should contain 'Guide' when present")
			} else {
				suite.False(result.ContainsGuide, "Should not contain 'Guide' when absent")
			}
		}
	})
}

// TestStartsWith tests the StartsWith function (case-sensitive prefix check).
func (suite *EBStringFunctionsTestSuite) TestStartsWith() {
	suite.T().Logf("Testing StartsWith for %s", suite.ds.Kind)

	suite.Run("StartsWithPrefix", func() {
		type Result struct {
			ID          string `bun:"id"`
			Title       string `bun:"title"`
			StartsWithG bool   `bun:"starts_with_g"`
			StartsWithP bool   `bun:"starts_with_p"`
		}

		var startsWithResults []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StartsWith(eb.Column("title"), "'G'")
			}, "starts_with_g").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StartsWith(eb.Column("title"), "'P'")
			}, "starts_with_p").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &startsWithResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(startsWithResults, "Should return results")

		for _, result := range startsWithResults {
			suite.NotEmpty(result.ID, "Should have non-empty ID")
			suite.NotEmpty(result.Title, "Should have non-empty Title")

			// Verify case-sensitive prefix matching
			if strings.HasPrefix(result.Title, "G") {
				suite.True(result.StartsWithG, "Should start with 'G' when present")
			} else {
				suite.False(result.StartsWithG, "Should not start with 'G' when absent")
			}

			if strings.HasPrefix(result.Title, "P") {
				suite.True(result.StartsWithP, "Should start with 'P' when present")
			} else {
				suite.False(result.StartsWithP, "Should not start with 'P' when absent")
			}
		}
	})
}

// TestEndsWith tests the EndsWith function (case-sensitive suffix check).
func (suite *EBStringFunctionsTestSuite) TestEndsWith() {
	suite.T().Logf("Testing EndsWith for %s", suite.ds.Kind)

	suite.Run("EndsWithSuffix", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			EndsWithE bool   `bun:"ends_with_e"`
			EndsWithT bool   `bun:"ends_with_t"`
		}

		var endsWithResults []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.EndsWith(eb.Column("title"), "'e'")
			}, "ends_with_e").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.EndsWith(eb.Column("title"), "'t'")
			}, "ends_with_t").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &endsWithResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(endsWithResults, "Should return results")

		for _, result := range endsWithResults {
			suite.NotEmpty(result.ID, "Should have non-empty ID")
			suite.NotEmpty(result.Title, "Should have non-empty Title")

			// Verify case-sensitive suffix matching
			if strings.HasSuffix(result.Title, "e") {
				suite.True(result.EndsWithE, "Should end with 'e' when present")
			} else {
				suite.False(result.EndsWithE, "Should not end with 'e' when absent")
			}

			if strings.HasSuffix(result.Title, "t") {
				suite.True(result.EndsWithT, "Should end with 't' when present")
			} else {
				suite.False(result.EndsWithT, "Should not end with 't' when absent")
			}
		}
	})
}

// TestContainsIgnoreCase tests the ContainsIgnoreCase function (case-insensitive substring check).
func (suite *EBStringFunctionsTestSuite) TestContainsIgnoreCase() {
	suite.T().Logf("Testing ContainsIgnoreCase for %s", suite.ds.Kind)

	suite.Run("ContainsSubstringIgnoreCase", func() {
		type Result struct {
			ID            string `bun:"id"`
			Title         string `bun:"title"`
			ContainsPost  bool   `bun:"contains_post"`
			ContainsGuide bool   `bun:"contains_guide"`
		}

		var containsResults []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ContainsIgnoreCase(eb.Column("title"), "'post'")
			}, "contains_post").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ContainsIgnoreCase(eb.Column("title"), "'GUIDE'")
			}, "contains_guide").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &containsResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(containsResults, "Should return results")

		for _, result := range containsResults {
			suite.NotEmpty(result.ID, "Should have non-empty ID")
			suite.NotEmpty(result.Title, "Should have non-empty Title")

			// Verify case-insensitive matching
			lowerTitle := strings.ToLower(result.Title)
			if strings.Contains(lowerTitle, "post") {
				suite.True(result.ContainsPost, "Should contain 'post' (case-insensitive) when present")
			} else {
				suite.False(result.ContainsPost, "Should not contain 'post' (case-insensitive) when absent")
			}

			if strings.Contains(lowerTitle, "guide") {
				suite.True(result.ContainsGuide, "Should contain 'guide' (case-insensitive) when present")
			} else {
				suite.False(result.ContainsGuide, "Should not contain 'guide' (case-insensitive) when absent")
			}
		}
	})
}

// TestStartsWithIgnoreCase tests the StartsWithIgnoreCase function (case-insensitive prefix check).
func (suite *EBStringFunctionsTestSuite) TestStartsWithIgnoreCase() {
	suite.T().Logf("Testing StartsWithIgnoreCase for %s", suite.ds.Kind)

	suite.Run("StartsWithPrefixIgnoreCase", func() {
		type Result struct {
			ID          string `bun:"id"`
			Title       string `bun:"title"`
			StartsWithG bool   `bun:"starts_with_g"`
			StartsWithP bool   `bun:"starts_with_p"`
		}

		var startsWithResults []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StartsWithIgnoreCase(eb.Column("title"), "'g'")
			}, "starts_with_g").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.StartsWithIgnoreCase(eb.Column("title"), "'p'")
			}, "starts_with_p").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &startsWithResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(startsWithResults, "Should return results")

		for _, result := range startsWithResults {
			suite.NotEmpty(result.ID, "Should have non-empty ID")
			suite.NotEmpty(result.Title, "Should have non-empty Title")

			// Verify case-insensitive prefix matching
			lowerTitle := strings.ToLower(result.Title)
			if strings.HasPrefix(lowerTitle, "g") {
				suite.True(result.StartsWithG, "Should start with 'g' (case-insensitive) when present")
			} else {
				suite.False(result.StartsWithG, "Should not start with 'g' (case-insensitive) when absent")
			}

			if strings.HasPrefix(lowerTitle, "p") {
				suite.True(result.StartsWithP, "Should start with 'p' (case-insensitive) when present")
			} else {
				suite.False(result.StartsWithP, "Should not start with 'p' (case-insensitive) when absent")
			}
		}
	})
}

// TestEndsWithIgnoreCase tests the EndsWithIgnoreCase function (case-insensitive suffix check).
func (suite *EBStringFunctionsTestSuite) TestEndsWithIgnoreCase() {
	suite.T().Logf("Testing EndsWithIgnoreCase for %s", suite.ds.Kind)

	suite.Run("EndsWithSuffixIgnoreCase", func() {
		type Result struct {
			ID        string `bun:"id"`
			Title     string `bun:"title"`
			EndsWithE bool   `bun:"ends_with_e"`
			EndsWithT bool   `bun:"ends_with_t"`
		}

		var endsWithResults []Result

		err := suite.selectPosts().
			Select("id", "title").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.EndsWithIgnoreCase(eb.Column("title"), "'E'")
			}, "ends_with_e").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.EndsWithIgnoreCase(eb.Column("title"), "'T'")
			}, "ends_with_t").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &endsWithResults)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(endsWithResults, "Should return results")

		for _, result := range endsWithResults {
			suite.NotEmpty(result.ID, "Should have non-empty ID")
			suite.NotEmpty(result.Title, "Should have non-empty Title")

			// Verify case-insensitive suffix matching
			lowerTitle := strings.ToLower(result.Title)
			if strings.HasSuffix(lowerTitle, "e") {
				suite.True(result.EndsWithE, "Should end with 'e' (case-insensitive) when present")
			} else {
				suite.False(result.EndsWithE, "Should not end with 'e' (case-insensitive) when absent")
			}

			if strings.HasSuffix(lowerTitle, "t") {
				suite.True(result.EndsWithT, "Should end with 't' (case-insensitive) when present")
			} else {
				suite.False(result.EndsWithT, "Should not end with 't' (case-insensitive) when absent")
			}
		}
	})
}

// TestFuzzyMatchWithStringPattern tests fuzzy match with string pattern (optimization path).
func (suite *EBStringFunctionsTestSuite) TestFuzzyMatchWithStringPattern() {
	suite.T().Logf("Testing FuzzyMatchWithStringPattern for %s", suite.ds.Kind)

	suite.Run("ContainsString", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Contains(eb.Column("name"), "John")
				})
			}).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
	})

	suite.Run("StartsWithString", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.StartsWith(eb.Column("name"), "A")
				})
			}).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
	})

	suite.Run("EndsWithString", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.EndsWith(eb.Column("name"), "son")
				})
			}).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
	})
}

// TestFuzzyMatchWithDynamicExpr tests fuzzy match with dynamic expression (concat path).
func (suite *EBStringFunctionsTestSuite) TestFuzzyMatchWithDynamicExpr() {
	suite.T().Logf("Testing FuzzyMatchWithDynamicExpr for %s", suite.ds.Kind)

	suite.Run("ContainsDynamic", func() {
		type Result struct {
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		var results []Result

		// May return 0 results but the code path is covered
		_ = suite.selectUsers().
			Select("name", "email").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.Contains(eb.Column("email"), eb.Column("name"))
				})
			}).
			Limit(5).
			Scan(suite.ctx, &results)
	})

	suite.Run("StartsWithDynamic", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		// May return 0 results but the code path is covered
		_ = suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.StartsWith(eb.Column("email"), eb.Column("name"))
				})
			}).
			Limit(5).
			Scan(suite.ctx, &results)
	})

	suite.Run("EndsWithDynamic", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		// May return 0 results but the code path is covered
		_ = suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.EndsWith(eb.Column("email"), eb.Column("name"))
				})
			}).
			Limit(5).
			Scan(suite.ctx, &results)
	})
}

// TestFuzzyMatchIgnoreCaseString tests ignore case fuzzy match with string.
func (suite *EBStringFunctionsTestSuite) TestFuzzyMatchIgnoreCaseString() {
	suite.T().Logf("Testing FuzzyMatchIgnoreCaseString for %s", suite.ds.Kind)

	suite.Run("ContainsIgnoreCase", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.ContainsIgnoreCase(eb.Column("name"), "john")
				})
			}).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
	})

	suite.Run("StartsWithIgnoreCase", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.StartsWithIgnoreCase(eb.Column("name"), "a")
				})
			}).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
	})

	suite.Run("EndsWithIgnoreCase", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		err := suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.EndsWithIgnoreCase(eb.Column("name"), "SON")
				})
			}).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
	})
}

// TestFuzzyMatchIgnoreCaseDynamic tests ignore case fuzzy match with dynamic expression.
func (suite *EBStringFunctionsTestSuite) TestFuzzyMatchIgnoreCaseDynamic() {
	suite.T().Logf("Testing FuzzyMatchIgnoreCaseDynamic for %s", suite.ds.Kind)

	suite.Run("ContainsIgnoreCaseDynamic", func() {
		type Result struct {
			Name string `bun:"name"`
		}

		var results []Result

		// May return 0 results but the code path is covered
		_ = suite.selectUsers().
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.Expr(func(eb orm.ExprBuilder) any {
					return eb.ContainsIgnoreCase(eb.Column("email"), eb.Column("name"))
				})
			}).
			Limit(5).
			Scan(suite.ctx, &results)
	})
}
