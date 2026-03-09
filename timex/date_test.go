package timex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDateOf tests date of functionality.
func TestDateOf(t *testing.T) {
	now := MakeTime(2023, 12, 25, 14, 30, 45)
	date := DateOf(now)

	unwrapped := date.Unwrap()
	assert.Equal(t, 2023, unwrapped.Year(), "Year should be preserved")
	assert.Equal(t, int(time.December), int(unwrapped.Month()), "Month should be preserved")
	assert.Equal(t, 25, unwrapped.Day(), "Day should be preserved")
	assert.Equal(t, 0, unwrapped.Hour(), "Hour should be zeroed")
	assert.Equal(t, 0, unwrapped.Minute(), "Minute should be zeroed")
	assert.Equal(t, 0, unwrapped.Second(), "Second should be zeroed")
	assert.Equal(t, 0, unwrapped.Nanosecond(), "Nanosecond should be zeroed")
}

// TestNowDate tests now date functionality.
func TestNowDate(t *testing.T) {
	before := time.Now()
	date := NowDate()

	unwrapped := date.Unwrap()
	assert.Equal(t, before.Year(), unwrapped.Year(), "Year should match current")
	assert.Equal(t, int(before.Month()), int(unwrapped.Month()), "Month should match current")
	assert.Equal(t, before.Day(), unwrapped.Day(), "Day should match current")
	assert.Equal(t, 0, unwrapped.Hour(), "Hour should be zero")
	assert.Equal(t, 0, unwrapped.Minute(), "Minute should be zero")
	assert.Equal(t, 0, unwrapped.Second(), "Second should be zero")
}

// TestParseDate tests parse date functionality.
func TestParseDate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		pattern   []string
		shouldErr bool
		expected  time.Time
	}{
		{
			"ValidDate",
			"2023-12-25",
			nil,
			false,
			MakeTime(2023, 12, 25, 0, 0, 0),
		},
		{
			"ValidDateWithCustomPattern",
			"25/12/2023",
			[]string{"02/01/2006"},
			false,
			MakeTime(2023, 12, 25, 0, 0, 0),
		},
		{
			"InvalidDate",
			"invalid",
			nil,
			true,
			time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, err := ParseDate(tt.input, tt.pattern...)
			if tt.shouldErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should parse valid date")

				if !tt.expected.IsZero() {
					unwrapped := date.Unwrap()
					assert.Equal(t, tt.expected.Year(), unwrapped.Year(), "Year should match")
					assert.Equal(t, int(tt.expected.Month()), int(unwrapped.Month()), "Month should match")
					assert.Equal(t, tt.expected.Day(), unwrapped.Day(), "Day should match")
					assert.Equal(t, 0, unwrapped.Hour(), "Hour should be zero")
					assert.Equal(t, 0, unwrapped.Minute(), "Minute should be zero")
					assert.Equal(t, 0, unwrapped.Second(), "Second should be zero")
				}
			}
		})
	}
}

// TestDateString tests date string functionality.
func TestDateString(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	expected := "2023-12-25"
	assert.Equal(t, expected, date.String(), "String representation should match format")
}

// TestDateEqual tests date equal functionality.
func TestDateEqual(t *testing.T) {
	date1 := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	date2 := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	date3 := Date(MakeTime(2023, 12, 26, 0, 0, 0))

	assert.True(t, date1.Equal(date2), "Equal dates should be equal")
	assert.False(t, date1.Equal(date3), "Different dates should not be equal")
}

// TestDateBefore tests date before functionality.
func TestDateBefore(t *testing.T) {
	date1 := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	date2 := Date(MakeTime(2023, 12, 26, 0, 0, 0))

	assert.True(t, date1.Before(date2), "Earlier date should be before later")
	assert.False(t, date2.Before(date1), "Later date should not be before earlier")
}

// TestDateAfter tests date after functionality.
func TestDateAfter(t *testing.T) {
	date1 := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	date2 := Date(MakeTime(2023, 12, 26, 0, 0, 0))

	assert.False(t, date1.After(date2), "Earlier date should not be after later")
	assert.True(t, date2.After(date1), "Later date should be after earlier")
}

// TestDateBetween tests date between functionality.
func TestDateBetween(t *testing.T) {
	start := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	middle := Date(MakeTime(2023, 12, 26, 0, 0, 0))
	end := Date(MakeTime(2023, 12, 27, 0, 0, 0))

	assert.True(t, middle.Between(start, end), "Middle date should be between start and end")
	assert.False(t, start.Between(middle, end), "Start date should not be between middle and end")
}

