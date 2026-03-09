package orm_test

import (
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &EBDateTimeFunctionsTestSuite{BaseTestSuite: base}
	})
}

// EBDateTimeFunctionsTestSuite tests date and time manipulation methods of orm.ExprBuilder.
type EBDateTimeFunctionsTestSuite struct {
	*BaseTestSuite
}

// TestCurrentDate tests the CurrentDate function.
func (suite *EBDateTimeFunctionsTestSuite) TestCurrentDate() {
	suite.T().Logf("Testing CurrentDate function for %s", suite.ds.Kind)

	suite.Run("GetCurrentDate", func() {
		type CurrentDateResult struct {
			CurrentDate time.Time `bun:"current_date"`
		}

		var result CurrentDateResult

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CurrentDate()
			}, "current_date").
			Limit(1).
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "CurrentDate query should execute successfully")
		suite.NotZero(result.CurrentDate, "CurrentDate should return a non-zero date value")

		now := time.Now()
		suite.True(result.CurrentDate.Year() >= now.Year()-1, "CurrentDate year should be within reasonable range (current year or previous year)")

		suite.T().Logf("CurrentDate: %v", result.CurrentDate)
	})
}

// TestCurrentTime tests the CurrentTime function.
func (suite *EBDateTimeFunctionsTestSuite) TestCurrentTime() {
	suite.T().Logf("Testing CurrentTime function for %s", suite.ds.Kind)

	suite.Run("GetCurrentTime", func() {
		type CurrentTimeResult struct {
			CurrentTime time.Time `bun:"current_time"`
		}

		var result CurrentTimeResult

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CurrentTime()
			}, "current_time").
			Limit(1).
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "CurrentTime query should execute successfully")
		suite.NotZero(result.CurrentTime, "CurrentTime should return a non-zero time value")

		suite.T().Logf("CurrentTime: %v", result.CurrentTime)
	})
}

// TestCurrentTimestamp tests the CurrentTimestamp function.
func (suite *EBDateTimeFunctionsTestSuite) TestCurrentTimestamp() {
	suite.T().Logf("Testing CurrentTimestamp function for %s", suite.ds.Kind)

	suite.Run("GetCurrentTimestamp", func() {
		type CurrentTimestampResult struct {
			CurrentTimestamp time.Time `bun:"current_timestamp"`
		}

		var result CurrentTimestampResult

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CurrentTimestamp()
			}, "current_timestamp").
			Limit(1).
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "CurrentTimestamp query should execute successfully")
		suite.NotZero(result.CurrentTimestamp, "CurrentTimestamp should return a non-zero timestamp value")

		now := time.Now()
		suite.True(result.CurrentTimestamp.Year() >= now.Year()-1, "CurrentTimestamp year should be within reasonable range (current year or previous year)")

		suite.T().Logf("CurrentTimestamp: %v", result.CurrentTimestamp)
	})
}

// TestNow tests the Now function.
func (suite *EBDateTimeFunctionsTestSuite) TestNow() {
	suite.T().Logf("Testing Now function for %s", suite.ds.Kind)

	suite.Run("GetNow", func() {
		type NowResult struct {
			Now time.Time `bun:"now"`
		}

		var result NowResult

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Now()
			}, "now").
			Limit(1).
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Now query should execute successfully")
		suite.NotZero(result.Now, "Now should return a non-zero timestamp value")

		now := time.Now()
		suite.True(result.Now.Year() >= now.Year()-1, "Now year should be within reasonable range (current year or previous year)")

		suite.T().Logf("Now: %v", result.Now)
	})

	suite.Run("AllCurrentTimeFunctions", func() {
		type AllCurrentResult struct {
			CurrentDate      time.Time `bun:"current_date"`
			CurrentTime      time.Time `bun:"current_time"`
			CurrentTimestamp time.Time `bun:"current_timestamp"`
			Now              time.Time `bun:"now"`
		}

		var result AllCurrentResult

		err := suite.selectPosts().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CurrentDate()
			}, "current_date").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CurrentTime()
			}, "current_time").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.CurrentTimestamp()
			}, "current_timestamp").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Now()
			}, "now").
			Limit(1).
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Combined current time functions query should execute successfully")
		suite.NotZero(result.CurrentDate, "CurrentDate should return a non-zero value in combined query")
		suite.NotZero(result.CurrentTime, "CurrentTime should return a non-zero value in combined query")
		suite.NotZero(result.CurrentTimestamp, "CurrentTimestamp should return a non-zero value in combined query")
		suite.NotZero(result.Now, "Now should return a non-zero value in combined query")

		suite.T().Logf("CurrentDate: %v | CurrentTime: %v | CurrentTimestamp: %v | Now: %v",
			result.CurrentDate, result.CurrentTime, result.CurrentTimestamp, result.Now)
	})
}

