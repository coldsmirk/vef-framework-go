package timex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDateTimeOf(t *testing.T) {
	now := MakeTime(2023, 12, 25, 14, 30, 45)
	dt := Of(now)

	assert.True(t, now.Equal(dt.Unwrap()), "Of should preserve the original time")
}

func TestNow(t *testing.T) {
	before := time.Now()
	dt := Now()
	after := time.Now()

	unwrapped := dt.Unwrap()
	assert.False(t, unwrapped.Before(before) || unwrapped.After(after), "Now should return current time")
}

func TestFromUnix(t *testing.T) {
	timestamp := int64(1703514645)
	dt := FromUnix(timestamp, 0)

	testLocation := time.FixedZone("UTC+8", 8*60*60)
	expected := time.Date(2023, 12, 25, 22, 30, 45, 0, testLocation)
	assert.True(t, expected.Equal(dt.Unwrap()), "FromUnix should create correct datetime")
}

func TestFromUnixMilli(t *testing.T) {
	timestamp := int64(1703514645000)
	dt := FromUnixMilli(timestamp)

	testLocation := time.FixedZone("UTC+8", 8*60*60)
	expected := time.Date(2023, 12, 25, 22, 30, 45, 0, testLocation)
	assert.True(t, expected.Equal(dt.Unwrap()), "FromUnixMilli should create correct datetime")
}

func TestFromUnixMicro(t *testing.T) {
	timestamp := int64(1703514645000000)
	dt := FromUnixMicro(timestamp)

	testLocation := time.FixedZone("UTC+8", 8*60*60)
	expected := time.Date(2023, 12, 25, 22, 30, 45, 0, testLocation)
	assert.True(t, expected.Equal(dt.Unwrap()), "FromUnixMicro should create correct datetime")
}

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		pattern   []string
		shouldErr bool
		expected  time.Time
	}{
		{
			"ValidDateTime",
			"2023-12-25 14:30:45",
			nil,
			false,
			MakeTime(2023, 12, 25, 14, 30, 45),
		},
		{
			"ValidDateTimeWithCustomPattern",
			"25/12/2023 14:30:45",
			[]string{"02/01/2006 15:04:05"},
			false,
			MakeTime(2023, 12, 25, 14, 30, 45),
		},
		{
			"InvalidDateTime",
			"invalid",
			nil,
			true,
			time.Time{},
		},
		{
			"ISOFormat",
			"2023-12-25T14:30:45Z",
			nil,
			false,
			MakeTimeUTC(2023, 12, 25, 14, 30, 45),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt, err := Parse(tt.input, tt.pattern...)
			if tt.shouldErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should parse valid datetime")

				if !tt.expected.IsZero() {
					assert.True(t, tt.expected.Equal(dt.Unwrap()), "Parsed datetime should match expected")
				}
			}
		})
	}
}

func TestDateTimeString(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	expected := "2023-12-25 14:30:45"
	assert.Equal(t, expected, dt.String(), "String representation should match format")
}

func TestDateTimeEqual(t *testing.T) {
	dt1 := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	dt2 := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	dt3 := DateTime(MakeTime(2023, 12, 25, 14, 30, 46))

	assert.True(t, dt1.Equal(dt2), "Equal datetimes should be equal")
	assert.False(t, dt1.Equal(dt3), "Different datetimes should not be equal")
}

func TestDateTimeBefore(t *testing.T) {
	dt1 := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	dt2 := DateTime(MakeTime(2023, 12, 25, 14, 30, 46))

	assert.True(t, dt1.Before(dt2), "Earlier datetime should be before later")
	assert.False(t, dt2.Before(dt1), "Later datetime should not be before earlier")
}

func TestDateTimeAfter(t *testing.T) {
	dt1 := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	dt2 := DateTime(MakeTime(2023, 12, 25, 14, 30, 46))

	assert.False(t, dt1.After(dt2), "Earlier datetime should not be after later")
	assert.True(t, dt2.After(dt1), "Later datetime should be after earlier")
}

