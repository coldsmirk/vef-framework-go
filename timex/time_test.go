package timex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTimeOf tests time of functionality.
func TestTimeOf(t *testing.T) {
	now := MakeTime(2023, 12, 25, 14, 30, 45)
	timeOnly := TimeOf(now)

	unwrapped := timeOnly.Unwrap()
	assert.Equal(t, 1970, unwrapped.Year(), "Year should be epoch")
	assert.Equal(t, int(time.January), int(unwrapped.Month()), "Month should be epoch")
	assert.Equal(t, 1, unwrapped.Day(), "Day should be epoch")
	assert.Equal(t, 14, unwrapped.Hour(), "Hour should be preserved")
	assert.Equal(t, 30, unwrapped.Minute(), "Minute should be preserved")
	assert.Equal(t, 45, unwrapped.Second(), "Second should be preserved")
}

// TestNowTime tests now time functionality.
func TestNowTime(t *testing.T) {
	before := time.Now()
	timeOnly := NowTime()

	unwrapped := timeOnly.Unwrap()
	assert.Equal(t, 1970, unwrapped.Year(), "Year should be epoch")
	assert.Equal(t, int(time.January), int(unwrapped.Month()), "Month should be epoch")
	assert.Equal(t, 1, unwrapped.Day(), "Day should be epoch")

	assert.GreaterOrEqual(t, unwrapped.Hour(), before.Hour()-1, "Hour should be close to current")
	assert.LessOrEqual(t, unwrapped.Hour(), before.Hour()+1, "Hour should be close to current")
}

// TestParseTime tests parse time functionality.
func TestParseTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		pattern   []string
		shouldErr bool
		expected  time.Time
	}{
		{
			"ValidTime",
			"14:30:45",
			nil,
			false,
			time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local),
		},
		{
			"ValidTimeWithCustomPattern",
			"2:30:45 PM",
			[]string{"3:04:05 PM"},
			false,
			time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local),
		},
		{
			"InvalidTime",
			"invalid",
			nil,
			true,
			time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeOnly, err := ParseTime(tt.input, tt.pattern...)
			if tt.shouldErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should parse valid time")

				if !tt.expected.IsZero() {
					unwrapped := timeOnly.Unwrap()
					assert.Equal(t, 1970, unwrapped.Year(), "Year should be epoch")
					assert.Equal(t, int(time.January), int(unwrapped.Month()), "Month should be epoch")
					assert.Equal(t, 1, unwrapped.Day(), "Day should be epoch")
					assert.Equal(t, tt.expected.Hour(), unwrapped.Hour(), "Hour should match")
					assert.Equal(t, tt.expected.Minute(), unwrapped.Minute(), "Minute should match")
					assert.Equal(t, tt.expected.Second(), unwrapped.Second(), "Second should match")
				}
			}
		})
	}
}

// TestTimeString tests time string functionality.
func TestTimeString(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	expected := "14:30:45"
	assert.Equal(t, expected, timeOnly.String(), "String representation should match format")
}

// TestTimeEqual tests time equal functionality.
func TestTimeEqual(t *testing.T) {
	time1 := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	time2 := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	time3 := Time(time.Date(1970, 1, 1, 14, 30, 46, 0, time.Local))

	assert.True(t, time1.Equal(time2), "Equal times should be equal")
	assert.False(t, time1.Equal(time3), "Different times should not be equal")
}

// TestTimeBefore tests time before functionality.
func TestTimeBefore(t *testing.T) {
	time1 := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	time2 := Time(time.Date(1970, 1, 1, 14, 30, 46, 0, time.Local))

	assert.True(t, time1.Before(time2), "Earlier time should be before later")
	assert.False(t, time2.Before(time1), "Later time should not be before earlier")
}

// TestTimeAfter tests time after functionality.
func TestTimeAfter(t *testing.T) {
	time1 := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	time2 := Time(time.Date(1970, 1, 1, 14, 30, 46, 0, time.Local))

	assert.False(t, time1.After(time2), "Earlier time should not be after later")
	assert.True(t, time2.After(time1), "Later time should be after earlier")
}