// TestExtractYear tests the ExtractYear function.
func (suite *EBDateTimeFunctionsTestSuite) TestExtractYear() {
	suite.T().Logf("Testing ExtractYear function for %s", suite.ds.Kind)

	suite.Run("ExtractYearFromCreatedAt", func() {
		type YearResult struct {
			CreatedAt time.Time `bun:"created_at"`
			Year      int64     `bun:"year"`
		}

		var results []YearResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractYear(eb.Column("created_at"))
			}, "year").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "ExtractYear query should execute successfully")
		suite.Require().NotEmpty(results, "ExtractYear should return at least one result")

		for _, result := range results {
			suite.Equal(int64(result.CreatedAt.Year()), result.Year, "Extracted year should match the actual year from created_at timestamp")
			suite.T().Logf("CreatedAt: %v | Year: %d", result.CreatedAt, result.Year)
		}
	})
}

// TestExtractMonth tests the ExtractMonth function.
func (suite *EBDateTimeFunctionsTestSuite) TestExtractMonth() {
	suite.T().Logf("Testing ExtractMonth function for %s", suite.ds.Kind)

	suite.Run("ExtractMonthFromCreatedAt", func() {
		type MonthResult struct {
			CreatedAt time.Time `bun:"created_at"`
			Month     int64     `bun:"month"`
		}

		var results []MonthResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractMonth(eb.Column("created_at"))
			}, "month").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "ExtractMonth query should execute successfully")
		suite.Require().NotEmpty(results, "ExtractMonth should return at least one result")

		for _, result := range results {
			suite.Equal(int64(result.CreatedAt.Month()), result.Month, "Extracted month should match the actual month from created_at timestamp")
			suite.True(result.Month >= 1 && result.Month <= 12, "Extracted month should be in valid range (1-12)")
			suite.T().Logf("CreatedAt: %v | Month: %d", result.CreatedAt, result.Month)
		}
	})
}

// TestExtractDay tests the ExtractDay function.
func (suite *EBDateTimeFunctionsTestSuite) TestExtractDay() {
	suite.T().Logf("Testing ExtractDay function for %s", suite.ds.Kind)

	suite.Run("ExtractDayFromCreatedAt", func() {
		type DayResult struct {
			CreatedAt time.Time `bun:"created_at"`
			Day       int64     `bun:"day"`
		}

		var results []DayResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractDay(eb.Column("created_at"))
			}, "day").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "ExtractDay query should execute successfully")
		suite.Require().NotEmpty(results, "ExtractDay should return at least one result")

		for _, result := range results {
			suite.Equal(int64(result.CreatedAt.Day()), result.Day, "Extracted day should match the actual day from created_at timestamp")
			suite.True(result.Day >= 1 && result.Day <= 31, "Extracted day should be in valid range (1-31)")
			suite.T().Logf("CreatedAt: %v | Day: %d", result.CreatedAt, result.Day)
		}
	})
}

// TestExtractHour tests the ExtractHour function.
func (suite *EBDateTimeFunctionsTestSuite) TestExtractHour() {
	suite.T().Logf("Testing ExtractHour function for %s", suite.ds.Kind)

	suite.Run("ExtractHourFromCreatedAt", func() {
		type HourResult struct {
			CreatedAt time.Time `bun:"created_at"`
			Hour      int64     `bun:"hour"`
		}

		var results []HourResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractHour(eb.Column("created_at"))
			}, "hour").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "ExtractHour query should execute successfully")
		suite.Require().NotEmpty(results, "ExtractHour should return at least one result")

		for _, result := range results {
			suite.Equal(int64(result.CreatedAt.Hour()), result.Hour, "Extracted hour should match the actual hour from created_at timestamp")
			suite.True(result.Hour >= 0 && result.Hour < 24, "Extracted hour should be in valid range (0-23)")
			suite.T().Logf("CreatedAt: %v | Hour: %d", result.CreatedAt, result.Hour)
		}
	})
}