// TestDateAddDays tests date add days functionality.
func TestDateAddDays(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	result := date.AddDays(5)

	expected := MakeTime(2023, 12, 30, 0, 0, 0)
	assert.True(t, expected.Equal(result.Unwrap()), "AddDays should add days correctly")
}

// TestDateAddMonths tests date add months functionality.
func TestDateAddMonths(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	result := date.AddMonths(2)

	expected := MakeTime(2024, 2, 25, 0, 0, 0)
	assert.True(t, expected.Equal(result.Unwrap()), "AddMonths should add months correctly")
}

// TestDateAddYears tests date add years functionality.
func TestDateAddYears(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	result := date.AddYears(1)

	expected := MakeTime(2024, 12, 25, 0, 0, 0)
	assert.True(t, expected.Equal(result.Unwrap()), "AddYears should add years correctly")
}

// TestDateSub tests date sub functionality.
func TestDateSub(t *testing.T) {
	d1 := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	d2 := Date(MakeTime(2023, 12, 20, 0, 0, 0))

	assert.Equal(t, 5*24*time.Hour, d1.Sub(d2), "Sub should return correct duration")
	assert.Equal(t, -5*24*time.Hour, d2.Sub(d1), "Sub should return negative for earlier minus later")
}

// TestDateSince tests date since functionality.
func TestDateSince(t *testing.T) {
	d := Date(MakeTime(2020, 1, 1, 0, 0, 0))

	since := d.Since()
	assert.True(t, since > 0, "Since should return positive duration for past date")
}

// TestDateUntil tests date until functionality.
func TestDateUntil(t *testing.T) {
	d := Date(MakeTime(2099, 1, 1, 0, 0, 0))

	until := d.Until()
	assert.True(t, until > 0, "Until should return positive duration for future date")
}

// TestDateComponents tests date components functionality.
func TestDateComponents(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))

	assert.Equal(t, 2023, date.Year(), "Year should match")
	assert.Equal(t, int(time.December), int(date.Month()), "Month should match")
	assert.Equal(t, 25, date.Day(), "Day should match")
	assert.Equal(t, int(time.Monday), int(date.Weekday()), "Weekday should match")
	assert.Equal(t, 359, date.YearDay(), "YearDay should match")
}

// TestDateIsZero tests date is zero functionality.
func TestDateIsZero(t *testing.T) {
	zeroDate := Date{}
	nonZeroDate := Date(MakeTime(2023, 12, 25, 0, 0, 0))

	assert.True(t, zeroDate.IsZero(), "Zero date should be zero")
	assert.False(t, nonZeroDate.IsZero(), "Non-zero date should not be zero")
}

// TestDateBeginOfMethods tests date begin of methods functionality.
func TestDateBeginOfMethods(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))

	t.Run("BeginOfDay", func(t *testing.T) {
		beginDay := date.BeginOfDay()
		assert.True(t, date.Unwrap().Equal(beginDay.Unwrap()), "BeginOfDay should return same date")
	})

	t.Run("BeginOfWeek", func(t *testing.T) {
		beginWeek := date.BeginOfWeek()
		expected := MakeTime(2023, 12, 24, 0, 0, 0)
		assert.True(t, expected.Equal(beginWeek.Unwrap()), "BeginOfWeek should return Sunday")
	})

	t.Run("BeginOfMonth", func(t *testing.T) {
		beginMonth := date.BeginOfMonth()
		expected := MakeTime(2023, 12, 1, 0, 0, 0)
		assert.True(t, expected.Equal(beginMonth.Unwrap()), "BeginOfMonth should return first day")
	})

	t.Run("BeginOfQuarter", func(t *testing.T) {
		beginQuarter := date.BeginOfQuarter()
		expected := MakeTime(2023, 10, 1, 0, 0, 0)
		assert.True(t, expected.Equal(beginQuarter.Unwrap()), "BeginOfQuarter should return Q4 start")
	})

	t.Run("BeginOfYear", func(t *testing.T) {
		beginYear := date.BeginOfYear()
		expected := MakeTime(2023, 1, 1, 0, 0, 0)
		assert.True(t, expected.Equal(beginYear.Unwrap()), "BeginOfYear should return Jan 1")
	})
}

