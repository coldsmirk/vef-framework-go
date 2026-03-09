package timex

import (
	"database/sql/driver"
	"time"

	"github.com/gofiber/utils/v2"
)

// Date represents a date value (without time) with database and JSON support.
// It uses the standard time.DateOnly format (2006-01-02).
type Date time.Time

// Unwrap returns the underlying time.Time value.
func (d Date) Unwrap() time.Time {
	return time.Time(d)
}

// Format returns the string representation using the provided layout.
func (d Date) Format(layout string) string {
	return time.Time(d).Format(layout)
}

// Scan implements the sql.Scanner interface for database compatibility.
func (d *Date) Scan(src any) error {
	return scanTimeValue(src, func(s string) (any, error) {
		return ParseDate(s)
	}, func(t time.Time) any {
		return DateOf(t)
	}, "date", d)
}

// Value implements the driver.Valuer interface for database compatibility.
func (d Date) Value() (driver.Value, error) {
	return d.String(), nil
}

// String returns the string representation using the standard DateOnly layout.
func (d Date) String() string {
	return time.Time(d).Format(time.DateOnly)
}

// MarshalJSON implements the json.Marshaler interface for JSON serialization.
func (d Date) MarshalJSON() ([]byte, error) {
	bs := make([]byte, 0, datePatternLength+2)
	bs = append(bs, jsonQuote)
	bs = time.Time(d).AppendFormat(bs, time.DateOnly)
	bs = append(bs, jsonQuote)

	return bs, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSON deserialization.
func (d *Date) UnmarshalJSON(bs []byte) error {
	value := utils.UnsafeString(bs)
	if value == jsonNull {
		return nil
	}

	if err := validateJSONFormat(bs, datePatternLength); err != nil {
		return ErrInvalidDateFormat
	}

	parsed, err := ParseDate(value[1 : datePatternLength+1])
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// Equal compares two Date values for equality.
func (d Date) Equal(other Date) bool {
	return d.Unwrap().Equal(other.Unwrap())
}

// Before reports whether the date d is before other.
func (d Date) Before(other Date) bool {
	return d.Unwrap().Before(other.Unwrap())
}

// After reports whether the date d is after other.
func (d Date) After(other Date) bool {
	return d.Unwrap().After(other.Unwrap())
}

// AddDays adds the specified number of days to the date.
func (d Date) AddDays(days int) Date {
	return DateOf(d.Unwrap().AddDate(0, 0, days))
}

// AddMonths adds the specified number of months to the date.
func (d Date) AddMonths(months int) Date {
	return DateOf(d.Unwrap().AddDate(0, months, 0))
}

// AddYears adds the specified number of years to the date.
func (d Date) AddYears(years int) Date {
	return DateOf(d.Unwrap().AddDate(years, 0, 0))
}

// Year returns the year in which d occurs.
func (d Date) Year() int {
	return d.Unwrap().Year()
}

// Month returns the month of the year specified by d.
func (d Date) Month() time.Month {
	return d.Unwrap().Month()
}

// Day returns the day of the month specified by d.
func (d Date) Day() int {
	return d.Unwrap().Day()
}

// Weekday returns the day of the week specified by d.
func (d Date) Weekday() time.Weekday {
	return d.Unwrap().Weekday()
}

// YearDay returns the day of the year specified by d, in the range [1,365] for non-leap years,
// and [1,366] in leap years.
func (d Date) YearDay() int {
	return d.Unwrap().YearDay()
}

// Location returns the time zone information associated with d.
func (d Date) Location() *time.Location {
	return d.Unwrap().Location()
}

// Sub returns the duration d-other.
func (d Date) Sub(other Date) time.Duration {
	return d.Unwrap().Sub(other.Unwrap())
}

// Since returns the time elapsed since d (equivalent to time.Since).
func (d Date) Since() time.Duration {
	return time.Since(d.Unwrap())
}

// Until returns the duration until d (equivalent to time.Until).
func (d Date) Until() time.Duration {
	return time.Until(d.Unwrap())
}

// IsZero reports whether d represents the zero time instant, January 1, year 1.
func (d Date) IsZero() bool {
	return d.Unwrap().IsZero()
}

// Unix returns d as a Unix time, the number of seconds elapsed since January 1, 1970 UTC.
func (d Date) Unix() int64 {
	return d.Unwrap().Unix()
}

// Between reports whether d is between start and end.
func (d Date) Between(start, end Date) bool {
	return d.After(start) && d.Before(end)
}

// BeginOfDay returns the beginning of the day for d (same as the date itself).
func (d Date) BeginOfDay() Date {
	return d
}

// EndOfDay returns the end of the day for d (same as the date itself).
func (d Date) EndOfDay() Date {
	return d
}

// BeginOfWeek returns the beginning of the week (Sunday) for d.
func (d Date) BeginOfWeek() Date {
	t := d.Unwrap()
	weekday := int(t.Weekday())

	return d.AddDays(-weekday)
}

// EndOfWeek returns the end of the week (Saturday) for d.
func (d Date) EndOfWeek() Date {
	t := d.Unwrap()
	weekday := int(t.Weekday())

	return d.AddDays(6 - weekday)
}

// BeginOfMonth returns the beginning of the month for d.
func (d Date) BeginOfMonth() Date {
	t := d.Unwrap()

	return DateOf(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()))
}

// EndOfMonth returns the end of the month for d.
func (d Date) EndOfMonth() Date {
	t := d.Unwrap()
	nextMonth := t.AddDate(0, 1, 0)
	firstOfNextMonth := time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, t.Location())

	return DateOf(firstOfNextMonth.AddDate(0, 0, -1))
}

