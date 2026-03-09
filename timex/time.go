package timex

import (
	"database/sql/driver"
	"time"

	"github.com/gofiber/utils/v2"
)

// Time represents a time value (without date) with database and JSON support.
// It uses the standard time.TimeOnly format (15:04:05).
type Time time.Time

// Unwrap returns the underlying time.Time value.
func (t Time) Unwrap() time.Time {
	return time.Time(t)
}

// Format returns the string representation using the provided layout.
func (t Time) Format(layout string) string {
	return time.Time(t).Format(layout)
}

// Scan implements the sql.Scanner interface for database compatibility.
func (t *Time) Scan(src any) error {
	return scanTimeValue(src, func(s string) (any, error) {
		return ParseTime(s)
	}, func(t time.Time) any {
		return TimeOf(t)
	}, "time", t)
}

// Value implements the driver.Valuer interface for database compatibility.
func (t Time) Value() (driver.Value, error) {
	return t.String(), nil
}

// String returns the string representation using the standard TimeOnly layout.
func (t Time) String() string {
	return time.Time(t).Format(time.TimeOnly)
}

// MarshalJSON implements the json.Marshaler interface for JSON serialization.
func (t Time) MarshalJSON() ([]byte, error) {
	bs := make([]byte, 0, timePatternLength+2)
	bs = append(bs, jsonQuote)
	bs = time.Time(t).AppendFormat(bs, time.TimeOnly)
	bs = append(bs, jsonQuote)

	return bs, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSON deserialization.
func (t *Time) UnmarshalJSON(bs []byte) error {
	value := utils.UnsafeString(bs)
	if value == jsonNull {
		return nil
	}

	if err := validateJSONFormat(bs, timePatternLength); err != nil {
		return ErrInvalidTimeFormat
	}

	parsed, err := ParseTime(value[1 : timePatternLength+1])
	if err != nil {
		return err
	}

	*t = parsed

	return nil
}

// Equal compares two Time values for equality.
func (t Time) Equal(other Time) bool {
	return t.Unwrap().Equal(other.Unwrap())
}

// Before reports whether the time t is before other.
func (t Time) Before(other Time) bool {
	return t.Unwrap().Before(other.Unwrap())
}

// After reports whether the time t is after other.
func (t Time) After(other Time) bool {
	return t.Unwrap().After(other.Unwrap())
}

// AddHours adds the specified number of hours to the time.
func (t Time) AddHours(hours int) Time {
	return TimeOf(t.Unwrap().Add(time.Duration(hours) * time.Hour))
}

// AddMinutes adds the specified number of minutes to the time.
func (t Time) AddMinutes(minutes int) Time {
	return TimeOf(t.Unwrap().Add(time.Duration(minutes) * time.Minute))
}

// AddSeconds adds the specified number of seconds to the time.
func (t Time) AddSeconds(seconds int) Time {
	return TimeOf(t.Unwrap().Add(time.Duration(seconds) * time.Second))
}

// Hour returns the hour within the day specified by t, in the range [0, 23].
func (t Time) Hour() int {
	return t.Unwrap().Hour()
}

// Minute returns the minute offset within the hour specified by t, in the range [0, 59].
func (t Time) Minute() int {
	return t.Unwrap().Minute()
}

// Second returns the second offset within the minute specified by t, in the range [0, 59].
func (t Time) Second() int {
	return t.Unwrap().Second()
}

// Nanosecond returns the nanosecond offset within the second specified by t, in the range [0, 999999999].
func (t Time) Nanosecond() int {
	return t.Unwrap().Nanosecond()
}

// Add returns the time t+d.
func (t Time) Add(d time.Duration) Time {
	return TimeOf(t.Unwrap().Add(d))
}

// AddNanoseconds adds the specified number of nanoseconds to the time.
func (t Time) AddNanoseconds(nanoseconds int64) Time {
	return TimeOf(t.Unwrap().Add(time.Duration(nanoseconds) * time.Nanosecond))
}

// AddMicroseconds adds the specified number of microseconds to the time.
func (t Time) AddMicroseconds(microseconds int64) Time {
	return TimeOf(t.Unwrap().Add(time.Duration(microseconds) * time.Microsecond))
}

// AddMilliseconds adds the specified number of milliseconds to the time.
func (t Time) AddMilliseconds(milliseconds int64) Time {
	return TimeOf(t.Unwrap().Add(time.Duration(milliseconds) * time.Millisecond))
}

// Sub returns the duration t-other.
func (t Time) Sub(other Time) time.Duration {
	return t.Unwrap().Sub(other.Unwrap())
}

// IsZero reports whether t represents the zero time instant.
func (t Time) IsZero() bool {
	return t.Unwrap().IsZero()
}

// Between reports whether t is between start and end.
func (t Time) Between(start, end Time) bool {
	return t.After(start) && t.Before(end)
}

// BeginOfMinute returns the beginning of the minute for t.
func (t Time) BeginOfMinute() Time {
	unwrapped := t.Unwrap()

	return TimeOf(time.Date(1970, 1, 1, unwrapped.Hour(), unwrapped.Minute(), 0, 0, unwrapped.Location()))
}

// EndOfMinute returns the end of the minute for t.
func (t Time) EndOfMinute() Time {
	unwrapped := t.Unwrap()

	return TimeOf(time.Date(1970, 1, 1, unwrapped.Hour(), unwrapped.Minute(), 59, 999999999, unwrapped.Location()))
}

// BeginOfHour returns the beginning of the hour for t.
func (t Time) BeginOfHour() Time {
	unwrapped := t.Unwrap()

	return TimeOf(time.Date(1970, 1, 1, unwrapped.Hour(), 0, 0, 0, unwrapped.Location()))
}

// EndOfHour returns the end of the hour for t.
func (t Time) EndOfHour() Time {
	unwrapped := t.Unwrap()

	return TimeOf(time.Date(1970, 1, 1, unwrapped.Hour(), 59, 59, 999999999, unwrapped.Location()))
}

// MarshalText implements the encoding.TextMarshaler interface.
func (t Time) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (t *Time) UnmarshalText(text []byte) error {
	parsed, err := ParseTime(string(text))
	if err != nil {
		return err
	}

	*t = parsed

	return nil
}

// NowTime returns the current time in the local timezone.
func NowTime() Time {
	return TimeOf(time.Now().Local())
}

// TimeOf returns the time of the given time (date components are set to epoch).
func TimeOf(t time.Time) Time {
	return Time(time.Date(1970, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()))
}

// ParseTime parses a time string and returns a Time.
// First tries with the provided pattern, then falls back to cast.ToTime as a backup.
func ParseTime(value string, pattern ...string) (Time, error) {
	layout := timeLayout
	if len(pattern) > 0 {
		layout = pattern[0]
	}

	parsed, err := parseTimeWithFallback(value, layout)
	if err != nil {
		return Time{}, err
	}

	return TimeOf(parsed), nil
}