// TestDateEndOfMethods tests date end of methods functionality.
func TestDateEndOfMethods(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))

	t.Run("EndOfDay", func(t *testing.T) {
		endDay := date.EndOfDay()
		assert.True(t, date.Unwrap().Equal(endDay.Unwrap()), "EndOfDay should return same date")
	})

	t.Run("EndOfWeek", func(t *testing.T) {
		endWeek := date.EndOfWeek()
		expected := MakeTime(2023, 12, 30, 0, 0, 0)
		assert.True(t, expected.Equal(endWeek.Unwrap()), "EndOfWeek should return Saturday")
	})

	t.Run("EndOfMonth", func(t *testing.T) {
		endMonth := date.EndOfMonth()
		expected := MakeTime(2023, 12, 31, 0, 0, 0)
		assert.True(t, expected.Equal(endMonth.Unwrap()), "EndOfMonth should return last day")
	})

	t.Run("EndOfQuarter", func(t *testing.T) {
		endQuarter := date.EndOfQuarter()
		expected := MakeTime(2023, 12, 31, 0, 0, 0)
		assert.True(t, expected.Equal(endQuarter.Unwrap()), "EndOfQuarter should return Q4 end")
	})

	t.Run("EndOfYear", func(t *testing.T) {
		endYear := date.EndOfYear()
		expected := MakeTime(2023, 12, 31, 0, 0, 0)
		assert.True(t, expected.Equal(endYear.Unwrap()), "EndOfYear should return Dec 31")
	})
}

// TestDateWeekdayMethods tests date weekday methods functionality.
func TestDateWeekdayMethods(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))

	t.Run("Monday", func(t *testing.T) {
		monday := date.Monday()
		expected := MakeTime(2023, 12, 25, 0, 0, 0)
		assert.True(t, expected.Equal(monday.Unwrap()), "Monday should return Dec 25")
	})

	t.Run("Tuesday", func(t *testing.T) {
		tuesday := date.Tuesday()
		expected := MakeTime(2023, 12, 26, 0, 0, 0)
		assert.True(t, expected.Equal(tuesday.Unwrap()), "Tuesday should return Dec 26")
	})

	t.Run("Sunday", func(t *testing.T) {
		sunday := date.Sunday()
		expected := MakeTime(2023, 12, 24, 0, 0, 0)
		assert.True(t, expected.Equal(sunday.Unwrap()), "Sunday should return Dec 24")
	})

	t.Run("Saturday", func(t *testing.T) {
		saturday := date.Saturday()
		expected := MakeTime(2023, 12, 30, 0, 0, 0)
		assert.True(t, expected.Equal(saturday.Unwrap()), "Saturday should return Dec 30")
	})
}

// TestDateMarshalJSON tests date marshal JSON functionality.
func TestDateMarshalJSON(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	data, err := date.MarshalJSON()
	assert.NoError(t, err, "MarshalJSON should succeed")

	expected := `"2023-12-25"`
	assert.Equal(t, expected, string(data), "JSON should match expected format")
}

// TestDateUnmarshalJSON tests date unmarshal JSON functionality.
func TestDateUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
	}{
		{"ValidDate", `"2023-12-25"`, false},
		{"NullValue", `null`, false},
		{"InvalidFormat", `"invalid"`, true},
		{"WrongLength", `"2023-12-25 14:30:45"`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var date Date

			err := date.UnmarshalJSON([]byte(tt.input))

			if tt.shouldErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should unmarshal valid input")
			}
		})
	}
}

// TestDateValue tests date value functionality.
func TestDateValue(t *testing.T) {
	date := Date(MakeTime(2023, 12, 25, 0, 0, 0))
	value, err := date.Value()
	assert.NoError(t, err, "Value should succeed")

	expected := "2023-12-25"
	str, ok := value.(string)
	assert.True(t, ok, "Value should be string")
	assert.Equal(t, expected, str, "Value should match expected format")
}

// TestDateScan tests date scan functionality.
func TestDateScan(t *testing.T) {
	tests := []struct {
		name   string
		src    any
		hasErr bool
	}{
		{"String", "2023-12-25", false},
		{"ByteSlice", []byte("2023-12-25"), false},
		{"TimeTime", MakeTime(2023, 12, 25, 14, 30, 45), false},
		{"NilPointer", (*string)(nil), false},
		{"InvalidString", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var date Date

			err := date.Scan(tt.src)

			if tt.hasErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should scan valid input")
			}
		})
	}
}

// TestDateJSONRoundTrip tests date JSON round trip functionality.
func TestDateJSONRoundTrip(t *testing.T) {
	original := Date(MakeTime(2023, 12, 25, 0, 0, 0))

	data, err := json.Marshal(original)
	assert.NoError(t, err, "Marshal should succeed")

	var result Date

	err = json.Unmarshal(data, &result)
	assert.NoError(t, err, "Unmarshal should succeed")

	assert.Equal(t, original.String(), result.String(), "Round trip should preserve value")
}
