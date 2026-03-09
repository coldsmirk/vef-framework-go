package timex

import "time"

func MakeTime(year int, month time.Month, day, hour, minutes, seconds int) time.Time {
	return time.Date(year, month, day, hour, minutes, seconds, 0, time.Local)
}

func MakeTimeUTC(year int, month time.Month, day, hour, minutes, seconds int) time.Time {
	return time.Date(year, month, day, hour, minutes, seconds, 0, time.UTC)
}
