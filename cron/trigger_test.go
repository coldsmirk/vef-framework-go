package cron

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()

	location, err := time.LoadLocation(name)
	require.NoError(t, err, "Timezone %q must load", name)

	return location
}

func TestTriggerConstructors(t *testing.T) {
	t.Run("Expr", func(t *testing.T) {
		spec := Expr("0 2 * * *", "Asia/Shanghai")

		assert.Equal(t, TriggerCron, spec.Kind, "Expr must build a cron trigger")
		assert.Equal(t, "0 2 * * *", spec.Expr, "Expression must carry through")
		assert.Equal(t, "Asia/Shanghai", spec.Timezone, "Timezone must carry through")
	})

	t.Run("ExprDefaultsToUTC", func(t *testing.T) {
		spec := Expr("0 2 * * *", "")

		assert.Equal(t, DefaultTimezone, spec.Timezone, "An empty timezone must canonicalize to UTC")
	})

	t.Run("Every", func(t *testing.T) {
		spec := Every(90 * time.Second)

		assert.Equal(t, TriggerInterval, spec.Kind, "Every must build an interval trigger")
		assert.Equal(t, int64(90_000), spec.EveryMs, "Interval must convert to milliseconds")
	})

	t.Run("Once", func(t *testing.T) {
		at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		spec := Once(at)

		assert.Equal(t, TriggerOnce, spec.Kind, "Once must build a one-shot trigger")
		require.NotNil(t, spec.At, "Fire time must be set")
		assert.True(t, spec.At.Equal(at), "Fire time must carry through")
	})
}

func TestTriggerValidate(t *testing.T) {
	future := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		spec    TriggerSpec
		wantErr error
	}{
		{name: "FiveFieldCron", spec: Expr("*/5 * * * *", "")},
		{name: "SixFieldCronWithSeconds", spec: Expr("30 */5 * * * *", "UTC")},
		{name: "Descriptor", spec: Expr("@daily", "Asia/Shanghai")},
		{name: "BlankExpression", spec: Expr("", ""), wantErr: ErrTriggerExprRequired},
		{name: "GarbageExpression", spec: Expr("not a cron", ""), wantErr: ErrTriggerExprInvalid},
		{name: "SevenFields", spec: Expr("* * * * * * *", ""), wantErr: ErrTriggerExprInvalid},
		{name: "BadTimezone", spec: Expr("0 2 * * *", "Mars/Olympus"), wantErr: ErrTriggerTimezoneInvalid},
		{name: "ProcessLocalTimezone", spec: Expr("0 2 * * *", "Local"), wantErr: ErrTriggerTimezoneInvalid},
		{name: "IntervalAtMinimum", spec: Every(MinInterval)},
		{name: "IntervalBelowMinimum", spec: Every(999 * time.Millisecond), wantErr: ErrTriggerIntervalTooShort},
		{name: "IntervalZero", spec: Every(0), wantErr: ErrTriggerIntervalTooShort},
		{name: "IntervalNegative", spec: Every(-time.Minute), wantErr: ErrTriggerIntervalTooShort},
		{
			name:    "IntervalNegativeOverflow",
			spec:    TriggerSpec{Kind: TriggerInterval, EveryMs: -1 << 63},
			wantErr: ErrTriggerIntervalTooShort,
		},
		{
			name:    "IntervalBeyondDurationRange",
			spec:    TriggerSpec{Kind: TriggerInterval, EveryMs: maxDurationMilliseconds + 1},
			wantErr: ErrTriggerIntervalTooLong,
		},
		{name: "OnceWithFireTime", spec: Once(future)},
		{name: "OnceZeroFireTime", spec: Once(time.Time{}), wantErr: ErrTriggerFireTimeRequired},
		{name: "OnceNilFireTime", spec: TriggerSpec{Kind: TriggerOnce}, wantErr: ErrTriggerFireTimeRequired},
		{
			name:    "CronWithIntervalField",
			spec:    TriggerSpec{Kind: TriggerCron, Expr: "0 2 * * *", EveryMs: MinInterval.Milliseconds()},
			wantErr: ErrTriggerFieldsConflict,
		},
		{
			name:    "IntervalWithCronField",
			spec:    TriggerSpec{Kind: TriggerInterval, EveryMs: MinInterval.Milliseconds(), Expr: "0 2 * * *"},
			wantErr: ErrTriggerFieldsConflict,
		},
		{
			name:    "OnceWithIntervalField",
			spec:    TriggerSpec{Kind: TriggerOnce, At: &future, EveryMs: MinInterval.Milliseconds()},
			wantErr: ErrTriggerFieldsConflict,
		},
		{name: "UnknownKind", spec: TriggerSpec{Kind: "weekly"}, wantErr: ErrTriggerKindUnknown},
		{name: "EmptyKind", spec: TriggerSpec{}, wantErr: ErrTriggerKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()

			if tt.wantErr == nil {
				assert.NoError(t, err, "Spec must validate")
			} else {
				assert.ErrorIs(t, err, tt.wantErr, "Validation must fail with the expected sentinel")
			}
		})
	}
}

