package timex

import "time"

// MakeTime builds a time.Time in the local timezone for use as a test fixture across the package.
func MakeTime(year int, month time.Month, day, hour, minutes, seconds int) time.Time {
	return time.Date(year, month, day, hour, minutes, seconds, 0, time.Local)
}

// MakeTimeUTC builds a time.Time in UTC for use as a test fixture across the package.
func MakeTimeUTC(year int, month time.Month, day, hour, minutes, seconds int) time.Time {
	return time.Date(year, month, day, hour, minutes, seconds, 0, time.UTC)
}
