package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func decisionSchedule(due time.Time, policy cron.MisfirePolicy) *cron.Schedule {
	schedule := scheduleFixture("orders.sync", "orders.sync", due)
	schedule.MisfirePolicy = policy

	return schedule
}

func TestDecide(t *testing.T) {
	due := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	threshold := time.Minute

	t.Run("OnTimeFire", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)

		decision := decide(schedule, due.Add(2*time.Second), threshold)

		assert.True(t, decision.fire, "an on-time occurrence must fire")
		assert.Equal(t, due, decision.scheduledAt, "the fire must carry its logical time")
		assert.Zero(t, decision.missed, "nothing is missed on time")
		require.NotNil(t, decision.next, "an interval trigger always yields a next fire")
		assert.Equal(t, due.Add(time.Minute), *decision.next, "the next fire advances one interval from the due time")
	})

	t.Run("LatenessAtThresholdStillFires", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireSkip)

		decision := decide(schedule, due.Add(threshold), threshold)

		assert.True(t, decision.fire, "lateness exactly at the threshold is not a misfire")
		assert.Zero(t, decision.missed, "no occurrence is missed at the threshold")
	})

	t.Run("MisfireFireNowCatchesUpOnce", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)

		// 5m30s late on a per-minute schedule: due + 5 further occurrences
		// are overdue.
		now := due.Add(5*time.Minute + 30*time.Second)
		decision := decide(schedule, now, threshold)

		assert.True(t, decision.fire, "fire_now must run one catch-up")
		assert.Equal(t, due, decision.scheduledAt, "the catch-up runs the oldest due occurrence")
		assert.Equal(t, 5, decision.missed, "the remaining overdue occurrences are missed")
		assert.Equal(t, due.Add(time.Minute), decision.missedFrom, "the missed row starts at the first skipped occurrence")
		require.NotNil(t, decision.next, "the schedule must advance")
		assert.Equal(t, due.Add(6*time.Minute), *decision.next, "the next fire is strictly after now")
	})

	t.Run("MisfireSkipAccountsEverything", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireSkip)

		now := due.Add(5*time.Minute + 30*time.Second)
		decision := decide(schedule, now, threshold)

		assert.False(t, decision.fire, "skip must not run a catch-up")
		assert.Equal(t, 6, decision.missed, "the due occurrence and every overdue one are missed")
		assert.Equal(t, due, decision.missedFrom, "the missed row starts at the due occurrence")
		require.NotNil(t, decision.next, "the schedule must advance")
		assert.Equal(t, due.Add(6*time.Minute), *decision.next, "the next fire is strictly after now")
	})

	t.Run("OneShotSpendsItself", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)
		schedule.Kind = cron.TriggerOnce
		schedule.EveryMs = 0
		fireAt := timex.DateTime(due)
		schedule.FireAt = &fireAt

		decision := decide(schedule, due.Add(time.Second), threshold)

		assert.True(t, decision.fire, "the one-shot must fire")
		assert.Nil(t, decision.next, "a fired one-shot yields no further occurrence")
	})

	t.Run("WindowEndStopsAdvancing", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireFireNow)
		ends := timex.DateTime(due.Add(30 * time.Second))
		schedule.EndsAt = &ends

		decision := decide(schedule, due.Add(2*time.Second), threshold)

		assert.True(t, decision.fire, "the in-window occurrence must fire")
		assert.Nil(t, decision.next, "no next fire exists past the window end")
	})

	t.Run("MisfireBeyondWindowEndCountsOnlyInWindow", func(t *testing.T) {
		schedule := decisionSchedule(due, cron.MisfireSkip)
		ends := timex.DateTime(due.Add(2 * time.Minute))
		schedule.EndsAt = &ends

		decision := decide(schedule, due.Add(10*time.Minute), threshold)

		assert.False(t, decision.fire, "skip must not fire")
		assert.Equal(t, 3, decision.missed, "only the due occurrence and the two in-window ones are missed")
		assert.Nil(t, decision.next, "the window is over")
	})
}

func TestNextFire(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)

	t.Run("StartsAtIsAValidFirstFire", func(t *testing.T) {
		schedule := scheduleFixture("windowed", "job", base)
		starts := timex.DateTime(base.Add(time.Hour))
		schedule.StartsAt = &starts

		next, ok := nextFire(schedule, base)
		require.True(t, ok, "a future window must yield a fire")
		assert.Equal(t, base.Add(time.Hour), next, "the interval anchors on the window start, firing exactly there")
	})

	t.Run("EndsAtCutsOff", func(t *testing.T) {
		schedule := scheduleFixture("windowed", "job", base)
		ends := timex.DateTime(base.Add(30 * time.Second))
		schedule.EndsAt = &ends

		_, ok := nextFire(schedule, base)
		assert.False(t, ok, "no occurrence fits inside a sub-interval window")
	})

	t.Run("AnchorKeepsPhase", func(t *testing.T) {
		schedule := scheduleFixture("anchored", "job", base)

		next, ok := nextFire(schedule, base.Add(90*time.Second))
		require.True(t, ok, "an interval trigger always yields a fire")

		anchor := schedule.CreatedAt.Unwrap()
		phase := next.Sub(anchor) % time.Minute
		assert.Zero(t, phase, "fires must stay on the anchor's phase grid")
	})
}