func TestTriggerNext(t *testing.T) {
	t.Run("CronEvaluatesInTimezone", func(t *testing.T) {
		shanghai := mustLocation(t, "Asia/Shanghai")
		spec := Expr("0 2 * * *", "Asia/Shanghai")

		after := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC) // 18:00 in Shanghai

		next, ok := spec.Next(after, time.Time{})
		require.True(t, ok, "A daily expression always yields a next fire")
		assert.Equal(t,
			time.Date(2026, 7, 18, 2, 0, 0, 0, shanghai).UTC(), next.UTC(),
			"The 02:00 Shanghai occurrence must resolve against the trigger's zone, not the input's")
	})

	t.Run("CronStrictlyAfter", func(t *testing.T) {
		spec := Expr("0 * * * *", "UTC")
		onTheHour := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)

		next, ok := spec.Next(onTheHour, time.Time{})
		require.True(t, ok, "An hourly expression always yields a next fire")
		assert.Equal(t, onTheHour.Add(time.Hour), next.UTC(),
			"An instant equal to an occurrence must yield the following one")
	})

	t.Run("EmptyTimezoneIsIndependentOfProcessLocal", func(t *testing.T) {
		previousLocal := time.Local
		defer func() { time.Local = previousLocal }()

		spec := TriggerSpec{Kind: TriggerCron, Expr: "0 2 * * *"}
		after := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)

		time.Local = time.FixedZone("UTCPlusEight", 8*60*60)
		first, ok := spec.Next(after, time.Time{})
		require.True(t, ok, "The UTC-defaulted expression should yield a next fire")

		time.Local = time.FixedZone("UTCMinusFive", -5*60*60)
		second, ok := spec.Next(after, time.Time{})
		require.True(t, ok, "Changing process-local time should not disable the expression")

		assert.Equal(t, first, second, "The default timezone must produce one cluster-wide instant")
		assert.Equal(t, time.Date(2026, 7, 17, 2, 0, 0, 0, time.UTC), first,
			"The default timezone should evaluate the expression in UTC")
	})

	t.Run("CronSpringForwardGap", func(t *testing.T) {
		newYork := mustLocation(t, "America/New_York")
		spec := Expr("30 2 * * *", "America/New_York")

		// 2026-03-08: 02:00-03:00 local does not exist in America/New_York.
		before := time.Date(2026, 3, 8, 1, 0, 0, 0, newYork)

		next, ok := spec.Next(before, time.Time{})
		require.True(t, ok, "The gap day still yields a next fire")
		assert.True(t, next.After(before), "Next fire must land after the DST gap")
		assert.True(t,
			next.Before(time.Date(2026, 3, 9, 12, 0, 0, 0, newYork)),
			"Next fire must not overshoot past the following morning")
	})

	t.Run("IntervalAnchoredPhase", func(t *testing.T) {
		anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(5 * time.Minute)

		next, ok := spec.Next(anchor.Add(7*time.Minute), anchor)
		require.True(t, ok, "An interval trigger always yields a next fire")
		assert.Equal(t, anchor.Add(10*time.Minute), next,
			"The phase must stay anchored regardless of the query instant")
	})

	t.Run("IntervalStrictlyAfter", func(t *testing.T) {
		anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(5 * time.Minute)

		next, ok := spec.Next(anchor.Add(10*time.Minute), anchor)
		require.True(t, ok, "An interval trigger always yields a next fire")
		assert.Equal(t, anchor.Add(15*time.Minute), next,
			"An instant on an occurrence must yield the following one")
	})

	t.Run("IntervalBeforeAnchor", func(t *testing.T) {
		anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(time.Hour)

		next, ok := spec.Next(anchor.Add(-30*time.Minute), anchor)
		require.True(t, ok, "An interval trigger always yields a next fire")
		assert.Equal(t, anchor, next, "Instants before the anchor fire first at the anchor")
	})

	t.Run("IntervalZeroAnchor", func(t *testing.T) {
		after := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(time.Minute)

		next, ok := spec.Next(after, time.Time{})
		require.True(t, ok, "An interval trigger always yields a next fire")
		assert.Equal(t, after.Add(time.Minute), next,
			"A zero anchor must degrade to one interval after the query instant")
	})

	t.Run("LargeIntervalAcrossLongGap", func(t *testing.T) {
		anchor := time.Date(1776, 1, 1, 0, 0, 0, 0, time.UTC)
		after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		every := 200 * 365 * 24 * time.Hour
		spec := Every(every)

		next, ok := spec.Next(after, anchor)
		require.True(t, ok, "A representable large interval should yield a next fire")

		expected := after.Add(every - after.Sub(anchor)%every)
		assert.Equal(t, expected, next, "The next fire must not wrap when multiple large periods exceed Duration")
		assert.True(t, next.After(after), "An interval fire must remain strictly after the query instant")
	})

	t.Run("OnceFutureThenSpent", func(t *testing.T) {
		at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		spec := Once(at)

		next, ok := spec.Next(at.Add(-time.Hour), time.Time{})
		require.True(t, ok, "A future one-shot must yield its fire time")
		assert.Equal(t, at, next, "The one-shot fire time must be returned verbatim")

		_, ok = spec.Next(at, time.Time{})
		assert.False(t, ok, "An instant equal to the fire time must yield no further occurrence")

		_, ok = spec.Next(at.Add(time.Hour), time.Time{})
		assert.False(t, ok, "A spent one-shot must yield no further occurrence")
	})

	t.Run("InvalidSpecYieldsNothing", func(t *testing.T) {
		_, ok := TriggerSpec{Kind: TriggerCron, Expr: "garbage"}.Next(time.Now(), time.Time{})
		assert.False(t, ok, "An unparsable expression must yield no occurrence")

		_, ok = TriggerSpec{Kind: "weekly"}.Next(time.Now(), time.Time{})
		assert.False(t, ok, "An unknown kind must yield no occurrence")

		_, ok = Expr("0 2 * * *", "Local").Next(time.Now(), time.Time{})
		assert.False(t, ok, "A process-local timezone must yield no occurrence")

		_, ok = TriggerSpec{Kind: TriggerInterval, EveryMs: time.Minute.Milliseconds(), Expr: "0 2 * * *"}.
			Next(time.Now(), time.Time{})
		assert.False(t, ok, "A trigger with conflicting fields must yield no occurrence")

		_, ok = TriggerSpec{Kind: TriggerInterval, EveryMs: maxDurationMilliseconds + 1}.
			Next(time.Now(), time.Time{})
		assert.False(t, ok, "An overflowing interval must yield no occurrence")
	})
}