// TestTimeBetween tests time between functionality.
func TestTimeBetween(t *testing.T) {
	start := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	middle := Time(time.Date(1970, 1, 1, 14, 30, 46, 0, time.Local))
	end := Time(time.Date(1970, 1, 1, 14, 30, 47, 0, time.Local))

	assert.True(t, middle.Between(start, end), "Middle time should be between start and end")
	assert.False(t, start.Between(middle, end), "Start time should not be between middle and end")
}

// TestTimeAdd tests time add functionality.
func TestTimeAdd(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	duration := 2 * time.Hour
	result := timeOnly.Add(duration)

	expected := time.Date(1970, 1, 1, 16, 30, 45, 0, time.Local)
	assert.True(t, expected.Equal(result.Unwrap()), "Add should add duration correctly")
}

// TestTimeAddHours tests time add hours functionality.
func TestTimeAddHours(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	result := timeOnly.AddHours(3)

	expected := time.Date(1970, 1, 1, 17, 30, 45, 0, time.Local)
	assert.True(t, expected.Equal(result.Unwrap()), "AddHours should add hours correctly")
}

// TestTimeAddMinutes tests time add minutes functionality.
func TestTimeAddMinutes(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	result := timeOnly.AddMinutes(15)

	expected := time.Date(1970, 1, 1, 14, 45, 45, 0, time.Local)
	assert.True(t, expected.Equal(result.Unwrap()), "AddMinutes should add minutes correctly")
}

// TestTimeAddSeconds tests time add seconds functionality.
func TestTimeAddSeconds(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	result := timeOnly.AddSeconds(30)

	expected := time.Date(1970, 1, 1, 14, 31, 15, 0, time.Local)
	assert.True(t, expected.Equal(result.Unwrap()), "AddSeconds should add seconds correctly")
}

// TestTimeAddNanoseconds tests time add nanoseconds functionality.
func TestTimeAddNanoseconds(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	result := timeOnly.AddNanoseconds(500000000)

	expected := time.Date(1970, 1, 1, 14, 30, 45, 500000000, time.Local)
	assert.True(t, expected.Equal(result.Unwrap()), "AddNanoseconds should add nanoseconds correctly")
}

// TestTimeAddMicroseconds tests time add microseconds functionality.
func TestTimeAddMicroseconds(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	result := timeOnly.AddMicroseconds(500000)

	expected := time.Date(1970, 1, 1, 14, 30, 45, 500000000, time.Local)
	assert.True(t, expected.Equal(result.Unwrap()), "AddMicroseconds should add microseconds correctly")
}

// TestTimeAddMilliseconds tests time add milliseconds functionality.
func TestTimeAddMilliseconds(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	result := timeOnly.AddMilliseconds(500)

	expected := time.Date(1970, 1, 1, 14, 30, 45, 500000000, time.Local)
	assert.True(t, expected.Equal(result.Unwrap()), "AddMilliseconds should add milliseconds correctly")
}

// TestTimeSub tests time sub functionality.
func TestTimeSub(t *testing.T) {
	t1 := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	t2 := Time(time.Date(1970, 1, 1, 12, 30, 45, 0, time.Local))

	assert.Equal(t, 2*time.Hour, t1.Sub(t2), "Sub should return correct duration")
	assert.Equal(t, -2*time.Hour, t2.Sub(t1), "Sub should return negative for earlier minus later")
}

// TestTimeComponents tests time components functionality.
func TestTimeComponents(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 123456789, time.Local))

	assert.Equal(t, 14, timeOnly.Hour(), "Hour should match")
	assert.Equal(t, 30, timeOnly.Minute(), "Minute should match")
	assert.Equal(t, 45, timeOnly.Second(), "Second should match")
	assert.Equal(t, 123456789, timeOnly.Nanosecond(), "Nanosecond should match")
}

// TestTimeIsZero tests time is zero functionality.
func TestTimeIsZero(t *testing.T) {
	zeroTime := Time{}
	nonZeroTime := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))

	assert.True(t, zeroTime.IsZero(), "Zero time should be zero")
	assert.False(t, nonZeroTime.IsZero(), "Non-zero time should not be zero")
}