// TestExtractMinute tests the ExtractMinute function.
func (suite *EBDateTimeFunctionsTestSuite) TestExtractMinute() {
	suite.T().Logf("Testing ExtractMinute function for %s", suite.ds.Kind)

	suite.Run("ExtractMinuteFromCreatedAt", func() {
		type MinuteResult struct {
			CreatedAt time.Time `bun:"created_at"`
			Minute    int64     `bun:"minute"`
		}

		var results []MinuteResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractMinute(eb.Column("created_at"))
			}, "minute").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "ExtractMinute query should execute successfully")
		suite.Require().NotEmpty(results, "ExtractMinute should return at least one result")

		for _, result := range results {
			suite.Equal(int64(result.CreatedAt.Minute()), result.Minute, "Extracted minute should match the actual minute from created_at timestamp")
			suite.True(result.Minute >= 0 && result.Minute < 60, "Extracted minute should be in valid range (0-59)")
			suite.T().Logf("CreatedAt: %v | Minute: %d", result.CreatedAt, result.Minute)
		}
	})
}

// TestExtractSecond tests the ExtractSecond function.
func (suite *EBDateTimeFunctionsTestSuite) TestExtractSecond() {
	suite.T().Logf("Testing ExtractSecond function for %s", suite.ds.Kind)

	suite.Run("ExtractSecondFromCreatedAt", func() {
		type SecondResult struct {
			ID        string    `bun:"id"`
			CreatedAt time.Time `bun:"created_at"`
			Second    float64   `bun:"second"`
		}

		var results []SecondResult

		err := suite.selectPosts().
			Select("id", "created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractSecond(eb.Column("created_at"))
			}, "second").
			OrderBy("id").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "ExtractSecond query should execute successfully")
		suite.Require().NotEmpty(results, "ExtractSecond should return at least one result")

		for _, result := range results {
			suite.True(result.Second >= 0 && result.Second < 60, "Extracted second should be in valid range (0-59)")
			suite.T().Logf("ID: %s, CreatedAt: %s, Second: %.0f",
				result.ID, result.CreatedAt.Format(time.RFC3339), result.Second)
		}
	})

	suite.Run("AllExtractFunctions", func() {
		type AllExtractResult struct {
			CreatedAt time.Time `bun:"created_at"`
			Year      int64     `bun:"year"`
			Month     int64     `bun:"month"`
			Day       int64     `bun:"day"`
			Hour      int64     `bun:"hour"`
			Minute    int64     `bun:"minute"`
			Second    float64   `bun:"second"`
		}

		var results []AllExtractResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractYear(eb.Column("created_at"))
			}, "year").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractMonth(eb.Column("created_at"))
			}, "month").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractDay(eb.Column("created_at"))
			}, "day").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractHour(eb.Column("created_at"))
			}, "hour").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractMinute(eb.Column("created_at"))
			}, "minute").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractSecond(eb.Column("created_at"))
			}, "second").
			OrderBy("created_at").
			Limit(3).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Combined extract functions query should execute successfully")
		suite.Require().NotEmpty(results, "Combined extract query should return at least one result")

		for _, result := range results {
			suite.Equal(int64(result.CreatedAt.Year()), result.Year, "Extracted year should match actual year")
			suite.Equal(int64(result.CreatedAt.Month()), result.Month, "Extracted month should match actual month")
			suite.Equal(int64(result.CreatedAt.Day()), result.Day, "Extracted day should match actual day")
			suite.Equal(int64(result.CreatedAt.Hour()), result.Hour, "Extracted hour should match actual hour")
			suite.Equal(int64(result.CreatedAt.Minute()), result.Minute, "Extracted minute should match actual minute")
			suite.True(result.Year >= 2000 && result.Year <= 3000, "Extracted year should be in reasonable range (2000-3000)")
			suite.True(result.Month >= 1 && result.Month <= 12, "Extracted month should be in valid range (1-12)")
			suite.True(result.Day >= 1 && result.Day <= 31, "Extracted day should be in valid range (1-31)")
			suite.True(result.Hour >= 0 && result.Hour < 24, "Extracted hour should be in valid range (0-23)")
			suite.True(result.Minute >= 0 && result.Minute < 60, "Extracted minute should be in valid range (0-59)")
			suite.True(result.Second >= 0 && result.Second < 60, "Extracted second should be in valid range (0-59)")

			suite.T().Logf("CreatedAt: %s, Y:%d M:%d D:%d H:%d M:%d S:%.0f",
				result.CreatedAt.Format(time.RFC3339),
				result.Year, result.Month, result.Day, result.Hour, result.Minute, result.Second)
		}
	})
}

