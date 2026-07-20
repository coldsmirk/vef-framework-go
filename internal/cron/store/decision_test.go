package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
)

func decisionSchedule(due time.Time, policy cron.MisfirePolicy) *cron.Schedule {
	schedule := scheduleFixture("orders.sync", "orders.sync", due)
	schedule.MisfirePolicy = policy

	return schedule
}

func TestDecide(t *testing.T) {
	due := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	threshold := time.Minute

	t.Run("OnTimeFire", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)

		decision := decide(schedule, due.Add(2*time.Second), threshold)

		assert.True(t, decision.fire, "An on-time occurrence must fire")
		assert.Equal(t, due, decision.scheduledAt, "The fire must carry its logical time")
		assert.Zero(t, decision.missed, "Nothing is missed on time")
		require.NotNil(t, decision.next, "An interval trigger always yields a next fire")
		assert.Equal(t, due.Add(time.Minute), *decision.next, "The next fire advances one interval from the due time")
	})

	t.Run("LatenessAtThresholdStillFires", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireSkip)

		decision := decide(schedule, due.Add(threshold), threshold)

		assert.True(t, decision.fire, "Lateness exactly at the threshold is not a misfire")
		assert.Zero(t, decision.missed, "No occurrence is missed at the threshold")
	})

	t.Run("MisfireFireNowCatchesUpOnce", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)

		// 5m30s late on a per-minute schedule: due + 5 further occurrences
		// are overdue.
		now := due.Add(5*time.Minute + 30*time.Second)
		decision := decide(schedule, now, threshold)

		assert.True(t, decision.fire, "Fire_now must run one catch-up")
		assert.Equal(t, due, decision.scheduledAt, "The catch-up runs the oldest due occurrence")
		assert.Equal(t, 5, decision.missed, "The remaining overdue occurrences are missed")
		assert.Equal(t, due.Add(time.Minute), decision.missedFrom, "The missed row starts at the first skipped occurrence")
		require.NotNil(t, decision.next, "The schedule must advance")
		assert.Equal(t, due.Add(6*time.Minute), *decision.next, "The next fire is strictly after now")
	})

	t.Run("MisfireSkipAccountsEverything", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireSkip)

		now := due.Add(5*time.Minute + 30*time.Second)
		decision := decide(schedule, now, threshold)

		assert.False(t, decision.fire, "Skip must not run a catch-up")
		assert.Equal(t, 6, decision.missed, "The due occurrence and every overdue one are missed")
		assert.Equal(t, due, decision.missedFrom, "The missed row starts at the due occurrence")
		require.NotNil(t, decision.next, "The schedule must advance")
		assert.Equal(t, due.Add(6*time.Minute), *decision.next, "The next fire is strictly after now")
	})

	t.Run("ZonedCronAdvancesTheInstant", func(t *testing.T) {
		// An hourly expression evaluated in a zone behind the node must still
		// advance the persisted instant; otherwise every tick reclaims it.
		gmt12, err := time.LoadLocation("Etc/GMT+12")
		require.NoError(t, err, "The fixed-offset zone must load")

		schedule := decisionSchedule(due, cron.MisfireFireNow)
		schedule.Kind = cron.TriggerCron
		schedule.Expr = "0 * * * *"
		schedule.Timezone = "Etc/GMT+12"
		schedule.EveryMs = 0

		decision := decide(schedule, due.Add(2*time.Second), threshold)

		require.True(t, decision.fire, "An on-time occurrence must fire")
		require.NotNil(t, decision.next, "An hourly expression always yields a next fire")
		assert.True(t, decision.next.After(due), "The next fire's instant must be strictly after the due one")
		assert.LessOrEqual(t, decision.next.Sub(due), time.Hour, "An hourly cadence advances at most one hour")
		assert.Zero(t, decision.next.In(gmt12).Minute(), "The instant must sit on the trigger zone's hour boundary")
		assert.Greater(t, decision.next.UnixMilli(), due.UnixMilli(),
			"The authoritative epoch must advance past the due instant")
	})

	t.Run("OneShotSpendsItself", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)
		schedule.Kind = cron.TriggerOnce
		schedule.EveryMs = 0
		schedule.FireAtUnixMs = unixMillisPtr(due)

		decision := decide(schedule, due.Add(time.Second), threshold)

		assert.True(t, decision.fire, "The one-shot must fire")
		assert.Nil(t, decision.next, "A fired one-shot yields no further occurrence")
	})

	t.Run("WindowEndStopsAdvancing", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)
		schedule.EndsAtUnixMs = unixMillisPtr(due.Add(30 * time.Second))

		decision := decide(schedule, due.Add(2*time.Second), threshold)

		assert.True(t, decision.fire, "The in-window occurrence must fire")
		assert.Nil(t, decision.next, "No next fire exists past the window end")
	})

	t.Run("MisfireBeyondWindowEndCountsOnlyInWindow", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireSkip)
		schedule.EndsAtUnixMs = unixMillisPtr(due.Add(2 * time.Minute))

		decision := decide(schedule, due.Add(10*time.Minute), threshold)

		assert.False(t, decision.fire, "Skip must not fire")
		assert.Equal(t, 3, decision.missed, "Only the due occurrence and the two in-window ones are missed")
		assert.Nil(t, decision.next, "The window is over")
	})
}