func TestDateTimeBetween(t *testing.T) {
	start := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	middle := DateTime(MakeTime(2023, 12, 25, 14, 30, 46))
	end := DateTime(MakeTime(2023, 12, 25, 14, 30, 47))

	assert.True(t, middle.Between(start, end), "Middle datetime should be between start and end")
	assert.False(t, start.Between(middle, end), "Start datetime should not be between middle and end")
	assert.False(t, start.Between(start, end), "Between is exclusive of the start endpoint")
	assert.False(t, end.Between(start, end), "Between is exclusive of the end endpoint")
}

func TestDateTimeAdd(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	duration := 2 * time.Hour
	result := dt.Add(duration)

	expected := MakeTime(2023, 12, 25, 16, 30, 45)
	assert.True(t, expected.Equal(result.Unwrap()), "Add should add duration correctly")
}

func TestDateTimeAddDate(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	result := dt.AddDate(1, 2, 3)

	expected := MakeTime(2025, 2, 28, 14, 30, 45)
	assert.True(t, expected.Equal(result.Unwrap()), "AddDate should add years, months, days correctly")
}

func TestDateTimeAddDays(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	result := dt.AddDays(5)

	expected := MakeTime(2023, 12, 30, 14, 30, 45)
	assert.True(t, expected.Equal(result.Unwrap()), "AddDays should add days correctly")
}

func TestDateTimeAddMonths(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	result := dt.AddMonths(2)

	expected := MakeTime(2024, 2, 25, 14, 30, 45)
	assert.True(t, expected.Equal(result.Unwrap()), "AddMonths should add months correctly")
}

func TestDateTimeAddYears(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	result := dt.AddYears(1)

	expected := MakeTime(2024, 12, 25, 14, 30, 45)
	assert.True(t, expected.Equal(result.Unwrap()), "AddYears should add years correctly")
}

func TestDateTimeAddHours(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	result := dt.AddHours(3)

	expected := MakeTime(2023, 12, 25, 17, 30, 45)
	assert.True(t, expected.Equal(result.Unwrap()), "AddHours should add hours correctly")
}

func TestDateTimeAddMinutes(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	result := dt.AddMinutes(15)

	expected := MakeTime(2023, 12, 25, 14, 45, 45)
	assert.True(t, expected.Equal(result.Unwrap()), "AddMinutes should add minutes correctly")
}

func TestDateTimeAddSeconds(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	result := dt.AddSeconds(30)

	expected := MakeTime(2023, 12, 25, 14, 31, 15)
	assert.True(t, expected.Equal(result.Unwrap()), "AddSeconds should add seconds correctly")
}

func TestDateTimeSub(t *testing.T) {
	dt1 := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	dt2 := DateTime(MakeTime(2023, 12, 25, 12, 30, 45))

	assert.Equal(t, 2*time.Hour, dt1.Sub(dt2), "Sub should return correct duration")
	assert.Equal(t, -2*time.Hour, dt2.Sub(dt1), "Sub should return negative for earlier minus later")
}

func TestDateTimeSince(t *testing.T) {
	dt := DateTime(MakeTime(2020, 1, 1, 0, 0, 0))

	since := dt.Since()
	assert.True(t, since > 0, "Since should return positive duration for past datetime")
}

func TestDateTimeUntil(t *testing.T) {
	dt := DateTime(MakeTime(2099, 1, 1, 0, 0, 0))

	until := dt.Until()
	assert.True(t, until > 0, "Until should return positive duration for future datetime")
}

func TestDateTimeComponents(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))

	assert.Equal(t, 2023, dt.Year(), "Year should match")
	assert.Equal(t, int(time.December), int(dt.Month()), "Month should match")
	assert.Equal(t, 25, dt.Day(), "Day should match")
	assert.Equal(t, 14, dt.Hour(), "Hour should match")
	assert.Equal(t, 30, dt.Minute(), "Minute should match")
	assert.Equal(t, 45, dt.Second(), "Second should match")
	assert.Equal(t, 0, dt.Nanosecond(), "Nanosecond should match")
	assert.Equal(t, int(time.Monday), int(dt.Weekday()), "Weekday should match")
	assert.Equal(t, 359, dt.YearDay(), "YearDay should match")
}