// TestDateTrunc tests the DateTrunc function.
func (suite *EBDateTimeFunctionsTestSuite) TestDateTrunc() {
	suite.T().Logf("Testing DateTrunc function for %s", suite.ds.Kind)

	suite.Run("TruncateToDifferentPrecisions", func() {
		type TruncResult struct {
			CreatedAt  time.Time `bun:"created_at"`
			TruncYear  time.Time `bun:"trunc_year"`
			TruncMonth time.Time `bun:"trunc_month"`
			TruncDay   time.Time `bun:"trunc_day"`
			TruncHour  time.Time `bun:"trunc_hour"`
		}

		var results []TruncResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateTrunc(orm.UnitYear, eb.Column("created_at"))
			}, "trunc_year").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateTrunc(orm.UnitMonth, eb.Column("created_at"))
			}, "trunc_month").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateTrunc(orm.UnitDay, eb.Column("created_at"))
			}, "trunc_day").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateTrunc(orm.UnitHour, eb.Column("created_at"))
			}, "trunc_hour").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateTrunc query should execute successfully")
		suite.Require().NotEmpty(results, "DateTrunc should return at least one result")

		for _, result := range results {
			// Verify truncated dates are on or before original
			suite.True(!result.TruncYear.After(result.CreatedAt), "Truncated year should not be after original timestamp")
			suite.True(!result.TruncMonth.After(result.CreatedAt), "Truncated month should not be after original timestamp")
			suite.True(!result.TruncDay.After(result.CreatedAt), "Truncated day should not be after original timestamp")
			suite.True(!result.TruncHour.After(result.CreatedAt), "Truncated hour should not be after original timestamp")

			suite.T().Logf("Original: %v | Year: %v | Month: %v | Day: %v | Hour: %v",
				result.CreatedAt, result.TruncYear, result.TruncMonth, result.TruncDay, result.TruncHour)
		}
	})
}

