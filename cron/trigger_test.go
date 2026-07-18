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
	require.NoError(t, err, "timezone %q must load", name)

	return location
}

func TestTriggerConstructors(t *testing.T) {
	t.Run("Expr", func(t *testing.T) {
		spec := Expr("0 2 * * *", "Asia/Shanghai")

		assert.Equal(t, TriggerCron, spec.Kind, "Expr must build a cron trigger")
		assert.Equal(t, "0 2 * * *", spec.Expr, "expression must carry through")
		assert.Equal(t, "Asia/Shanghai", spec.Timezone, "timezone must carry through")
	})

	t.Run("Every", func(t *testing.T) {
		spec := Every(90 * time.Second)

		assert.Equal(t, TriggerInterval, spec.Kind, "Every must build an interval trigger")
		assert.Equal(t, int64(90_000), spec.EveryMs, "interval must convert to milliseconds")
	})

	t.Run("Once", func(t *testing.T) {
		at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		spec := Once(at)

		assert.Equal(t, TriggerOnce, spec.Kind, "Once must build a one-shot trigger")
		require.NotNil(t, spec.At, "fire time must be set")
		assert.True(t, spec.At.Equal(at), "fire time must carry through")
	})
}

func TestTriggerValidate(t *testing.T) {
	future := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		spec    TriggerSpec
		wantErr error
	}{
		{name: "five-field cron", spec: Expr("*/5 * * * *", "")},
		{name: "six-field cron with seconds", spec: Expr("30 */5 * * * *", "UTC")},
		{name: "descriptor", spec: Expr("@daily", "Asia/Shanghai")},
		{name: "blank expression", spec: Expr("", ""), wantErr: ErrTriggerExprRequired},
		{name: "garbage expression", spec: Expr("not a cron", ""), wantErr: ErrTriggerExprInvalid},
		{name: "seven fields", spec: Expr("* * * * * * *", ""), wantErr: ErrTriggerExprInvalid},
		{name: "bad timezone", spec: Expr("0 2 * * *", "Mars/Olympus"), wantErr: ErrTriggerTimezoneInvalid},
		{name: "interval at minimum", spec: Every(MinInterval)},
		{name: "interval below minimum", spec: Every(999 * time.Millisecond), wantErr: ErrTriggerIntervalTooShort},
		{name: "interval zero", spec: Every(0), wantErr: ErrTriggerIntervalTooShort},
		{name: "interval negative", spec: Every(-time.Minute), wantErr: ErrTriggerIntervalTooShort},
		{name: "once with fire time", spec: Once(future)},
		{name: "once zero fire time", spec: Once(time.Time{}), wantErr: ErrTriggerFireTimeRequired},
		{name: "once nil fire time", spec: TriggerSpec{Kind: TriggerOnce}, wantErr: ErrTriggerFireTimeRequired},
		{name: "unknown kind", spec: TriggerSpec{Kind: "weekly"}, wantErr: ErrTriggerKindUnknown},
		{name: "empty kind", spec: TriggerSpec{}, wantErr: ErrTriggerKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()

			if tt.wantErr == nil {
				assert.NoError(t, err, "spec must validate")
			} else {
				assert.ErrorIs(t, err, tt.wantErr, "validation must fail with the expected sentinel")
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
		require.True(t, ok, "a daily expression always yields a next fire")
		assert.Equal(t,
			time.Date(2026, 7, 18, 2, 0, 0, 0, shanghai).UTC(), next.UTC(),
			"02:00 Shanghai must resolve against the trigger's zone, not the input's")
	})

	t.Run("CronStrictlyAfter", func(t *testing.T) {
		spec := Expr("0 * * * *", "UTC")
		onTheHour := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)

		next, ok := spec.Next(onTheHour, time.Time{})
		require.True(t, ok, "an hourly expression always yields a next fire")
		assert.Equal(t, onTheHour.Add(time.Hour), next.UTC(),
			"an instant equal to an occurrence must yield the following one")
	})

	t.Run("CronSpringForwardGap", func(t *testing.T) {
		newYork := mustLocation(t, "America/New_York")
		spec := Expr("30 2 * * *", "America/New_York")

		// 2026-03-08: 02:00-03:00 local does not exist in America/New_York.
		before := time.Date(2026, 3, 8, 1, 0, 0, 0, newYork)

		next, ok := spec.Next(before, time.Time{})
		require.True(t, ok, "the gap day still yields a next fire")
		assert.True(t, next.After(before), "next fire must land after the DST gap")
		assert.True(t,
			next.Before(time.Date(2026, 3, 9, 12, 0, 0, 0, newYork)),
			"next fire must not overshoot past the following morning")
	})

	t.Run("IntervalAnchoredPhase", func(t *testing.T) {
		anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(5 * time.Minute)

		next, ok := spec.Next(anchor.Add(7*time.Minute), anchor)
		require.True(t, ok, "an interval trigger always yields a next fire")
		assert.Equal(t, anchor.Add(10*time.Minute), next,
			"the phase must stay anchored regardless of the query instant")
	})

	t.Run("IntervalStrictlyAfter", func(t *testing.T) {
		anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(5 * time.Minute)

		next, ok := spec.Next(anchor.Add(10*time.Minute), anchor)
		require.True(t, ok, "an interval trigger always yields a next fire")
		assert.Equal(t, anchor.Add(15*time.Minute), next,
			"an instant on an occurrence must yield the following one")
	})

	t.Run("IntervalBeforeAnchor", func(t *testing.T) {
		anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(time.Hour)

		next, ok := spec.Next(anchor.Add(-30*time.Minute), anchor)
		require.True(t, ok, "an interval trigger always yields a next fire")
		assert.Equal(t, anchor, next, "instants before the anchor fire first at the anchor")
	})

	t.Run("IntervalZeroAnchor", func(t *testing.T) {
		after := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
		spec := Every(time.Minute)

		next, ok := spec.Next(after, time.Time{})
		require.True(t, ok, "an interval trigger always yields a next fire")
		assert.Equal(t, after.Add(time.Minute), next,
			"a zero anchor must degrade to one interval after the query instant")
	})

	t.Run("OnceFutureThenSpent", func(t *testing.T) {
		at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		spec := Once(at)

		next, ok := spec.Next(at.Add(-time.Hour), time.Time{})
		require.True(t, ok, "a future one-shot must yield its fire time")
		assert.Equal(t, at, next, "the one-shot fire time must be returned verbatim")

		_, ok = spec.Next(at, time.Time{})
		assert.False(t, ok, "an instant equal to the fire time must yield no further occurrence")

		_, ok = spec.Next(at.Add(time.Hour), time.Time{})
		assert.False(t, ok, "a spent one-shot must yield no further occurrence")
	})

	t.Run("InvalidSpecYieldsNothing", func(t *testing.T) {
		_, ok := TriggerSpec{Kind: TriggerCron, Expr: "garbage"}.Next(time.Now(), time.Time{})
		assert.False(t, ok, "an unparsable expression must yield no occurrence")

		_, ok = TriggerSpec{Kind: "weekly"}.Next(time.Now(), time.Time{})
		assert.False(t, ok, "an unknown kind must yield no occurrence")
	})
}

