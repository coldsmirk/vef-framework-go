package timex

import (
	"database/sql/driver"
	"time"

	"github.com/gofiber/utils/v2"
)

// DateTime represents a date and time value with database and JSON support.
// It uses the standard time.DateTime format (2006-01-02 15:04:05).
type DateTime time.Time

// Unwrap returns the underlying time.Time value.
func (dt DateTime) Unwrap() time.Time {
	return time.Time(dt)
}

// Format returns the string representation using the provided layout.
func (dt DateTime) Format(layout string) string {
	return time.Time(dt).Format(layout)
}

// Scan implements the sql.Scanner interface for database compatibility.
func (dt *DateTime) Scan(src any) error {
	return scanTimeValue(src, func(s string) (any, error) {
		return Parse(s)
	}, func(t time.Time) any {
		return DateTime(t)
	}, "datetime", dt)
}

// Value implements the driver.Valuer interface for database compatibility.
func (dt DateTime) Value() (driver.Value, error) {
	return dt.String(), nil
}

// String returns the string representation using the standard DateTime layout.
func (dt DateTime) String() string {
	return time.Time(dt).Format(time.DateTime)
}

// MarshalJSON implements the json.Marshaler interface for JSON serialization.
func (dt DateTime) MarshalJSON() ([]byte, error) {
	bs := make([]byte, 0, dateTimePatternLength+2)
	bs = append(bs, jsonQuote)
	bs = time.Time(dt).AppendFormat(bs, time.DateTime)
	bs = append(bs, jsonQuote)

	return bs, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSON deserialization.
func (dt *DateTime) UnmarshalJSON(bs []byte) error {
	value := utils.UnsafeString(bs)
	if value == jsonNull {
		return nil
	}

	if err := validateJSONFormat(bs, dateTimePatternLength); err != nil {
		return ErrInvalidDateTimeFormat
	}

	parsed, err := Parse(value[1 : dateTimePatternLength+1])
	if err != nil {
		return err
	}

	*dt = parsed

	return nil
}

// Equal compares two DateTime values for equality.
func (dt DateTime) Equal(other DateTime) bool {
	return dt.Unwrap().Equal(other.Unwrap())
}

// Before reports whether the datetime dt is before other.
func (dt DateTime) Before(other DateTime) bool {
	return dt.Unwrap().Before(other.Unwrap())
}

// After reports whether the datetime dt is after other.
func (dt DateTime) After(other DateTime) bool {
	return dt.Unwrap().After(other.Unwrap())
}

// Add returns the datetime dt+d.
func (dt DateTime) Add(d time.Duration) DateTime {
	return DateTime(dt.Unwrap().Add(d))
}

// AddDate returns the datetime corresponding to adding the given number of years, months, and days to dt.
func (dt DateTime) AddDate(years, months, days int) DateTime {
	return DateTime(dt.Unwrap().AddDate(years, months, days))
}

// AddDays adds the specified number of days to the datetime.
func (dt DateTime) AddDays(days int) DateTime {
	return DateTime(dt.Unwrap().AddDate(0, 0, days))
}

// AddMonths adds the specified number of months to the datetime.
func (dt DateTime) AddMonths(months int) DateTime {
	return DateTime(dt.Unwrap().AddDate(0, months, 0))
}

// AddYears adds the specified number of years to the datetime.
func (dt DateTime) AddYears(years int) DateTime {
	return DateTime(dt.Unwrap().AddDate(years, 0, 0))
}

// AddHours adds the specified number of hours to the datetime.
func (dt DateTime) AddHours(hours int) DateTime {
	return DateTime(dt.Unwrap().Add(time.Duration(hours) * time.Hour))
}

// AddMinutes adds the specified number of minutes to the datetime.
func (dt DateTime) AddMinutes(minutes int) DateTime {
	return DateTime(dt.Unwrap().Add(time.Duration(minutes) * time.Minute))
}

// AddSeconds adds the specified number of seconds to the datetime.
func (dt DateTime) AddSeconds(seconds int) DateTime {
	return DateTime(dt.Unwrap().Add(time.Duration(seconds) * time.Second))
}

// Year returns the year in which dt occurs.
func (dt DateTime) Year() int {
	return dt.Unwrap().Year()
}

// Month returns the month of the year specified by dt.
func (dt DateTime) Month() time.Month {
	return dt.Unwrap().Month()
}

// Day returns the day of the month specified by dt.
func (dt DateTime) Day() int {
	return dt.Unwrap().Day()
}

// Hour returns the hour within the day specified by dt, in the range [0, 23].
func (dt DateTime) Hour() int {
	return dt.Unwrap().Hour()
}

// Minute returns the minute offset within the hour specified by dt, in the range [0, 59].
func (dt DateTime) Minute() int {
	return dt.Unwrap().Minute()
}

// Second returns the second offset within the minute specified by dt, in the range [0, 59].
func (dt DateTime) Second() int {
	return dt.Unwrap().Second()
}

// Nanosecond returns the nanosecond offset within the second specified by dt, in the range [0, 999999999].
func (dt DateTime) Nanosecond() int {
	return dt.Unwrap().Nanosecond()
}

// Weekday returns the day of the week specified by dt.
func (dt DateTime) Weekday() time.Weekday {
	return dt.Unwrap().Weekday()
}

// YearDay returns the day of the year specified by dt, in the range [1,365] for non-leap years,
// and [1,366] in leap years.
func (dt DateTime) YearDay() int {
	return dt.Unwrap().YearDay()
}

// Location returns the time zone information associated with dt.
func (dt DateTime) Location() *time.Location {
	return dt.Unwrap().Location()
}

// Unix returns dt as a Unix time, the number of seconds elapsed since January 1, 1970 UTC.
func (dt DateTime) Unix() int64 {
	return dt.Unwrap().Unix()
}

// UnixMilli returns dt as a Unix time, the number of milliseconds elapsed since January 1, 1970 UTC.
func (dt DateTime) UnixMilli() int64 {
	return dt.Unwrap().UnixMilli()
}

// UnixMicro returns dt as a Unix time, the number of microseconds elapsed since January 1, 1970 UTC.
func (dt DateTime) UnixMicro() int64 {
	return dt.Unwrap().UnixMicro()
}

// UnixNano returns dt as a Unix time, the number of nanoseconds elapsed since January 1, 1970 UTC.
func (dt DateTime) UnixNano() int64 {
	return dt.Unwrap().UnixNano()
}

// Sub returns the duration dt-other.
func (dt DateTime) Sub(other DateTime) time.Duration {
	return dt.Unwrap().Sub(other.Unwrap())
}

// Since returns the time elapsed since dt (equivalent to time.Since).
func (dt DateTime) Since() time.Duration {
	return time.Since(dt.Unwrap())
}

// Until returns the duration until dt (equivalent to time.Until).
func (dt DateTime) Until() time.Duration {
	return time.Until(dt.Unwrap())
}

// IsZero reports whether dt represents the zero time instant, January 1, year 1, 00:00:00 UTC.
func (dt DateTime) IsZero() bool {
	return dt.Unwrap().IsZero()
}

// Between reports whether dt is between start and end.
func (dt DateTime) Between(start, end DateTime) bool {
	return dt.After(start) && dt.Before(end)
}

// BeginOfMinute returns the beginning of the minute for dt.
func (dt DateTime) BeginOfMinute() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location()))
}