// TestDateAdd tests the DateAdd function.
func (suite *EBDateTimeFunctionsTestSuite) TestDateAdd() {
	suite.T().Logf("Testing DateAdd function for %s", suite.ds.Kind)

	suite.Run("AddDateIntervals", func() {
		type DateAddResult struct {
			CreatedAt   time.Time `bun:"created_at"`
			AddedDays   time.Time `bun:"added_days"`
			AddedMonths time.Time `bun:"added_months"`
			AddedYears  time.Time `bun:"added_years"`
		}

		var results []DateAddResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateAdd(eb.Column("created_at"), 7, orm.UnitDay)
			}, "added_days").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateAdd(eb.Column("created_at"), 3, orm.UnitMonth)
			}, "added_months").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateAdd(eb.Column("created_at"), 1, orm.UnitYear)
			}, "added_years").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateAdd query should execute successfully")
		suite.Require().NotEmpty(results, "DateAdd should return at least one result")

		for _, result := range results {
			suite.True(result.AddedDays.After(result.CreatedAt), "Date with added days should be after original timestamp")
			suite.True(result.AddedMonths.After(result.CreatedAt), "Date with added months should be after original timestamp")
			suite.True(result.AddedYears.After(result.CreatedAt), "Date with added years should be after original timestamp")

			suite.T().Logf("Original: %v | +7 days: %v | +3 months: %v | +1 year: %v",
				result.CreatedAt, result.AddedDays, result.AddedMonths, result.AddedYears)
		}
	})

	suite.Run("AddTimeIntervals", func() {
		type TimeAddResult struct {
			CreatedAt    time.Time `bun:"created_at"`
			AddedSeconds time.Time `bun:"added_seconds"`
			AddedMinutes time.Time `bun:"added_minutes"`
			AddedHours   time.Time `bun:"added_hours"`
		}

		var results []TimeAddResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateAdd(eb.Column("created_at"), 30, orm.UnitSecond)
			}, "added_seconds").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateAdd(eb.Column("created_at"), 45, orm.UnitMinute)
			}, "added_minutes").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateAdd(eb.Column("created_at"), 2, orm.UnitHour)
			}, "added_hours").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateAdd with time units should execute successfully")
		suite.Require().NotEmpty(results, "DateAdd should return at least one result")

		for _, result := range results {
			suite.True(result.AddedSeconds.After(result.CreatedAt), "Date with added seconds should be after original timestamp")
			suite.True(result.AddedMinutes.After(result.CreatedAt), "Date with added minutes should be after original timestamp")
			suite.True(result.AddedHours.After(result.CreatedAt), "Date with added hours should be after original timestamp")

			suite.T().Logf("Original: %v | +30s: %v | +45m: %v | +2h: %v",
				result.CreatedAt, result.AddedSeconds, result.AddedMinutes, result.AddedHours)
		}
	})

	suite.Run("AddDynamicColumnInterval", func() {
		type DynamicAddResult struct {
			CreatedAt time.Time `bun:"created_at"`
			ViewCount int       `bun:"view_count"`
			AddedDays time.Time `bun:"added_days"`
		}

		var results []DynamicAddResult

		err := suite.selectPosts().
			Select("created_at", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateAdd(eb.Column("created_at"), eb.Column("view_count"), orm.UnitDay)
			}, "added_days").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateAdd with column interval should execute successfully")
		suite.Require().NotEmpty(results, "Should return results")

		for _, r := range results {
			expected := r.CreatedAt.Add(time.Duration(r.ViewCount) * 24 * time.Hour)
			suite.InDelta(expected.Unix(), r.AddedDays.Unix(), 1,
				"DateAdd with column interval should produce correct result (view_count=%d)", r.ViewCount)

			suite.T().Logf("Original: %v | ViewCount: %d | +%d days: %v",
				r.CreatedAt, r.ViewCount, r.ViewCount, r.AddedDays)
		}
	})

	suite.Run("AddComputedExpressionInterval", func() {
		type ComputedAddResult struct {
			CreatedAt  time.Time `bun:"created_at"`
			ViewCount  int       `bun:"view_count"`
			AddedHours time.Time `bun:"added_hours"`
		}

		var results []ComputedAddResult

		err := suite.selectPosts().
			Select("created_at", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// Use a computed expression: Abs(view_count) as dynamic interval
				return eb.DateAdd(eb.Column("created_at"), eb.Abs(eb.Column("view_count")), orm.UnitHour)
			}, "added_hours").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateAdd with computed expression interval should execute successfully")
		suite.Require().NotEmpty(results, "Should return results")

		for _, r := range results {
			expected := r.CreatedAt.Add(time.Duration(r.ViewCount) * time.Hour)
			suite.InDelta(expected.Unix(), r.AddedHours.Unix(), 1,
				"DateAdd with Abs(column) interval should produce correct result (view_count=%d)", r.ViewCount)

			suite.T().Logf("Original: %v | Abs(ViewCount): %d | +%d hours: %v",
				r.CreatedAt, r.ViewCount, r.ViewCount, r.AddedHours)
		}
	})
}