func TestNextFire(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)

	t.Run("StartsAtIsAValidFirstFire", func(t *testing.T) {
		schedule := scheduleFixture("windowed", "job", base)
		schedule.StartsAtUnixMs = unixMillisPtr(base.Add(time.Hour))

		next, ok := nextFire(schedule, base)
		require.True(t, ok, "A future window must yield a fire")
		assert.Equal(t, base.Add(time.Hour), next, "The interval anchors on the window start, firing exactly there")
	})

	t.Run("EndsAtCutsOff", func(t *testing.T) {
		schedule := scheduleFixture("windowed", "job", base)
		schedule.EndsAtUnixMs = unixMillisPtr(base.Add(30 * time.Second))

		_, ok := nextFire(schedule, base)
		assert.False(t, ok, "No occurrence fits inside a sub-interval window")
	})

	t.Run("AnchorKeepsPhase", func(t *testing.T) {
		schedule := scheduleFixture("anchored", "job", base)

		next, ok := nextFire(schedule, base.Add(90*time.Second))
		require.True(t, ok, "An interval trigger always yields a fire")

		anchor := unixTime(schedule.AnchorAtUnixMs)
		phase := next.Sub(anchor) % time.Minute
		assert.Zero(t, phase, "Fires must stay on the anchor's phase grid")
	})

	t.Run("ZonedCronKeepsTheTriggerInstant", func(t *testing.T) {
		schedule := scheduleFixture("zoned", "job", base)
		schedule.Kind = cron.TriggerCron
		schedule.Expr = "0 3 * * *"
		schedule.Timezone = "UTC"
		schedule.EveryMs = 0

		probe := base
		next, ok := nextFire(schedule, probe)
		require.True(t, ok, "A daily expression always yields a fire")

		assert.Same(t, time.UTC, next.Location(),
			"The trigger's real instant must not be relabeled to the process-local zone for persistence")
		assert.Equal(t, 3, next.Hour(), "The instant must stay correct in the trigger's zone")
		assert.True(t, next.After(probe), "The fire must be strictly after the probe instant")
	})

	t.Run("DSTFallbackOccurrencesRemainDistinct", func(t *testing.T) {
		newYork, err := time.LoadLocation("America/New_York")
		require.NoError(t, err, "The fallback timezone should load")

		beforeFold := time.Date(2012, 11, 4, 0, 30, 0, 0, newYork)
		schedule := &cron.Schedule{
			Kind:           cron.TriggerCron,
			Expr:           "0 1 * * *",
			Timezone:       "America/New_York",
			IsEnabled:      true,
			AnchorAtUnixMs: beforeFold.UnixMilli(),
		}

		first, ok := nextFire(schedule, beforeFold)
		require.True(t, ok, "The cron expression should yield the EDT occurrence")
		second, ok := nextFire(schedule, first)
		require.True(t, ok, "The cron expression should yield the EST occurrence")

		assert.Equal(t, first.In(newYork).Format(time.DateTime), second.In(newYork).Format(time.DateTime),
			"Both fallback occurrences deliberately share one wall-clock label")
		assert.Equal(t, time.Hour, second.Sub(first),
			"Their authoritative instants must remain one hour apart")
		assert.NotEqual(t, first.UnixMilli(), second.UnixMilli(),
			"Fallback occurrences should keep distinct epoch values")
	})
}
