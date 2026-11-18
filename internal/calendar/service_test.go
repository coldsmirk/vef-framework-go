package calendar

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type loader struct {
	source   *store
	calls    []int
	override map[string]calendar.DayKind
}

func (l *loader) LoadDays(ctx context.Context, start, end timex.Date) ([]calendar.Day, error) {
	l.calls = append(l.calls, start.Year())

	days, err := l.source.LoadDays(ctx, start, end)
	for i := range days {
		if kind, ok := l.override[days[i].Date.String()]; ok {
			days[i].Kind = kind
		}
	}

	return days, err
}

func TestCalendarService(t *testing.T) {
	bus := eventtest.NewFakeBus()
	source := &loader{source: &store{cfg: new(config.CalendarConfig)}, override: map[string]calendar.DayKind{"2025-01-04": calendar.Workday}}
	cal, err := NewService(source, bus)
	require.NoError(t, err, "calendar should subscribe")

	d := timex.DateOf(time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC))

	t.Run("ArithmeticAndCache", func(t *testing.T) {
		days, err := cal.Days(t.Context(), d, d.AddDays(3))
		require.NoError(t, err, "range should load")
		assert.Len(t, days, 4, "both endpoints should be included")
		days[1].Kind = calendar.Holiday
		got, err := cal.Day(t.Context(), d.AddDays(1))
		require.NoError(t, err, "individual day should be cached")
		assert.Equal(t, calendar.Workday, got.Kind, "a caller cannot mutate cached days")

		for _, tc := range []struct{ n, offset int }{{0, 0}, {1, 1}, {2, 3}, {-1, -1}, {-3, -3}, {-4, -4}} {
			date, err := cal.AddWorkdays(t.Context(), d, tc.n)
			require.NoError(t, err, "arithmetic should succeed")
			assert.True(t, date.Equal(d.AddDays(tc.offset)), "offset should account for makeup Saturdays and cross years")
		}

		assert.Equal(t, []int{2025, 2024}, source.calls, "one load per year should serve all queries")
		source.override["2025-01-04"] = calendar.Holiday

		require.NoError(t, calendar.PublishChangedEvent(t.Context(), bus, 2025), "one year should invalidate")
		got, err = cal.Day(t.Context(), d.AddDays(1))
		require.NoError(t, err, "invalidated year should reload")
		assert.Equal(t, calendar.Holiday, got.Kind, "new classification should be visible")
		require.NoError(t, calendar.PublishChangedEvent(t.Context(), bus), "empty event should invalidate all years")
		_, err = cal.Day(t.Context(), d.AddYears(-1))
		require.NoError(t, err, "older year should reload")
		assert.Equal(t, []int{2025, 2024, 2025, 2024}, source.calls, "invalidation scope should match event years")
	})
	t.Run("WorkdaySearchHorizon", func(t *testing.T) {
		empty, err := NewService(&store{cfg: &config.CalendarConfig{Workdays: []int{}}}, eventtest.NewFakeBus())
		require.NoError(t, err, "empty baseline should construct")
		_, err = empty.AddWorkdays(t.Context(), d, 1)
		require.ErrorIs(t, err, errWorkdayHorizon, "a calendar without workdays must terminate")
	})
}