func TestDateTimeUnixMethods(t *testing.T) {
	testLocation := time.FixedZone("UTC+8", 8*60*60)
	dt := DateTime(time.Date(2023, 12, 25, 14, 30, 45, 0, testLocation))

	expectedUnix := int64(1703485845)
	assert.Equal(t, int(expectedUnix), int(dt.Unix()), "Unix timestamp should match")
	assert.Equal(t, int(expectedUnix*1000), int(dt.UnixMilli()), "Unix milliseconds should match")
	assert.Equal(t, int(expectedUnix*1000000), int(dt.UnixMicro()), "Unix microseconds should match")
	assert.Equal(t, int(expectedUnix*1000000000), int(dt.UnixNano()), "Unix nanoseconds should match")
}

func TestDateTimeIsZero(t *testing.T) {
	zeroTime := DateTime{}
	nonZeroTime := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))

	assert.True(t, zeroTime.IsZero(), "Zero datetime should be zero")
	assert.False(t, nonZeroTime.IsZero(), "Non-zero datetime should not be zero")
}

func TestDateTimeBeginOfMethods(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))

	t.Run("BeginOfMinute", func(t *testing.T) {
		beginMinute := dt.BeginOfMinute()
		expected := MakeTime(2023, 12, 25, 14, 30, 0)
		assert.True(t, expected.Equal(beginMinute.Unwrap()), "BeginOfMinute should zero seconds")
	})

	t.Run("BeginOfHour", func(t *testing.T) {
		beginHour := dt.BeginOfHour()
		expected := MakeTime(2023, 12, 25, 14, 0, 0)
		assert.True(t, expected.Equal(beginHour.Unwrap()), "BeginOfHour should zero minutes and seconds")
	})

	t.Run("BeginOfDay", func(t *testing.T) {
		beginDay := dt.BeginOfDay()
		expected := MakeTime(2023, 12, 25, 0, 0, 0)
		assert.True(t, expected.Equal(beginDay.Unwrap()), "BeginOfDay should zero time components")
	})

	t.Run("BeginOfWeek", func(t *testing.T) {
		beginWeek := dt.BeginOfWeek()
		expected := MakeTime(2023, 12, 24, 0, 0, 0)
		assert.True(t, expected.Equal(beginWeek.Unwrap()), "BeginOfWeek should return Sunday")
	})

	t.Run("BeginOfMonth", func(t *testing.T) {
		beginMonth := dt.BeginOfMonth()
		expected := MakeTime(2023, 12, 1, 0, 0, 0)
		assert.True(t, expected.Equal(beginMonth.Unwrap()), "BeginOfMonth should return first day")
	})

	t.Run("BeginOfQuarter", func(t *testing.T) {
		beginQuarter := dt.BeginOfQuarter()
		expected := MakeTime(2023, 10, 1, 0, 0, 0)
		assert.True(t, expected.Equal(beginQuarter.Unwrap()), "BeginOfQuarter should return Q4 start")
	})

	t.Run("BeginOfYear", func(t *testing.T) {
		beginYear := dt.BeginOfYear()
		expected := MakeTime(2023, 1, 1, 0, 0, 0)
		assert.True(t, expected.Equal(beginYear.Unwrap()), "BeginOfYear should return Jan 1")
	})
}

func TestDateTimeEndOfMethods(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))

	t.Run("EndOfMinute", func(t *testing.T) {
		endMinute := dt.EndOfMinute()
		expected := time.Date(2023, 12, 25, 14, 30, 59, 999999999, time.Local)
		assert.True(t, expected.Equal(endMinute.Unwrap()), "EndOfMinute should set to last moment")
	})

	t.Run("EndOfHour", func(t *testing.T) {
		endHour := dt.EndOfHour()
		expected := time.Date(2023, 12, 25, 14, 59, 59, 999999999, time.Local)
		assert.True(t, expected.Equal(endHour.Unwrap()), "EndOfHour should set to last moment")
	})

	t.Run("EndOfDay", func(t *testing.T) {
		endDay := dt.EndOfDay()
		expected := time.Date(2023, 12, 25, 23, 59, 59, 999999999, time.Local)
		assert.True(t, expected.Equal(endDay.Unwrap()), "EndOfDay should set to last moment")
	})

	t.Run("EndOfWeek", func(t *testing.T) {
		endWeek := dt.EndOfWeek()
		expected := time.Date(2023, 12, 30, 23, 59, 59, 999999999, time.Local)
		assert.True(t, expected.Equal(endWeek.Unwrap()), "EndOfWeek should return Saturday")
	})

	t.Run("EndOfYear", func(t *testing.T) {
		endYear := dt.EndOfYear()
		expected := time.Date(2023, 12, 31, 23, 59, 59, 999999999, time.Local)
		assert.True(t, expected.Equal(endYear.Unwrap()), "EndOfYear should return Dec 31")
	})
}

