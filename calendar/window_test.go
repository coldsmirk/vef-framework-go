package calendar

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestWindowRuleValidate(t *testing.T) {
	midnight := timex.Time{}
	noon := timex.TimeOf(time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC))

	monday := timex.DateOf(time.Date(2100, 3, 1, 0, 0, 0, 0, time.UTC))
	for _, tc := range []struct {
		name  string
		w     Window
		valid bool
	}{
		{"Unconstrained", Window{}, true},
		{"Boundaries", Window{Months: []int{1, 12}, MonthDays: []int{-31, -1, 31}, Weekdays: []int{1, 7}, WorkdayOrdinals: []int{-23, 23}, DayKinds: []DayKind{Workday, RestDay, Holiday}}, true},
		{"LeapMonday", Window{Months: []int{2}, MonthDays: []int{29}, Weekdays: []int{1}}, true},
		{"ImpossibleDate", Window{Months: []int{2}, MonthDays: []int{30}}, false},
		{"EmptyDateIntersection", Window{Dates: &DateRange{monday, monday}, Weekdays: []int{7}}, false},
		{"ReversedDates", Window{Dates: &DateRange{monday, monday.AddDays(-1)}}, false},
		{"SameDay", Window{Dates: &DateRange{monday, monday}, Weekdays: []int{1}}, true},
		{"MonthOutOfRange", Window{Months: []int{13}}, false},
		{"NegativeMonthDayOutOfRange", Window{MonthDays: []int{-32}}, false},
		{"ZeroMonthDay", Window{MonthDays: []int{0}}, false},
		{"ZeroWeekday", Window{Weekdays: []int{0}}, false},
		{"InvalidKind", Window{DayKinds: []DayKind{"other"}}, false},
		{"ZeroOrdinal", Window{WorkdayOrdinals: []int{0}}, false},
		{"OrdinalOutOfRange", Window{WorkdayOrdinals: []int{24}}, false},
		{"MidnightClosing", Window{Times: []TimeRange{{noon, midnight}}}, true},
		{"EqualEndpoints", Window{Times: []TimeRange{{noon, noon}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := WindowRule{Windows: []Window{tc.w}}
			if tc.valid {
				require.NoError(t, rule.Validate(), "valid window should be accepted")
			} else {
				require.ErrorIs(t, rule.Validate(), ErrInvalidWindowRule, "invalid window should report a rule fault")
			}
		})
	}

	t.Run("RuleStructure", func(t *testing.T) {
		for _, r := range []WindowRule{{}, {Timezone: "bad/zone", Windows: []Window{{}}}, {Windows: []Window{{}}, Blackouts: []Window{{Months: []int{0}}}}} {
			require.ErrorIs(t, r.Validate(), ErrInvalidWindowRule, "invalid rule or blackout should be rejected")
		}
	})
}

func TestIntervalJSON(t *testing.T) {
	t.Run("UnboundedAndEpoch", func(t *testing.T) {
		epoch := time.UnixMilli(0)
		for _, tc := range []struct {
			wire     string
			value    Interval
			contains bool
		}{
			{"{}", Interval{}, true},
			{`{"startsAtUnixMs":0}`, Interval{StartsAt: epoch}, true},
			{`{"endsAtUnixMs":0}`, Interval{EndsAt: epoch}, false},
		} {
			data, err := json.Marshal(tc.value)
			require.NoError(t, err, "unbounded interval should encode")
			assert.JSONEq(t, tc.wire, string(data), "only present endpoints should be encoded")

			got := Interval{StartsAt: epoch, EndsAt: epoch}
			require.NoError(t, json.Unmarshal(data, &got), "unbounded interval should decode")
			assert.Equal(t, tc.value, got, "absent bounds should clear existing values")
			assert.Equal(t, tc.contains, got.Contains(epoch), "contains should respect half-open and unbounded ends")
		}
	})

	i := Interval{time.UnixMilli(-10), time.UnixMilli(100)}
	data, err := json.Marshal(i)
	require.NoError(t, err, "interval should encode")
	assert.JSONEq(t, `{"startsAtUnixMs":-10,"endsAtUnixMs":100}`, string(data), "wire endpoints should be milliseconds")

	var got Interval
	require.NoError(t, json.Unmarshal(data, &got), "wire interval should decode")
	assert.Equal(t, i, got, "instants should survive the round trip")
}
