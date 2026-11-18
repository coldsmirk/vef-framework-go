package calendar

import "errors"

var (
	errInvalidLoaderResult = errors.New("calendar loader returned an invalid year")
	errReversedDates       = errors.New("calendar: end date precedes start date")
	errWorkdayHorizon      = errors.New("calendar: workday search exceeded 3660 days")
)
