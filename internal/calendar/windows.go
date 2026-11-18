package calendar

import (
	"context"
	"slices"
	"time"

	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/internal/calendar/civil"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func (s *service) Evaluate(ctx context.Context, rule calendar.WindowRule, at time.Time) (calendar.WindowStatus, error) {
	windows, err := s.Windows(ctx, rule, at, 2)
	if err != nil {
		return calendar.WindowStatus{}, err
	}

	return windowStatus(windows, at), nil
}

func windowStatus(windows []calendar.Interval, at time.Time) calendar.WindowStatus {
	var status calendar.WindowStatus
	if len(windows) > 0 && windows[0].Contains(at) {
		status.Open = true
		status.Current = &windows[0]
		windows = windows[1:]
	}

	if len(windows) > 0 {
		status.Next = &windows[0]
	}

	return status
}

func (s *service) Windows(ctx context.Context, rule calendar.WindowRule, from time.Time, limit int) ([]calendar.Interval, error) {
	if err := rule.Validate(); err != nil {
		return nil, err
	}

	windows := []calendar.Interval{}
	if limit <= 0 {
		return windows, nil
	}

	loc := time.Local
	if rule.Timezone != "" {
		var err error

		loc, err = time.LoadLocation(rule.Timezone)
		if err != nil {
			return nil, err
		}
	}

	local := from.In(loc)
	// Use UTC to step between civil dates even where local midnight is skipped.
	today := timex.DateOf(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC))
	p := projection{read: s.reader(ctx), rule: rule, loc: loc, needsDays: needsCalendar(rule.Windows) || needsCalendar(rule.Blackouts)}

	windows, err := p.forward(ctx, today, from, limit)
	if err != nil {
		return nil, err
	}

	if len(windows) > 0 && windows[0].Contains(from) && windows[0].StartsAt.Equal(wallTime(today, timex.Time{}, loc)) {
		windows[0].StartsAt, err = p.startBefore(ctx, today, local.AddDate(0, 0, -366))
		if err != nil {
			return nil, err
		}
	}

	return windows, nil
}

type projection struct {
	read      yearReader
	rule      calendar.WindowRule
	loc       *time.Location
	needsDays bool
}

const blockDays = 7

// forward joins adjacent blocks until the requested intervals have actually
// closed or the upper horizon is crossed. A boundary exactly at the horizon
// is included when it is a genuine transition.
func (p *projection) forward(ctx context.Context, today timex.Date, from time.Time, limit int) ([]calendar.Interval, error) {
	windows := []calendar.Interval{}
	upper := from.In(p.loc).AddDate(0, 0, 366)

	last := today.AddDays(366)
	for start := today; !start.After(last); start = start.AddDays(blockDays) {
		end := start.AddDays(blockDays - 1)
		if end.After(last) {
			end = last
		}

		spans, err := p.block(ctx, start, end)
		if err != nil {
			return nil, err
		}

		for _, span := range spans {
			if !span.EndsAt.After(from) {
				continue
			}

			if len(windows) > 0 && windows[len(windows)-1].EndsAt.Equal(span.StartsAt) {
				windows[len(windows)-1].EndsAt = span.EndsAt
			} else {
				if len(windows) == limit || span.StartsAt.After(upper) {
					return windows, nil
				}

				windows = append(windows, span)
			}

			if span.EndsAt.After(upper) {
				windows[len(windows)-1].EndsAt = time.Time{}

				return windows, nil
			}
		}

		if len(windows) == limit && windows[len(windows)-1].EndsAt.Before(wallTime(end.AddDays(1), timex.Time{}, p.loc)) {
			break
		}
	}

	return windows, nil
}

// startBefore follows an open run backward from midnight. Each preceding
// block either reveals its start, reaches the lower horizon, or joins it.
func (p *projection) startBefore(ctx context.Context, today timex.Date, lower time.Time) (time.Time, error) {
	boundary := wallTime(today, timex.Time{}, p.loc)
	for end := today.AddDays(-1); !boundary.Before(lower); {
		start := end.AddDays(1 - blockDays)
		if floor := today.AddDays(-367); start.Before(floor) {
			start = floor
		}

		spans, err := p.block(ctx, start, end)
		if err != nil {
			return time.Time{}, err
		}

		if len(spans) == 0 || !spans[len(spans)-1].EndsAt.Equal(boundary) {
			return boundary, nil
		}

		boundary = spans[len(spans)-1].StartsAt
		if boundary.Before(lower) {
			break
		}

		if !boundary.Equal(wallTime(start, timex.Time{}, p.loc)) {
			return boundary, nil
		}

		end = start.AddDays(-1)
	}

	return time.Time{}, nil
}