// TestDateSubtract tests the DateSubtract function.
func (suite *EBDateTimeFunctionsTestSuite) TestDateSubtract() {
	suite.T().Logf("Testing DateSubtract function for %s", suite.ds.Kind)

	suite.Run("SubtractDateIntervals", func() {
		type DateSubtractResult struct {
			CreatedAt        time.Time `bun:"created_at"`
			SubtractedDays   time.Time `bun:"subtracted_days"`
			SubtractedMonths time.Time `bun:"subtracted_months"`
			SubtractedYears  time.Time `bun:"subtracted_years"`
		}

		var results []DateSubtractResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), 5, orm.UnitDay)
			}, "subtracted_days").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), 2, orm.UnitMonth)
			}, "subtracted_months").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), 1, orm.UnitYear)
			}, "subtracted_years").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateSubtract query should execute successfully")
		suite.Require().NotEmpty(results, "DateSubtract should return at least one result")

		for _, result := range results {
			suite.True(result.SubtractedDays.Before(result.CreatedAt), "Date with subtracted days should be before original timestamp")
			suite.True(result.SubtractedMonths.Before(result.CreatedAt), "Date with subtracted months should be before original timestamp")
			suite.True(result.SubtractedYears.Before(result.CreatedAt), "Date with subtracted years should be before original timestamp")

			suite.T().Logf("Original: %v | -5 days: %v | -2 months: %v | -1 year: %v",
				result.CreatedAt, result.SubtractedDays, result.SubtractedMonths, result.SubtractedYears)
		}
	})

	suite.Run("SubtractTimeIntervals", func() {
		type TimeSubtractResult struct {
			CreatedAt         time.Time `bun:"created_at"`
			SubtractedSeconds time.Time `bun:"subtracted_seconds"`
			SubtractedMinutes time.Time `bun:"subtracted_minutes"`
			SubtractedHours   time.Time `bun:"subtracted_hours"`
		}

		var results []TimeSubtractResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), 45, orm.UnitSecond)
			}, "subtracted_seconds").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), 30, orm.UnitMinute)
			}, "subtracted_minutes").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), 3, orm.UnitHour)
			}, "subtracted_hours").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateSubtract with time units should execute successfully")
		suite.Require().NotEmpty(results, "DateSubtract should return at least one result")

		for _, result := range results {
			suite.True(result.SubtractedSeconds.Before(result.CreatedAt), "Date with subtracted seconds should be before original timestamp")
			suite.True(result.SubtractedMinutes.Before(result.CreatedAt), "Date with subtracted minutes should be before original timestamp")
			suite.True(result.SubtractedHours.Before(result.CreatedAt), "Date with subtracted hours should be before original timestamp")

			suite.T().Logf("Original: %v | -45s: %v | -30m: %v | -3h: %v",
				result.CreatedAt, result.SubtractedSeconds, result.SubtractedMinutes, result.SubtractedHours)
		}
	})

	suite.Run("SubtractDynamicColumnInterval", func() {
		type DynamicSubResult struct {
			CreatedAt      time.Time `bun:"created_at"`
			ViewCount      int       `bun:"view_count"`
			SubtractedDays time.Time `bun:"subtracted_days"`
		}

		var results []DynamicSubResult

		err := suite.selectPosts().
			Select("created_at", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), eb.Column("view_count"), orm.UnitDay)
			}, "subtracted_days").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateSubtract with column interval should execute successfully")
		suite.Require().NotEmpty(results, "Should return results")

		for _, r := range results {
			expected := r.CreatedAt.Add(-time.Duration(r.ViewCount) * 24 * time.Hour)
			suite.InDelta(expected.Unix(), r.SubtractedDays.Unix(), 1,
				"DateSubtract with column interval should produce correct result (view_count=%d)", r.ViewCount)

			suite.T().Logf("Original: %v | ViewCount: %d | -%d days: %v",
				r.CreatedAt, r.ViewCount, r.ViewCount, r.SubtractedDays)
		}
	})

	suite.Run("SubtractComputedExpressionInterval", func() {
		type ComputedSubResult struct {
			CreatedAt       time.Time `bun:"created_at"`
			ViewCount       int       `bun:"view_count"`
			SubtractedHours time.Time `bun:"subtracted_hours"`
		}

		var results []ComputedSubResult

		err := suite.selectPosts().
			Select("created_at", "view_count").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateSubtract(eb.Column("created_at"), eb.Abs(eb.Column("view_count")), orm.UnitHour)
			}, "subtracted_hours").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateSubtract with computed expression interval should execute successfully")
		suite.Require().NotEmpty(results, "Should return results")

		for _, r := range results {
			expected := r.CreatedAt.Add(-time.Duration(r.ViewCount) * time.Hour)
			suite.InDelta(expected.Unix(), r.SubtractedHours.Unix(), 1,
				"DateSubtract with Abs(column) interval should produce correct result (view_count=%d)", r.ViewCount)

			suite.T().Logf("Original: %v | Abs(ViewCount): %d | -%d hours: %v",
				r.CreatedAt, r.ViewCount, r.ViewCount, r.SubtractedHours)
		}
	})
}

