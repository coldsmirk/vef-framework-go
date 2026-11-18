package civil

import "time"

// Filter matches the calendar-independent, recurring dimensions of a window.
type Filter struct {
	months, weekdays, monthDays, fromEnd uint32
}

// NewFilter compiles validated, possibly repeated alternatives into masks.
func NewFilter(months, weekdays, monthDays []int) Filter {
	var f Filter
	for _, n := range months {
		f.months |= 1 << n
	}

	for _, n := range weekdays {
		f.weekdays |= 1 << n
	}

	for _, n := range monthDays {
		if n > 0 {
			f.monthDays |= 1 << n
		} else {
			f.fromEnd |= 1 << -n
		}
	}

	return f
}

// Matches reports whether the date satisfies every configured dimension.
func (f Filter) Matches(d time.Time) bool {
	if f.months != 0 && f.months&(1<<d.Month()) == 0 {
		return false
	}

	if f.weekdays != 0 && f.weekdays&(1<<((int(d.Weekday())+6)%7+1)) == 0 {
		return false
	}

	if f.monthDays|f.fromEnd == 0 || f.monthDays&(1<<d.Day()) != 0 {
		return true
	}

	last := time.Date(d.Year(), d.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1).Day()

	return f.fromEnd&(1<<(last-d.Day()+1)) != 0
}