// block projects the inclusive civil dates into half-open intervals clipped
// at its two midnights. The preceding anchor contributes overnight spans.
func (p *projection) block(ctx context.Context, start, end timex.Date) ([]calendar.Interval, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	anchor := start.AddDays(-1)

	var kinds []classifiedDay
	if p.needsDays {
		days, err := p.read.days(anchor.BeginOfMonth(), end.EndOfMonth())
		if err != nil {
			return nil, err
		}

		kinds = classify(days)[anchor.Day()-1:]
	}

	opens := intervals(p.rule.Windows, anchor, end, p.loc, kinds)
	blackouts := intervals(p.rule.Blackouts, anchor, end, p.loc, kinds)
	left, right := wallTime(start, timex.Time{}, p.loc), wallTime(end.AddDays(1), timex.Time{}, p.loc)
	spans := subtract(opens, blackouts)

	clipped := spans[:0]
	for _, span := range spans {
		if span.StartsAt.Before(left) {
			span.StartsAt = left
		}

		if span.EndsAt.After(right) {
			span.EndsAt = right
		}

		if span.EndsAt.After(span.StartsAt) {
			clipped = append(clipped, span)
		}
	}

	return clipped, nil
}

func needsCalendar(windows []calendar.Window) bool {
	return slices.ContainsFunc(windows, func(w calendar.Window) bool { return len(w.DayKinds) > 0 || len(w.WorkdayOrdinals) > 0 })
}

type classifiedDay struct {
	kind             calendar.DayKind
	ordinal, fromEnd int
}

// classify receives complete months, so the last workday may precede month end.
func classify(days []calendar.Day) []classifiedDay {
	result := make([]classifiedDay, len(days))

	ordinal, start := 0, 0
	for i, d := range days {
		result[i].kind = d.Kind
		if d.Kind == calendar.Workday {
			ordinal++
			result[i].ordinal = ordinal
		}

		if i+1 == len(days) || days[i+1].Date.Day() == 1 {
			for j := start; j <= i; j++ {
				if result[j].ordinal != 0 {
					result[j].fromEnd = result[j].ordinal - ordinal - 1
				}
			}

			start = i + 1
			ordinal = 0
		}
	}

	return result
}

func intervals(windows []calendar.Window, first, last timex.Date, loc *time.Location, kinds []classifiedDay) []calendar.Interval {
	var all []calendar.Interval
	for _, w := range windows {
		filter := civil.NewFilter(w.Months, w.Weekdays, w.MonthDays)
		for i, d := 0, first; !d.After(last); i, d = i+1, d.AddDays(1) {
			if w.Dates != nil && (d.Before(w.Dates.StartsAt) || d.After(w.Dates.EndsAt)) || !filter.Matches(d.Unwrap()) {
				continue
			}

			if len(w.DayKinds) > 0 && !slices.Contains(w.DayKinds, kinds[i].kind) {
				continue
			}

			if len(w.WorkdayOrdinals) > 0 && !slices.Contains(w.WorkdayOrdinals, kinds[i].ordinal) && !slices.Contains(w.WorkdayOrdinals, kinds[i].fromEnd) {
				continue
			}

			if len(w.Times) == 0 {
				all = append(all, calendar.Interval{StartsAt: wallTime(d, timex.Time{}, loc), EndsAt: wallTime(d.AddDays(1), timex.Time{}, loc)})
			}

			for _, span := range w.Times {
				end := d
				if span.EndsAt.Before(span.StartsAt) {
					end = end.AddDays(1)
				}

				lo, hi := wallTime(d, span.StartsAt, loc), wallTime(end, span.EndsAt, loc)
				if hi.After(lo) {
					all = append(all, calendar.Interval{StartsAt: lo, EndsAt: hi})
				}
			}
		}
	}

	slices.SortFunc(all, func(a, b calendar.Interval) int { return a.StartsAt.Compare(b.StartsAt) })

	merged := all[:0]
	for _, span := range all {
		if len(merged) == 0 || span.StartsAt.After(merged[len(merged)-1].EndsAt) {
			merged = append(merged, span)

			continue
		}

		tail := &merged[len(merged)-1]
		if span.EndsAt.After(tail.EndsAt) {
			tail.EndsAt = span.EndsAt
		}
	}

	return merged
}

func wallTime(d timex.Date, t timex.Time, loc *time.Location) time.Time {
	wall := time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	instant := time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
	// Go may normalize a nonexistent wall time backward. Move it forward by the gap instead.
	actual := time.Date(instant.Year(), instant.Month(), instant.Day(), instant.Hour(), instant.Minute(), instant.Second(), instant.Nanosecond(), time.UTC)
	if actual.Before(wall) {
		instant = instant.Add(wall.Sub(actual))
	}

	return instant
}

// subtract walks two sorted unions, preserving half-open boundaries.
func subtract(open, closed []calendar.Interval) []calendar.Interval {
	var result []calendar.Interval
	for _, span := range open {
		for len(closed) > 0 && !closed[0].EndsAt.After(span.StartsAt) {
			closed = closed[1:]
		}

		start := span.StartsAt
		for _, cut := range closed {
			if !cut.StartsAt.Before(span.EndsAt) {
				break
			}

			if cut.StartsAt.After(start) {
				result = append(result, calendar.Interval{StartsAt: start, EndsAt: cut.StartsAt})
			}

			start = cut.EndsAt
			if !start.Before(span.EndsAt) {
				break
			}
		}

		if start.Before(span.EndsAt) {
			result = append(result, calendar.Interval{StartsAt: start, EndsAt: span.EndsAt})
		}
	}

	return result
}