// BeginOfQuarter returns the beginning of the quarter for d.
func (d Date) BeginOfQuarter() Date {
	t := d.Unwrap()
	month := ((int(t.Month())-1)/3)*3 + 1

	return DateOf(time.Date(t.Year(), time.Month(month), 1, 0, 0, 0, 0, t.Location()))
}

// EndOfQuarter returns the end of the quarter for d.
func (d Date) EndOfQuarter() Date {
	t := d.Unwrap()
	month := ((int(t.Month())-1)/3)*3 + 3
	lastDayOfQuarter := time.Date(t.Year(), time.Month(month+1), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 0, -1)

	return DateOf(lastDayOfQuarter)
}

// BeginOfYear returns the beginning of the year for d.
func (d Date) BeginOfYear() Date {
	t := d.Unwrap()

	return DateOf(time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location()))
}

// EndOfYear returns the end of the year for d.
func (d Date) EndOfYear() Date {
	t := d.Unwrap()

	return DateOf(time.Date(t.Year(), 12, 31, 0, 0, 0, 0, t.Location()))
}

// Monday returns the Monday of the week containing d.
func (d Date) Monday() Date {
	return d.weekdayOffset(time.Monday)
}

// Tuesday returns the Tuesday of the week containing d.
func (d Date) Tuesday() Date {
	return d.weekdayOffset(time.Tuesday)
}

// Wednesday returns the Wednesday of the week containing d.
func (d Date) Wednesday() Date {
	return d.weekdayOffset(time.Wednesday)
}

// Thursday returns the Thursday of the week containing d.
func (d Date) Thursday() Date {
	return d.weekdayOffset(time.Thursday)
}

// Friday returns the Friday of the week containing d.
func (d Date) Friday() Date {
	return d.weekdayOffset(time.Friday)
}

// Saturday returns the Saturday of the week containing d.
func (d Date) Saturday() Date {
	return d.weekdayOffset(time.Saturday)
}

// Sunday returns the Sunday of the week containing d.
func (d Date) Sunday() Date {
	return d.weekdayOffset(time.Sunday)
}

// weekdayOffset is a helper function to get a specific weekday of the current week.
func (d Date) weekdayOffset(weekday time.Weekday) Date {
	t := d.Unwrap()
	currentWeekday := int(t.Weekday())
	targetWeekday := int(weekday)
	offset := targetWeekday - currentWeekday

	return d.AddDays(offset)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (d Date) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (d *Date) UnmarshalText(text []byte) error {
	parsed, err := ParseDate(string(text))
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// NowDate returns the current date in the local timezone.
func NowDate() Date {
	return DateOf(time.Now().Local())
}

// DateOf returns the date of the given time (time components are zeroed).
func DateOf(t time.Time) Date {
	return Date(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()))
}

// ParseDate parses a date string and returns a Date.
// First tries with the provided pattern, then falls back to cast.ToTime as a backup.
func ParseDate(value string, pattern ...string) (Date, error) {
	layout := dateLayout
	if len(pattern) > 0 {
		layout = pattern[0]
	}

	parsed, err := parseTimeWithFallback(value, layout)
	if err != nil {
		return Date{}, err
	}

	return DateOf(parsed), nil
}
