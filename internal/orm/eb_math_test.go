package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBMathFunctionsTestSuite{BaseTestSuite: base}
	})
}

// EBMathFunctionsTestSuite tests mathematical function methods of orm.ExprBuilder.
type EBMathFunctionsTestSuite struct {
	*BaseTestSuite
}

// TestAbs tests the Abs function.
func (suite *EBMathFunctionsTestSuite) TestAbs() {
	suite.T().Logf("Testing Abs for %s", suite.ds.Kind)

	suite.Run("AbsoluteValue", func() {
		// First get average view count
		var avgViewCount float64

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.AvgColumn("view_count")
			}).
			Scan(suite.ctx, &avgViewCount)
		suite.Require().NoError(err, "Should execute query")

		type Result struct {
			Title      string `bun:"title"`
			ViewCount  int64  `bun:"view_count"`
			Difference int64  `bun:"difference"`
			AbsDiff    int64  `bun:"abs_diff"`
		}

		var results []Result

		err = suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? - ?", eb.Column("view_count"), int(avgViewCount))
			}, "difference").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Abs(eb.Expr("? - ?", eb.Column("view_count"), int(avgViewCount)))
			}, "abs_diff").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.AbsDiff >= 0, "Absolute value should always be non-negative")

			expectedAbs := result.Difference
			if expectedAbs < 0 {
				expectedAbs = -expectedAbs
			}

			suite.Equal(expectedAbs, result.AbsDiff, "Abs should return absolute value")
		}
	})
}

// TestCeil tests the Ceil function.
func (suite *EBMathFunctionsTestSuite) TestCeil() {
	suite.T().Logf("Testing Ceil for %s", suite.ds.Kind)

	suite.Run("CeilDecimalValues", func() {
		type Result struct {
			ViewCount    int64   `bun:"view_count"`
			DecimalValue float64 `bun:"decimal_value"`
			CeiledValue  float64 `bun:"ceiled_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? / 3.0", eb.Column("view_count"))
			}, "decimal_value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Ceil(eb.Expr("? / 3.0", eb.Column("view_count")))
			}, "ceiled_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.CeiledValue >= result.DecimalValue, "Ceil should be >= original value")
		}
	})
}

// TestFloor tests the Floor function.
func (suite *EBMathFunctionsTestSuite) TestFloor() {
	suite.T().Logf("Testing Floor for %s", suite.ds.Kind)

	suite.Run("FloorDecimalValues", func() {
		type Result struct {
			ViewCount    int64   `bun:"view_count"`
			DecimalValue float64 `bun:"decimal_value"`
			FlooredValue float64 `bun:"floored_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? / 3.0", eb.Column("view_count"))
			}, "decimal_value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Floor(eb.Expr("? / 3.0", eb.Column("view_count")))
			}, "floored_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.FlooredValue <= result.DecimalValue, "Floor should be <= original value")
		}
	})
}

// TestRound tests the Round function.
func (suite *EBMathFunctionsTestSuite) TestRound() {
	suite.T().Logf("Testing Round for %s", suite.ds.Kind)

	suite.Run("RoundWithDifferentPrecisions", func() {
		type Result struct {
			ViewCount     int64   `bun:"view_count"`
			RoundedNoPrec float64 `bun:"rounded_no_prec"`
			RoundedPrec1  float64 `bun:"rounded_prec1"`
			RoundedPrec2  float64 `bun:"rounded_prec2"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Round(eb.Expr("? * 1.456", eb.Column("view_count")))
			}, "rounded_no_prec").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Round(eb.Expr("? * 1.456", eb.Column("view_count")), 1)
			}, "rounded_prec1").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Round(eb.Expr("? * 1.456", eb.Column("view_count")), 2)
			}, "rounded_prec2").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.RoundedNoPrec >= 0, "Rounded value should be non-negative")
			suite.True(result.RoundedPrec1 >= 0, "Rounded value with precision 1 should be non-negative")
			suite.True(result.RoundedPrec2 >= 0, "Rounded value with precision 2 should be non-negative")
		}
	})
}

// TestTrunc tests the Trunc function.
func (suite *EBMathFunctionsTestSuite) TestTrunc() {
	suite.T().Logf("Testing Trunc for %s", suite.ds.Kind)

	suite.Run("TruncateDecimalValues", func() {
		type Result struct {
			ViewCount  int64   `bun:"view_count"`
			Divided    float64 `bun:"divided"`
			TruncValue float64 `bun:"trunc_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? / 3.0", eb.Column("view_count"))
			}, "divided").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Trunc(eb.Expr("? / 3.0", eb.Column("view_count")), 2)
			}, "trunc_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.TruncValue >= 0, "Truncated value should be non-negative")
		}
	})
}