// EndOfMinute returns the end of the minute for dt.
func (dt DateTime) EndOfMinute() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 59, 999999999, t.Location()))
}

// BeginOfHour returns the beginning of the hour for dt.
func (dt DateTime) BeginOfHour() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()))
}

// EndOfHour returns the end of the hour for dt.
func (dt DateTime) EndOfHour() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 59, 59, 999999999, t.Location()))
}

// BeginOfDay returns the beginning of the day for dt.
func (dt DateTime) BeginOfDay() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()))
}

// EndOfDay returns the end of the day for dt.
func (dt DateTime) EndOfDay() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location()))
}

// BeginOfWeek returns the beginning of the week (Sunday) for dt.
func (dt DateTime) BeginOfWeek() DateTime {
	t := dt.Unwrap()
	weekday := int(t.Weekday())

	return dt.BeginOfDay().AddDays(-weekday)
}

// EndOfWeek returns the end of the week (Saturday) for dt.
func (dt DateTime) EndOfWeek() DateTime {
	t := dt.Unwrap()
	weekday := int(t.Weekday())

	return dt.EndOfDay().AddDays(6 - weekday)
}

// BeginOfMonth returns the beginning of the month for dt.
func (dt DateTime) BeginOfMonth() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()))
}

// EndOfMonth returns the end of the month for dt.
func (dt DateTime) EndOfMonth() DateTime {
	t := dt.Unwrap()
	nextMonth := t.AddDate(0, 1, 0)
	firstOfNextMonth := time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, t.Location())

	return DateTime(firstOfNextMonth.Add(-time.Nanosecond))
}