// TestDateDiff tests the DateDiff function.
func (suite *EBDateTimeFunctionsTestSuite) TestDateDiff() {
	suite.T().Logf("Testing DateDiff function for %s", suite.ds.Kind)

	suite.Run("CalculateTimeDifferences", func() {
		type TimeDiffResult struct {
			CreatedAt   time.Time `bun:"created_at"`
			UpdatedAt   time.Time `bun:"updated_at"`
			SecondsDiff float64   `bun:"seconds_diff"`
			MinutesDiff float64   `bun:"minutes_diff"`
			HoursDiff   float64   `bun:"hours_diff"`
		}

		var results []TimeDiffResult

		err := suite.selectPosts().
			Select("created_at", "updated_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateDiff(eb.Column("created_at"), eb.Column("updated_at"), orm.UnitSecond)
			}, "seconds_diff").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateDiff(eb.Column("created_at"), eb.Column("updated_at"), orm.UnitMinute)
			}, "minutes_diff").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateDiff(eb.Column("created_at"), eb.Column("updated_at"), orm.UnitHour)
			}, "hours_diff").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateDiff query should execute successfully")
		suite.Require().NotEmpty(results, "DateDiff should return at least one result")

		for _, result := range results {
			// Verify that seconds, minutes, and hours maintain consistent relationships
			// The sign should be consistent (all positive or all negative based on date order)
			if result.CreatedAt.Before(result.UpdatedAt) {
				suite.True(result.SecondsDiff >= 0, "DateDiff in seconds should be non-negative when created_at is before updated_at")
				suite.True(result.MinutesDiff >= 0, "DateDiff in minutes should be non-negative when created_at is before updated_at")
				suite.True(result.HoursDiff >= 0, "DateDiff in hours should be non-negative when created_at is before updated_at")
			} else if result.CreatedAt.After(result.UpdatedAt) {
				suite.True(result.SecondsDiff <= 0, "DateDiff in seconds should be non-positive when created_at is after updated_at")
				suite.True(result.MinutesDiff <= 0, "DateDiff in minutes should be non-positive when created_at is after updated_at")
				suite.True(result.HoursDiff <= 0, "DateDiff in hours should be non-positive when created_at is after updated_at")
			}

			suite.T().Logf("Created: %v | Updated: %v | Seconds: %.2f | Minutes: %.2f | Hours: %.2f",
				result.CreatedAt, result.UpdatedAt, result.SecondsDiff, result.MinutesDiff, result.HoursDiff)
		}
	})

	suite.Run("CalculateDateDifferences", func() {
		type DateDiffResult struct {
			CreatedAt  time.Time `bun:"created_at"`
			UpdatedAt  time.Time `bun:"updated_at"`
			DaysDiff   float64   `bun:"days_diff"`
			MonthsDiff float64   `bun:"months_diff"`
			YearsDiff  float64   `bun:"years_diff"`
		}

		var results []DateDiffResult

		err := suite.selectPosts().
			Select("created_at", "updated_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateDiff(eb.Column("created_at"), eb.Column("updated_at"), orm.UnitDay)
			}, "days_diff").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateDiff(eb.Column("created_at"), eb.Column("updated_at"), orm.UnitMonth)
			}, "months_diff").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateDiff(eb.Column("created_at"), eb.Column("updated_at"), orm.UnitYear)
			}, "years_diff").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "DateDiff query should execute successfully")
		suite.Require().NotEmpty(results, "DateDiff should return at least one result")

		for _, result := range results {
			// Verify that days, months, and years maintain consistent relationships
			// The sign should be consistent (all positive or all negative based on date order)
			if result.CreatedAt.Before(result.UpdatedAt) {
				suite.True(result.DaysDiff >= 0, "DateDiff in days should be non-negative when created_at is before updated_at")
				suite.True(result.MonthsDiff >= 0, "DateDiff in months should be non-negative when created_at is before updated_at")
				suite.True(result.YearsDiff >= 0, "DateDiff in years should be non-negative when created_at is before updated_at")
			} else if result.CreatedAt.After(result.UpdatedAt) {
				suite.True(result.DaysDiff <= 0, "DateDiff in days should be non-positive when created_at is after updated_at")
				suite.True(result.MonthsDiff <= 0, "DateDiff in months should be non-positive when created_at is after updated_at")
				suite.True(result.YearsDiff <= 0, "DateDiff in years should be non-positive when created_at is after updated_at")
			}

			suite.T().Logf("Created: %v | Updated: %v | Days: %.2f | Months: %.2f | Years: %.2f",
				result.CreatedAt, result.UpdatedAt, result.DaysDiff, result.MonthsDiff, result.YearsDiff)
		}
	})
}