// TestPower tests the Power function.
func (suite *EBMathFunctionsTestSuite) TestPower() {
	suite.T().Logf("Testing Power for %s", suite.ds.Kind)

	suite.Run("PowerOfNumbers", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			Squared   float64 `bun:"squared"`
			Cubed     float64 `bun:"cubed"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Power(eb.Column("view_count"), 2)
			}, "squared").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Power(eb.Column("view_count"), 3)
			}, "cubed").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Squared >= 0, "Squared value should be non-negative")
			suite.True(result.Cubed >= 0, "Cubed value should be non-negative")
		}
	})
}

// TestSqrt tests the Sqrt function.
func (suite *EBMathFunctionsTestSuite) TestSqrt() {
	suite.T().Logf("Testing Sqrt for %s", suite.ds.Kind)

	suite.Run("SquareRootOfNumbers", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			SqrtValue float64 `bun:"sqrt_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sqrt(eb.Column("view_count"))
			}, "sqrt_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.SqrtValue >= 0, "Square root should be non-negative")
		}
	})
}

// TestExp tests the Exp function.
func (suite *EBMathFunctionsTestSuite) TestExp() {
	suite.T().Logf("Testing Exp for %s", suite.ds.Kind)

	suite.Run("ExponentialFunction", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			ExpValue  float64 `bun:"exp_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Exp(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "exp_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.ExpValue > 0, "Exponential should always be positive")
		}
	})
}

// TestLn tests the Ln function.
func (suite *EBMathFunctionsTestSuite) TestLn() {
	suite.T().Logf("Testing Ln for %s", suite.ds.Kind)

	suite.Run("NaturalLogarithm", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			LnValue   float64 `bun:"ln_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Ln(eb.Column("view_count"))
			}, "ln_value").
			Where(func(cb orm.ConditionBuilder) {
				cb.GreaterThan("view_count", 0)
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.LnValue > 0, "Natural log should be positive for view_count > 1")
		}
	})
}

// TestLog tests the Log function.
func (suite *EBMathFunctionsTestSuite) TestLog() {
	suite.T().Logf("Testing Log for %s", suite.ds.Kind)

	suite.Run("LogarithmBaseTen", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			LogValue  float64 `bun:"log_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Log(10, eb.Column("view_count"))
			}, "log_value").
			Where(func(cb orm.ConditionBuilder) {
				cb.GreaterThan("view_count", 0)
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.LogValue > 0, "Log base 10 should be positive for view_count > 1")
		}
	})
}

// TestSin tests the Sin function.
func (suite *EBMathFunctionsTestSuite) TestSin() {
	suite.T().Logf("Testing Sin for %s", suite.ds.Kind)

	suite.Run("SineTrigonometric", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			SinValue  float64 `bun:"sin_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sin(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "sin_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.SinValue >= -1.0 && result.SinValue <= 1.0, "Sin value should be between -1 and 1")
		}
	})
}

// TestCos tests the Cos function.
func (suite *EBMathFunctionsTestSuite) TestCos() {
	suite.T().Logf("Testing Cos for %s", suite.ds.Kind)

	suite.Run("CosineTrigonometric", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			CosValue  float64 `bun:"cos_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Cos(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "cos_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.CosValue >= -1.0 && result.CosValue <= 1.0, "Cos value should be between -1 and 1")
		}
	})
}

// TestTan tests the Tan function.
func (suite *EBMathFunctionsTestSuite) TestTan() {
	suite.T().Logf("Testing Tan for %s", suite.ds.Kind)

	suite.Run("TangentTrigonometric", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			TanValue  float64 `bun:"tan_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Tan(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "tan_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")
	})
}

// TestAsin tests the Asin function.
func (suite *EBMathFunctionsTestSuite) TestAsin() {
	suite.T().Logf("Testing Asin for %s", suite.ds.Kind)

	suite.Run("ArcsineInverse", func() {
		type Result struct {
			Value     float64 `bun:"value"`
			AsinValue float64 `bun:"asin_value"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? / 500.0", eb.Column("view_count"))
			}, "value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Asin(eb.Expr("? / 500.0", eb.Column("view_count")))
			}, "asin_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Value >= -1.0 && result.Value <= 1.0, "Value should be in valid range for asin")
		}
	})
}

// TestAcos tests the Acos function.
func (suite *EBMathFunctionsTestSuite) TestAcos() {
	suite.T().Logf("Testing Acos for %s", suite.ds.Kind)

	suite.Run("ArccosineInverse", func() {
		type Result struct {
			Value     float64 `bun:"value"`
			AcosValue float64 `bun:"acos_value"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? / 500.0", eb.Column("view_count"))
			}, "value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Acos(eb.Expr("? / 500.0", eb.Column("view_count")))
			}, "acos_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Value >= -1.0 && result.Value <= 1.0, "Value should be in valid range for acos")
		}
	})
}