// BeginOfYear returns the beginning of the year for dt.
func (dt DateTime) BeginOfYear() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location()))
}

// EndOfYear returns the end of the year for dt.
func (dt DateTime) EndOfYear() DateTime {
	t := dt.Unwrap()

	return DateTime(time.Date(t.Year(), 12, 31, 23, 59, 59, 999999999, t.Location()))
}

// BeginOfQuarter returns the beginning of the quarter for dt.
func (dt DateTime) BeginOfQuarter() DateTime {
	t := dt.Unwrap()
	month := ((int(t.Month())-1)/3)*3 + 1

	return DateTime(time.Date(t.Year(), time.Month(month), 1, 0, 0, 0, 0, t.Location()))
}

// EndOfQuarter returns the end of the quarter for dt.
func (dt DateTime) EndOfQuarter() DateTime {
	t := dt.Unwrap()
	month := ((int(t.Month())-1)/3)*3 + 3

	return DateTime(time.Date(t.Year(), time.Month(month+1), 1, 0, 0, 0, 0, t.Location()).Add(-time.Nanosecond))
}

// Monday returns the Monday of the week containing dt.
func (dt DateTime) Monday() DateTime {
	return dt.weekdayOffset(time.Monday)
}

// Tuesday returns the Tuesday of the week containing dt.
func (dt DateTime) Tuesday() DateTime {
	return dt.weekdayOffset(time.Tuesday)
}

// Wednesday returns the Wednesday of the week containing dt.
func (dt DateTime) Wednesday() DateTime {
	return dt.weekdayOffset(time.Wednesday)
}

// Thursday returns the Thursday of the week containing dt.
func (dt DateTime) Thursday() DateTime {
	return dt.weekdayOffset(time.Thursday)
}

// Friday returns the Friday of the week containing dt.
func (dt DateTime) Friday() DateTime {
	return dt.weekdayOffset(time.Friday)
}

// Saturday returns the Saturday of the week containing dt.
func (dt DateTime) Saturday() DateTime {
	return dt.weekdayOffset(time.Saturday)
}

// Sunday returns the Sunday of the week containing dt.
func (dt DateTime) Sunday() DateTime {
	return dt.weekdayOffset(time.Sunday)
}

// weekdayOffset is a helper function to get a specific weekday of the current week.
func (dt DateTime) weekdayOffset(weekday time.Weekday) DateTime {
	t := dt.Unwrap()
	currentWeekday := int(t.Weekday())
	targetWeekday := int(weekday)
	offset := targetWeekday - currentWeekday

	return dt.BeginOfDay().AddDays(offset)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (dt DateTime) MarshalText() ([]byte, error) {
	return []byte(dt.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (dt *DateTime) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}

	*dt = parsed

	return nil
}

// Now returns the current date time in the local timezone.
func Now() DateTime {
	return DateTime(time.Now().Local())
}

// Of converts a time.Time to DateTime.
func Of(t time.Time) DateTime {
	return DateTime(t)
}

// FromUnix returns the DateTime corresponding to the given Unix time, sec seconds and nsec nanoseconds since January 1, 1970 UTC.
func FromUnix(sec, nsec int64) DateTime {
	return DateTime(time.Unix(sec, nsec))
}

// FromUnixMilli returns the DateTime corresponding to the given Unix time, msec milliseconds since January 1, 1970 UTC.
func FromUnixMilli(msec int64) DateTime {
	return DateTime(time.UnixMilli(msec))
}

// FromUnixMicro returns the DateTime corresponding to the given Unix time, usec microseconds since January 1, 1970 UTC.
func FromUnixMicro(usec int64) DateTime {
	return DateTime(time.UnixMicro(usec))
}

// Parse parses a date time string and returns a DateTime.
// First tries with the provided pattern, then falls back to cast.ToTime as a backup.
func Parse(value string, pattern ...string) (DateTime, error) {
	layout := dateTimeLayout
	if len(pattern) > 0 {
		layout = pattern[0]
	}

	parsed, err := parseTimeWithFallback(value, layout)
	if err != nil {
		return DateTime{}, err
	}

	return DateTime(parsed), nil
}