func TestTriggerOccurrences(t *testing.T) {
	anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)

	t.Run("IntervalGap", func(t *testing.T) {
		spec := Every(time.Minute)

		count := spec.Occurrences(anchor, anchor.Add(10*time.Minute), anchor, 100)
		assert.Equal(t, 10, count, "A 10-minute gap holds ten per-minute occurrences")
	})

	t.Run("CapHonored", func(t *testing.T) {
		spec := Every(time.Second)

		count := spec.Occurrences(anchor, anchor.Add(24*time.Hour), anchor, 500)
		assert.Equal(t, 500, count, "The cap must bound pathological gaps")
	})

	t.Run("CronWindow", func(t *testing.T) {
		spec := Expr("* * * * *", "UTC")

		count := spec.Occurrences(anchor, anchor.Add(5*time.Minute), time.Time{}, 100)
		assert.Equal(t, 5, count, "A 5-minute window holds five per-minute fires")
	})

	t.Run("OnceInsideAndOutside", func(t *testing.T) {
		at := anchor.Add(30 * time.Minute)
		spec := Once(at)

		assert.Equal(t, 1, spec.Occurrences(anchor, anchor.Add(time.Hour), time.Time{}, 10),
			"A one-shot inside the window counts once")
		assert.Equal(t, 0, spec.Occurrences(at, anchor.Add(time.Hour), time.Time{}, 10),
			"The window is open at its start; an occurrence on it does not count")
		assert.Equal(t, 0, spec.Occurrences(anchor, at.Add(-time.Minute), time.Time{}, 10),
			"A one-shot after the window counts zero")
	})

	t.Run("DegenerateInputs", func(t *testing.T) {
		spec := Every(time.Minute)

		assert.Equal(t, 0, spec.Occurrences(anchor, anchor, anchor, 10),
			"An empty window holds no occurrences")
		assert.Equal(t, 0, spec.Occurrences(anchor, anchor.Add(-time.Hour), anchor, 10),
			"An inverted window holds no occurrences")
		assert.Equal(t, 0, spec.Occurrences(anchor, anchor.Add(time.Hour), anchor, 0),
			"A zero cap yields zero")
	})
}