// TestAtan tests the Atan function.
func (suite *EBMathFunctionsTestSuite) TestAtan() {
	suite.T().Logf("Testing Atan for %s", suite.ds.Kind)

	suite.Run("ArctangentInverse", func() {
		type Result struct {
			Value     float64 `bun:"value"`
			AtanValue float64 `bun:"atan_value"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("? / 200.0", eb.Column("view_count"))
			}, "value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Atan(eb.Expr("? / 200.0", eb.Column("view_count")))
			}, "atan_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")
	})
}

// TestPi tests the Pi function.
func (suite *EBMathFunctionsTestSuite) TestPi() {
	suite.T().Logf("Testing Pi for %s", suite.ds.Kind)

	suite.Run("PiConstant", func() {
		type Result struct {
			PiValue       float64 `bun:"pi_value"`
			CircleArea    float64 `bun:"circle_area"`
			Circumference float64 `bun:"circumference"`
		}

		var results []Result

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Pi()
			}, "pi_value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// Area = π * r²
				return eb.Expr("? * ? * ?",
					eb.Pi(),
					eb.Column("view_count"),
					eb.Column("view_count"))
			}, "circle_area").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// Circumference = 2 * π * r
				return eb.Expr("2 * ? * ?", eb.Pi(), eb.Column("view_count"))
			}, "circumference").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.InDelta(3.14159, result.PiValue, 0.001, "Pi value should be approximately 3.14159")
			suite.True(result.CircleArea > 0, "Circle area should be positive")
			suite.True(result.Circumference > 0, "Circumference should be positive")
		}
	})
}

// TestRandom tests the Random function.
func (suite *EBMathFunctionsTestSuite) TestRandom() {
	suite.T().Logf("Testing Random for %s", suite.ds.Kind)

	suite.Run("RandomNumberGeneration", func() {
		type Result struct {
			ID        string  `bun:"id"`
			RandomVal float64 `bun:"random_val"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("id").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Random()
			}, "random_val").
			OrderBy("id").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.RandomVal >= 0 && result.RandomVal < 1, "Random should be in [0, 1)")
		}
	})
}

// TestSign tests the Sign function.
func (suite *EBMathFunctionsTestSuite) TestSign() {
	suite.T().Logf("Testing Sign for %s", suite.ds.Kind)

	suite.Run("SignFunction", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			SignValue float64 `bun:"sign_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sign(eb.Expr("? - 50", eb.Column("view_count")))
			}, "sign_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.Contains([]float64{-1.0, 0.0, 1.0}, result.SignValue, "Sign should be -1, 0, or 1")
		}
	})
}

// TestMod tests the Mod function.
func (suite *EBMathFunctionsTestSuite) TestMod() {
	suite.T().Logf("Testing Mod for %s", suite.ds.Kind)

	suite.Run("ModuloOperation", func() {
		type Result struct {
			ViewCount int64 `bun:"view_count"`
			Mod5      int64 `bun:"mod5"`
			Mod10     int64 `bun:"mod10"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Mod(eb.Column("view_count"), 5)
			}, "mod5").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Mod(eb.Column("view_count"), 10)
			}, "mod10").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Mod5 >= 0 && result.Mod5 < 5, "Mod 5 should be between 0 and 4")
			suite.True(result.Mod10 >= 0 && result.Mod10 < 10, "Mod 10 should be between 0 and 9")
		}
	})
}

// TestGreatest tests the Greatest function.
func (suite *EBMathFunctionsTestSuite) TestGreatest() {
	suite.T().Logf("Testing Greatest for %s", suite.ds.Kind)

	suite.Run("GreatestValue", func() {
		type Result struct {
			ViewCount int64 `bun:"view_count"`
			Greatest  int64 `bun:"greatest"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Greatest(eb.Column("view_count"), 20, 30, 40)
			}, "greatest").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Greatest >= result.ViewCount, "Greatest should be >= view_count")
		}
	})
}

// TestLeast tests the Least function.
func (suite *EBMathFunctionsTestSuite) TestLeast() {
	suite.T().Logf("Testing Least for %s", suite.ds.Kind)

	suite.Run("LeastValue", func() {
		type Result struct {
			ViewCount int64 `bun:"view_count"`
			Least     int64 `bun:"least"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Least(eb.Column("view_count"), 20, 30, 40)
			}, "least").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.Least <= result.ViewCount, "Least should be <= view_count")
		}
	})
}