func TestDateTimeWeekdayMethods(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))

	t.Run("Monday", func(t *testing.T) {
		monday := dt.Monday()
		expected := MakeTime(2023, 12, 25, 0, 0, 0)
		assert.True(t, expected.Equal(monday.Unwrap()), "Monday should return Dec 25")
	})

	t.Run("Tuesday", func(t *testing.T) {
		tuesday := dt.Tuesday()
		expected := MakeTime(2023, 12, 26, 0, 0, 0)
		assert.True(t, expected.Equal(tuesday.Unwrap()), "Tuesday should return Dec 26")
	})

	t.Run("Sunday", func(t *testing.T) {
		sunday := dt.Sunday()
		expected := MakeTime(2023, 12, 24, 0, 0, 0)
		assert.True(t, expected.Equal(sunday.Unwrap()), "Sunday should return Dec 24")
	})
}

func TestDateTimeMarshalJSON(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	data, err := dt.MarshalJSON()
	assert.NoError(t, err, "MarshalJSON should succeed")

	expected := `"2023-12-25 14:30:45"`
	assert.Equal(t, expected, string(data), "JSON should match expected format")
}

func TestDateTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
	}{
		{"ValidDateTime", `"2023-12-25 14:30:45"`, false},
		{"NullValue", `null`, false},
		{"InvalidFormat", `"invalid"`, true},
		{"WrongLength", `"2023-12-25"`, true},
		{"MissingQuotes", `2023-12-25 14:30:45`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dt DateTime

			err := dt.UnmarshalJSON([]byte(tt.input))

			if tt.shouldErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should unmarshal valid input")
			}
		})
	}
}

func TestDateTimeValue(t *testing.T) {
	dt := DateTime(MakeTime(2023, 12, 25, 14, 30, 45))
	value, err := dt.Value()
	assert.NoError(t, err, "Value should succeed")

	expected := "2023-12-25 14:30:45"
	str, ok := value.(string)
	assert.True(t, ok, "Value should be string")
	assert.Equal(t, expected, str, "Value should match expected format")
}

func TestDateTimeScan(t *testing.T) {
	tests := []struct {
		name   string
		src    any
		hasErr bool
	}{
		{"String", "2023-12-25 14:30:45", false},
		{"ByteSlice", []byte("2023-12-25 14:30:45"), false},
		{"StringPointer", new("2023-12-25 14:30:45"), false},
		{"ByteSlicePointer", new([]byte("2023-12-25 14:30:45")), false},
		{"TimeTime", MakeTime(2023, 12, 25, 14, 30, 45), false},
		{"TimePointer", new(MakeTime(2023, 12, 25, 14, 30, 45)), false},
		{"NilStringPointer", (*string)(nil), false},
		{"NilByteSlicePointer", (*[]byte)(nil), false},
		{"NilTimePointer", (*time.Time)(nil), false},
		{"InvalidString", "invalid", true},
		{"UnsupportedType", complex(1, 2), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dt DateTime

			err := dt.Scan(tt.src)

			if tt.hasErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should scan valid input")
			}
		})
	}
}

func TestDateTimeJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		original DateTime
	}{
		{"NormalDateTime", DateTime(MakeTime(2023, 12, 25, 14, 30, 45))},
		{"EpochDateTime", DateTime(MakeTime(1970, 1, 1, 0, 0, 0))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.original)
			assert.NoError(t, err, "Marshal should succeed")

			var result DateTime

			err = json.Unmarshal(data, &result)
			assert.NoError(t, err, "Unmarshal should succeed")

			assert.Equal(t, tt.original.String(), result.String(), "Round trip should preserve value")
		})
	}
}