func TestCalendarWindows(t *testing.T) {
	cal, err := NewService(
		&loader{source: &store{cfg: new(config.CalendarConfig)}, override: map[string]calendar.DayKind{"2025-01-01": calendar.Holiday, "2025-01-04": calendar.Workday}},
		eventtest.NewFakeBus(),
	)
	require.NoError(t, err, "calendar should subscribe")

	span := func(h1, h2 int) calendar.TimeRange {
		return calendar.TimeRange{StartsAt: timex.TimeOf(time.Date(2000, 1, 1, h1, 0, 0, 0, time.UTC)), EndsAt: timex.TimeOf(time.Date(2000, 1, 1, h2, 0, 0, 0, time.UTC))}
	}
	instant := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err, "fixture should parse")

		return v
	}
	t.Run("CrossMidnightMergeAndBlackout", func(t *testing.T) {
		rule := calendar.WindowRule{Timezone: "UTC", Windows: []calendar.Window{
			{Weekdays: []int{5}, Times: []calendar.TimeRange{span(22, 2)}}, {Weekdays: []int{6}, Times: []calendar.TimeRange{span(1, 3), span(3, 5)}},
		}, Blackouts: []calendar.Window{{Weekdays: []int{6}, Times: []calendar.TimeRange{span(2, 3)}}}}
		at := instant("2025-01-04T01:00:00Z")
		status, err := cal.Evaluate(t.Context(), rule, at)
		require.NoError(t, err, "overlapping windows should evaluate")
		require.True(t, status.Open, "Friday's window should extend into Saturday")
		require.NotNil(t, status.Current, "open status should carry its interval")
		assert.True(t, status.Current.StartsAt.Equal(instant("2025-01-03T22:00:00Z")), "merged interval should start on Friday")
		assert.True(t, status.Current.EndsAt.Equal(at.Add(time.Hour)), "blackout should split the interval")
		require.NotNil(t, status.Next, "there should be an opening after the blackout")
		assert.True(t, status.Next.StartsAt.Equal(at.Add(2*time.Hour)), "reopens at the end of the blackout")
		assert.True(t, status.Next.EndsAt.Equal(at.Add(4*time.Hour)), "touching windows should merge")
		status, err = cal.Evaluate(t.Context(), rule, at.Add(time.Hour))
		require.NoError(t, err, "boundary should evaluate")
		assert.False(t, status.Open, "an interval excludes its endpoint")
	})
	t.Run("MonthlyOrdinalsAndKinds", func(t *testing.T) {
		rule := calendar.WindowRule{
			Timezone: "UTC",
			Windows:  []calendar.Window{{WorkdayOrdinals: []int{3, -1}, DayKinds: []calendar.DayKind{calendar.Workday}, Times: []calendar.TimeRange{span(12, 0)}}},
		}
		windows, err := cal.Windows(t.Context(), rule, instant("2025-01-01T00:00:00Z"), 2)
		require.NoError(t, err, "monthly ordinals should evaluate")
		require.Len(t, windows, 2, "preview should respect limit")
		assert.True(t, windows[0].StartsAt.Equal(instant("2025-01-04T12:00:00Z")), "makeup Saturday is January's third workday")
		assert.True(t, windows[0].EndsAt.Equal(instant("2025-01-05T00:00:00Z")), "midnight belongs to the following day")
		assert.True(t, windows[1].StartsAt.Equal(instant("2025-01-31T12:00:00Z")), "negative ordinal counts backward")
	})
	t.Run("ContinuousOpenings", func(t *testing.T) {
		at := instant("2025-02-12T10:00:00Z")

		today := timex.DateOf(at)
		for _, tc := range []struct {
			name   string
			window calendar.Window
			want   calendar.Interval
			open   bool
		}{
			{"Always", calendar.Window{}, calendar.Interval{}, true},
			{"WednesdayInsideWorkingWeek", calendar.Window{DayKinds: []calendar.DayKind{calendar.Workday}}, calendar.Interval{StartsAt: instant("2025-02-10T00:00:00Z"), EndsAt: instant("2025-02-15T00:00:00Z")}, true},
			{"FutureLongOpening", calendar.Window{Dates: &calendar.DateRange{StartsAt: today.AddMonths(6), EndsAt: today.AddYears(5)}}, calendar.Interval{StartsAt: today.AddMonths(6).Unwrap()}, false},
			{"ClosedAtMidnightBeforeToday", calendar.Window{Dates: &calendar.DateRange{StartsAt: today, EndsAt: today.AddDays(15)}}, calendar.Interval{StartsAt: today.Unwrap(), EndsAt: today.AddDays(16).Unwrap()}, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rule := calendar.WindowRule{Timezone: "UTC", Windows: []calendar.Window{tc.window}}
				status, err := cal.Evaluate(t.Context(), rule, at)
				require.NoError(t, err, "continuous opening should evaluate")
				assert.Equal(t, tc.open, status.Open, "open status should match the instant")

				got := status.Next
				if tc.open {
					got = status.Current
				}

				require.NotNil(t, got, "matching interval should be present")
				assert.True(t, tc.want.StartsAt.Equal(got.StartsAt), "start must be a true opening or unbounded: %v", got.StartsAt)
				assert.True(t, tc.want.EndsAt.Equal(got.EndsAt), "end must be a true closure or unbounded: %v", got.EndsAt)
				ws, err := cal.Windows(t.Context(), rule, at, 1)
				require.NoError(t, err, "window list should use the same projection")
				require.Len(t, ws, 1, "window list should respect the limit")
				assert.Equal(t, *got, ws[0], "status and list must agree on bounds")
			})
		}
	})
	t.Run("Horizon", func(t *testing.T) {
		at := instant("2025-01-01T00:00:00Z")
		for _, n := range []int{366, 367} {
			d := timex.DateOf(at.AddDate(0, 0, n))
			status, err := cal.Evaluate(t.Context(), calendar.WindowRule{Timezone: "UTC", Windows: []calendar.Window{{Dates: &calendar.DateRange{StartsAt: d, EndsAt: d}}}}, at)
			require.NoError(t, err, "bounded rule should evaluate")
			assert.Equal(t, n == 366, status.Next != nil, "only openings within the 366-day horizon should be returned")
		}
	})
	t.Run("SpringGapAndFallOverlap", func(t *testing.T) {
		rule := calendar.WindowRule{Timezone: "America/New_York", Windows: []calendar.Window{{Weekdays: []int{7}, Times: []calendar.TimeRange{span(2, 4)}}}}
		ws, err := cal.Windows(t.Context(), rule, instant("2025-03-09T06:00:00Z"), 1)
		require.NoError(t, err, "gap should normalize forward")
		require.Len(t, ws, 1, "spring Sunday should have one interval")
		assert.True(t, ws[0].StartsAt.Equal(instant("2025-03-09T07:00:00Z")), "missing 02:00 should start at 03:00 EDT")
		assert.Equal(t, time.Hour, ws[0].EndsAt.Sub(ws[0].StartsAt), "spring gap reduces elapsed duration")

		rule.Windows[0].Times = nil
		ws, err = cal.Windows(t.Context(), rule, instant("2025-11-02T12:00:00Z"), 1)
		require.NoError(t, err, "fall Sunday should resolve in the rule zone")
		require.Len(t, ws, 1, "fall Sunday should have one interval")
		assert.Equal(t, 25*time.Hour, ws[0].EndsAt.Sub(ws[0].StartsAt), "whole fall-back day spans 25 elapsed hours")
	})
}