// TestCombinedMathFunctions tests multiple math functions working together.
func (suite *EBMathFunctionsTestSuite) TestCombinedMathFunctions() {
	suite.T().Logf("Testing CombinedMathFunctions for %s", suite.ds.Kind)

	suite.Run("BasicMathCombination", func() {
		type Result struct {
			Title        string  `bun:"title"`
			ViewCount    int64   `bun:"view_count"`
			AbsViewCount int64   `bun:"abs_view_count"`
			CeiledViews  float64 `bun:"ceiled_views"`
			FlooredViews float64 `bun:"floored_views"`
			RoundedViews float64 `bun:"rounded_views"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Abs(eb.Expr("? - 100", eb.Column("view_count")))
			}, "abs_view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Ceil(eb.Expr("? / 10.0", eb.Column("view_count")))
			}, "ceiled_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Floor(eb.Expr("? / 10.0", eb.Column("view_count")))
			}, "floored_views").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Round(eb.Expr("? / 10.0", eb.Column("view_count")), 1)
			}, "rounded_views").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.AbsViewCount >= 0, "Abs should be non-negative")
			suite.True(result.CeiledViews >= result.FlooredViews, "Ceil should be >= Floor")
			suite.T().Logf("Post %s: ViewCount=%d, Abs=%d, Ceil=%.0f, Floor=%.0f, Round=%.1f",
				result.Title, result.ViewCount, result.AbsViewCount,
				result.CeiledViews, result.FlooredViews, result.RoundedViews)
		}
	})

	suite.Run("AdvancedMathCombination", func() {
		type Result struct {
			Title       string  `bun:"title"`
			ViewCount   int64   `bun:"view_count"`
			PowerResult float64 `bun:"power_result"`
			SqrtResult  float64 `bun:"sqrt_result"`
			ModResult   int64   `bun:"mod_result"`
			SignResult  float64 `bun:"sign_result"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("title", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Power(eb.Column("view_count"), 2)
			}, "power_result").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sqrt(eb.Column("view_count"))
			}, "sqrt_result").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Mod(eb.Column("view_count"), 7)
			}, "mod_result").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sign(eb.Expr("? - 50", eb.Column("view_count")))
			}, "sign_result").
			OrderBy("title").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.PowerResult >= 0, "Power result should be non-negative")
			suite.True(result.SqrtResult >= 0, "Sqrt result should be non-negative")
			suite.True(result.ModResult >= 0 && result.ModResult < 7, "Mod result should be between 0 and 6")
			suite.Contains([]float64{-1.0, 0.0, 1.0}, result.SignResult, "Sign result should be -1, 0, or 1")
			suite.T().Logf("Post %s: ViewCount=%d, Power=%.0f, Sqrt=%.2f, Mod=%d, Sign=%.0f",
				result.Title, result.ViewCount, result.PowerResult, result.SqrtResult,
				result.ModResult, result.SignResult)
		}
	})

	suite.Run("TrigonometricCombination", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			SinValue  float64 `bun:"sin_value"`
			CosValue  float64 `bun:"cos_value"`
			TanValue  float64 `bun:"tan_value"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sin(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "sin_value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Cos(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "cos_value").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Tan(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "tan_value").
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.SinValue >= -1.0 && result.SinValue <= 1.0, "Sin value should be between -1 and 1")
			suite.True(result.CosValue >= -1.0 && result.CosValue <= 1.0, "Cos value should be between -1 and 1")
		}
	})

	suite.Run("LogarithmicCombination", func() {
		type Result struct {
			ViewCount int64   `bun:"view_count"`
			SinLn     float64 `bun:"sin_ln"`
			CosExp    float64 `bun:"cos_exp"`
			TanLog    float64 `bun:"tan_log"`
		}

		var results []Result

		err := suite.selectPosts().
			Select("view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Sin(eb.Ln(eb.Column("view_count")))
			}, "sin_ln").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Cos(eb.Expr("? / 100.0", eb.Column("view_count")))
			}, "cos_exp").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Tan(eb.Log(10, eb.Column("view_count")))
			}, "tan_log").
			Where(func(cb orm.ConditionBuilder) {
				cb.GreaterThan("view_count", 1)
			}).
			OrderBy("view_count").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Should execute query")
		suite.Require().NotEmpty(results, "Should return results")

		for _, result := range results {
			suite.True(result.SinLn >= -1.0 && result.SinLn <= 1.0, "Sin(Ln) should be between -1 and 1")
			suite.True(result.CosExp >= -1.0 && result.CosExp <= 1.0, "Cos should be between -1 and 1")
		}
	})
}