func TestTriggerOccurrences(t *testing.T) {
	anchor := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)

	t.Run("IntervalGap", func(t *testing.T) {
		spec := Every(time.Minute)

		count := spec.Occurrences(anchor, anchor.Add(10*time.Minute), anchor, 100)
		assert.Equal(t, 10, count, "a 10-minute gap holds ten per-minute occurrences")
	})

	t.Run("CapHonored", func(t *testing.T) {
		spec := Every(time.Second)

		count := spec.Occurrences(anchor, anchor.Add(24*time.Hour), anchor, 500)
		assert.Equal(t, 500, count, "the cap must bound pathological gaps")
	})

	t.Run("CronWindow", func(t *testing.T) {
		spec := Expr("* * * * *", "UTC")

		count := spec.Occurrences(anchor, anchor.Add(5*time.Minute), time.Time{}, 100)
		assert.Equal(t, 5, count, "a 5-minute window holds five per-minute fires")
	})

	t.Run("OnceInsideAndOutside", func(t *testing.T) {
		at := anchor.Add(30 * time.Minute)
		spec := Once(at)

		assert.Equal(t, 1, spec.Occurrences(anchor, anchor.Add(time.Hour), time.Time{}, 10),
			"a one-shot inside the window counts once")
		assert.Equal(t, 0, spec.Occurrences(at, anchor.Add(time.Hour), time.Time{}, 10),
			"the window is open at its start — an occurrence on it does not count")
		assert.Equal(t, 0, spec.Occurrences(anchor, at.Add(-time.Minute), time.Time{}, 10),
			"a one-shot after the window counts zero")
	})

	t.Run("DegenerateInputs", func(t *testing.T) {
		spec := Every(time.Minute)

		assert.Equal(t, 0, spec.Occurrences(anchor, anchor, anchor, 10),
			"an empty window holds no occurrences")
		assert.Equal(t, 0, spec.Occurrences(anchor, anchor.Add(-time.Hour), anchor, 10),
			"an inverted window holds no occurrences")
		assert.Equal(t, 0, spec.Occurrences(anchor, anchor.Add(time.Hour), anchor, 0),
			"a zero cap yields zero")
	})
}