// TestAge tests the Age function.
func (suite *EBDateTimeFunctionsTestSuite) TestAge() {
	suite.T().Logf("Testing Age function for %s", suite.ds.Kind)

	suite.Run("CalculateAgeInterval", func() {
		type AgeResult struct {
			ID        string `bun:"id"`
			CreatedAt string `bun:"created_at"`
			UpdatedAt string `bun:"updated_at"`
			Age       string `bun:"age"`
		}

		var results []AgeResult

		err := suite.selectUsers().
			Select("id", "created_at", "updated_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				// Calculate age between created_at and updated_at
				return eb.Age(eb.Column("created_at"), eb.Column("updated_at"))
			}, "age").
			// Only select records where updated_at >= created_at to avoid negative intervals
			// which SQLite cannot handle properly
			Where(func(cb orm.ConditionBuilder) {
				cb.GreaterThanOrEqualColumn("updated_at", "created_at")
			}).
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Age query should execute successfully")
		suite.Require().NotEmpty(results, "Age should return at least one result")

		for _, result := range results {
			suite.NotEmpty(result.Age, "Age interval should not be empty")

			// Verify the format contains "years", "mons", and "days"
			suite.Contains(result.Age, "years", "Age interval should contain 'years'")
			suite.Contains(result.Age, "mons", "Age interval should contain 'mons'")
			suite.Contains(result.Age, "days", "Age interval should contain 'days'")

			suite.T().Logf("ID: %s, CreatedAt: %s, UpdatedAt: %s, Age: %s",
				result.ID, result.CreatedAt, result.UpdatedAt, result.Age)
		}
	})

	suite.Run("CalculateAgeWithKnownDates", func() {
		type AgeTestResult struct {
			Age string `bun:"age"`
		}

		var result AgeTestResult

		// Test with known dates: from 1957-06-13 to 2001-04-10
		// Expected PostgreSQL result: "43 years 9 mons 27 days"
		err := suite.db.NewSelect().
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.Age(
					eb.Literal("1957-06-13"),
					eb.Literal("2001-04-10"),
				)
			}, "age").
			Scan(suite.ctx, &result)

		suite.Require().NoError(err, "Age query with known dates should execute successfully")
		suite.NotEmpty(result.Age, "Age interval should not be empty")

		// The result should contain years, mons, and days
		suite.Contains(result.Age, "years", "Age interval should contain 'years'")
		suite.Contains(result.Age, "mons", "Age interval should contain 'mons'")
		suite.Contains(result.Age, "days", "Age interval should contain 'days'")

		suite.T().Logf("Age from 1957-06-13 to 2001-04-10: %s", result.Age)

		// For PostgreSQL, we can verify the exact result
		if suite.ds.Kind == config.Postgres {
			suite.Contains(result.Age, "43 years", "Should contain approximately 43 years")
			suite.Contains(result.Age, "9 mons", "Should contain approximately 9 months")
		}
	})
}

// TestCombinedDateTimeFunctions tests using multiple date/time functions together.
func (suite *EBDateTimeFunctionsTestSuite) TestCombinedDateTimeFunctions() {
	suite.T().Logf("Testing combined date/time functions for %s", suite.ds.Kind)

	suite.Run("NestedDateTimeFunctions", func() {
		type CombinedResult struct {
			CreatedAt     time.Time `bun:"created_at"`
			Year          int64     `bun:"year"`
			MonthsFromNow float64   `bun:"months_from_now"`
			FormattedDate time.Time `bun:"formatted_date"`
		}

		var results []CombinedResult

		err := suite.selectPosts().
			Select("created_at").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.ExtractYear(eb.Column("created_at"))
			}, "year").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateDiff(
					eb.DateTrunc(orm.UnitMonth, eb.Column("created_at")),
					eb.DateTrunc(orm.UnitMonth, eb.Now()),
					orm.UnitMonth,
				)
			}, "months_from_now").
			SelectExpr(func(eb orm.ExprBuilder) any {
				return eb.DateTrunc(orm.UnitDay, eb.Column("created_at"))
			}, "formatted_date").
			OrderBy("created_at").
			Limit(5).
			Scan(suite.ctx, &results)

		suite.Require().NoError(err, "Combined date/time functions query should execute successfully")
		suite.Require().NotEmpty(results, "Combined date/time functions should return at least one result")

		for _, result := range results {
			suite.True(result.Year > 2000, "Extracted year should be in reasonable range (after 2000)")
			suite.True(result.MonthsFromNow >= 0, "Calculated months from now should be non-negative (created_at should not be in the future)")
			suite.True(!result.FormattedDate.After(result.CreatedAt), "Truncated date should not be after original timestamp")

			suite.T().Logf("CreatedAt: %v | Year: %d | MonthsFromNow: %.0f | FormattedDate: %v",
				result.CreatedAt, result.Year, result.MonthsFromNow, result.FormattedDate)
		}
	})
}