// TestTimeBeginOfMethods tests time begin of methods functionality.
func TestTimeBeginOfMethods(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 123456789, time.Local))

	t.Run("BeginOfMinute", func(t *testing.T) {
		beginMinute := timeOnly.BeginOfMinute()
		expected := time.Date(1970, 1, 1, 14, 30, 0, 0, time.Local)
		assert.True(t, expected.Equal(beginMinute.Unwrap()), "BeginOfMinute should zero seconds and nanoseconds")
	})

	t.Run("BeginOfHour", func(t *testing.T) {
		beginHour := timeOnly.BeginOfHour()
		expected := time.Date(1970, 1, 1, 14, 0, 0, 0, time.Local)
		assert.True(t, expected.Equal(beginHour.Unwrap()), "BeginOfHour should zero minutes, seconds, and nanoseconds")
	})
}

// TestTimeEndOfMethods tests time end of methods functionality.
func TestTimeEndOfMethods(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 123456789, time.Local))

	t.Run("EndOfMinute", func(t *testing.T) {
		endMinute := timeOnly.EndOfMinute()
		expected := time.Date(1970, 1, 1, 14, 30, 59, 999999999, time.Local)
		assert.True(t, expected.Equal(endMinute.Unwrap()), "EndOfMinute should set to last moment")
	})

	t.Run("EndOfHour", func(t *testing.T) {
		endHour := timeOnly.EndOfHour()
		expected := time.Date(1970, 1, 1, 14, 59, 59, 999999999, time.Local)
		assert.True(t, expected.Equal(endHour.Unwrap()), "EndOfHour should set to last moment")
	})
}

// TestTimeMarshalJSON tests time marshal JSON functionality.
func TestTimeMarshalJSON(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	data, err := timeOnly.MarshalJSON()
	assert.NoError(t, err, "MarshalJSON should succeed")

	expected := `"14:30:45"`
	assert.Equal(t, expected, string(data), "JSON should match expected format")
}

// TestTimeUnmarshalJSON tests time unmarshal JSON functionality.
func TestTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
	}{
		{"ValidTime", `"14:30:45"`, false},
		{"NullValue", `null`, false},
		{"InvalidFormat", `"invalid"`, true},
		{"WrongLength", `"14:30"`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var timeOnly Time

			err := timeOnly.UnmarshalJSON([]byte(tt.input))

			if tt.shouldErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should unmarshal valid input")
			}
		})
	}
}

// TestTimeValue tests time value functionality.
func TestTimeValue(t *testing.T) {
	timeOnly := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))
	value, err := timeOnly.Value()
	assert.NoError(t, err, "Value should succeed")

	expected := "14:30:45"
	str, ok := value.(string)
	assert.True(t, ok, "Value should be string")
	assert.Equal(t, expected, str, "Value should match expected format")
}

// TestTimeScan tests time scan functionality.
func TestTimeScan(t *testing.T) {
	tests := []struct {
		name   string
		src    any
		hasErr bool
	}{
		{"String", "14:30:45", false},
		{"ByteSlice", []byte("14:30:45"), false},
		{"TimeTime", MakeTime(2023, 12, 25, 14, 30, 45), false},
		{"NilPointer", (*string)(nil), false},
		{"InvalidString", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var timeOnly Time

			err := timeOnly.Scan(tt.src)

			if tt.hasErr {
				assert.Error(t, err, "Should return error for invalid input")
			} else {
				assert.NoError(t, err, "Should scan valid input")
			}
		})
	}
}

// TestTimeJSONRoundTrip tests time JSON round trip functionality.
func TestTimeJSONRoundTrip(t *testing.T) {
	original := Time(time.Date(1970, 1, 1, 14, 30, 45, 0, time.Local))

	data, err := json.Marshal(original)
	assert.NoError(t, err, "Marshal should succeed")

	var result Time

	err = json.Unmarshal(data, &result)
	assert.NoError(t, err, "Unmarshal should succeed")

	assert.Equal(t, original.String(), result.String(), "Round trip should preserve value")
}
