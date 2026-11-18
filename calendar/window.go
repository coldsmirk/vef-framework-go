package calendar

import (
	"encoding/json"
	"fmt"
	"time"

	// Standalone calendar users can run without a system zoneinfo database.
	_ "time/tzdata"

	"github.com/coldsmirk/vef-framework-go/internal/calendar/civil"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// WindowRule opens the union of Windows after subtracting Blackouts.
type WindowRule struct {
	Timezone  string   `json:"timezone,omitempty"`
	Windows   []Window `json:"windows"`
	Blackouts []Window `json:"blackouts,omitempty"`
}

// Window intersects its nonempty dimensions. Negative positions count from the end.
type Window struct {
	Months          []int       `json:"months,omitempty"`
	MonthDays       []int       `json:"monthDays,omitempty"`
	Weekdays        []int       `json:"weekdays,omitempty"`
	DayKinds        []DayKind   `json:"dayKinds,omitempty"`
	WorkdayOrdinals []int       `json:"workdayOrdinals,omitempty"`
	Times           []TimeRange `json:"times,omitempty"`
	Dates           *DateRange  `json:"dates,omitempty"`
}

// TimeRange is half-open. An earlier closing time belongs to the next date.
type TimeRange struct {
	StartsAt timex.Time `json:"startsAt"`
	EndsAt   timex.Time `json:"endsAt"`
}

// DateRange includes both boundary dates.
type DateRange struct {
	StartsAt timex.Date `json:"startsAt"`
	EndsAt   timex.Date `json:"endsAt"`
}

// Interval is a half-open span of instants. A zero endpoint is unbounded on
// that side and is omitted from the Unix-millisecond wire representation.
type Interval struct {
	StartsAt time.Time
	EndsAt   time.Time
}

// Contains reports whether t lies in the interval, honoring unbounded sides.
func (i Interval) Contains(t time.Time) bool {
	return (i.StartsAt.IsZero() || !t.Before(i.StartsAt)) && (i.EndsAt.IsZero() || t.Before(i.EndsAt))
}

type wireInterval struct {
	StartsAtUnixMs *int64 `json:"startsAtUnixMs,omitempty"`
	EndsAtUnixMs   *int64 `json:"endsAtUnixMs,omitempty"`
}

// MarshalJSON encodes both instants in Unix milliseconds.
func (i Interval) MarshalJSON() ([]byte, error) {
	var w wireInterval
	if !i.StartsAt.IsZero() {
		w.StartsAtUnixMs = new(i.StartsAt.UnixMilli())
	}

	if !i.EndsAt.IsZero() {
		w.EndsAtUnixMs = new(i.EndsAt.UnixMilli())
	}

	return json.Marshal(w)
}

// UnmarshalJSON decodes both instants from Unix milliseconds.
func (i *Interval) UnmarshalJSON(data []byte) error {
	var w wireInterval
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	*i = Interval{}
	if w.StartsAtUnixMs != nil {
		i.StartsAt = time.UnixMilli(*w.StartsAtUnixMs)
	}

	if w.EndsAtUnixMs != nil {
		i.EndsAt = time.UnixMilli(*w.EndsAtUnixMs)
	}

	return nil
}

// WindowStatus describes the interval containing an instant and the next opening.
type WindowStatus struct {
	Open    bool      `json:"open"`
	Current *Interval `json:"current,omitempty"`
	Next    *Interval `json:"next,omitempty"`
}

// Validate checks the rule without consulting the work calendar.
func (r WindowRule) Validate() error {
	if len(r.Windows) == 0 {
		return invalidRule("windows must contain at least one window")
	}

	if _, err := time.LoadLocation(r.Timezone); err != nil || r.Timezone == "Local" {
		return invalidRule("unknown timezone " + r.Timezone)
	}

	for _, group := range []struct {
		name    string
		windows []Window
	}{{"windows", r.Windows}, {"blackouts", r.Blackouts}} {
		for i, w := range group.windows {
			if reason := w.invalidReason(); reason != "" {
				return invalidRule(fmt.Sprintf("%s[%d]: %s", group.name, i, reason))
			}
		}
	}

	return nil
}

func (w Window) invalidReason() string {
	for _, dim := range []struct {
		name     string
		values   []int
		min, max int
	}{
		{"months", w.Months, 1, 12}, {"monthDays", w.MonthDays, -31, 31}, {"weekdays", w.Weekdays, 1, 7}, {"workdayOrdinals", w.WorkdayOrdinals, -23, 23},
	} {
		for _, n := range dim.values {
			if n == 0 || n < dim.min || n > dim.max {
				return fmt.Sprintf("%s contains invalid value %d", dim.name, n)
			}
		}
	}

	for _, k := range w.DayKinds {
		if !k.IsValid() {
			return fmt.Sprintf("unknown day kind %q", k)
		}
	}

	for _, span := range w.Times {
		if span.StartsAt.Equal(span.EndsAt) {
			return "time range endpoints must differ"
		}
	}

	first := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if w.Dates != nil {
		if w.Dates.StartsAt.IsZero() || w.Dates.EndsAt.IsZero() {
			return "date range requires both endpoints"
		}

		if w.Dates.StartsAt.After(w.Dates.EndsAt) {
			return "date range ends before it starts"
		}

		d := w.Dates.StartsAt
		first = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	}
	// Gregorian month lengths and weekdays repeat exactly every 400 years.
	end := first.AddDate(400, 0, 0)
	if w.Dates != nil {
		d := w.Dates.EndsAt

		bound := time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, time.UTC)
		if bound.Before(end) {
			end = bound
		}
	}

	filter := civil.NewFilter(w.Months, w.Weekdays, w.MonthDays)
	for d := first; d.Before(end); d = d.AddDate(0, 0, 1) {
		if filter.Matches(d) {
			return ""
		}
	}

	return "no date matches the window"
}
