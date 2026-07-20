package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store/migration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func newTestClaimer(db orm.DB, registry *Registry, nodeID string, now func() time.Time) *claimer {
	return &claimer{
		db:       db,
		config:   fastStoreConfig(),
		registry: registry,
		nodeID:   nodeID,
		now:      now,
	}
}

func TestClaimDue(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	t.Run("ClaimsAndAdvances", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s1", "orders.sync", base))

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		require.Len(t, claimed, 1, "One due schedule must yield one fire")

		fire := claimed[0]
		assert.Equal(t, "node-a", fire.run.NodeID, "The run must carry the claiming node")
		assert.Equal(t, cron.RunRunning, fire.run.Status, "The claimed fire is running")
		assert.True(t, fire.run.ScheduledAt.Unwrap().Equal(base), "The run carries the logical fire time")
		assert.NotEmpty(t, fire.run.ID, "The journal row must have been inserted with an id")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAt, "The schedule must advance")
		assert.True(t, after.NextFireAt.AsLocal().Equal(base.Add(time.Minute)),
			"The next fire advances one interval")
		require.NotNil(t, after.LastFireAt, "The executed fire is recorded")
		assert.True(t, after.LastFireAt.AsLocal().Equal(base), "LastFireAt carries the logical fire time")

		again, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(2*time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Re-claiming should succeed")
		assert.Empty(t, again, "An advanced schedule is no longer due")
	})

	t.Run("MisfireSkipJournalsOneMissedRow", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := scheduleFixture("s2", "orders.sync", base)
		schedule.MisfirePolicy = cron.MisfireSkip
		insertSchedule(t, db, schedule)

		now := base.Add(5*time.Minute + 30*time.Second)

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(now)).ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		assert.Empty(t, claimed, "Skip policy must not fire")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 1, "The whole gap collapses into one journal row")
		assert.Equal(t, cron.RunMissed, runs[0].Status, "The row is a missed record")
		assert.Equal(t, 6, runs[0].MissedCount, "It covers every overdue occurrence")
		require.NotNil(t, runs[0].FinishedAt, "A missed row is terminal")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAt, "The schedule must advance past the gap")
		assert.True(t, after.NextFireAt.AsLocal().After(now), "The next fire is strictly future")
		assert.Nil(t, after.LastFireAt, "Nothing executed, so LastFireAt stays unset")
	})

	t.Run("PausedGapIsJournaledAfterResume", func(t *testing.T) {
		// Pausing preserves the fire cursor, so the paused gap reaches the
		// claim as an ordinary misfire: it is caught up and accounted for
		// instead of vanishing between Pause and Resume.
		db := newStoreDB(t)
		clock := base
		manager := newTestManager(db, registry, func() time.Time { return clock })

		created, err := manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "paused",
			JobName: "orders.sync",
			Trigger: cron.Every(time.Minute),
		})
		require.NoError(t, err, "Creating the schedule should succeed")
		require.NoError(t, manager.Pause(context.Background(), "paused"), "Pausing should succeed")

		// Five minutes of paused occurrences, then the operator resumes.
		clock = base.Add(6 * time.Minute)

		require.NoError(t, manager.Resume(context.Background(), "paused"), "Resuming should succeed")

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(clock)).ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		require.Len(t, claimed, 1, "Fire_now must catch the paused gap up with one run")

		runs := loadRuns(t, db, created.ID)
		require.Len(t, runs, 2, "The claim journals the catch-up and the paused gap")
		assert.Equal(t, cron.RunRunning, runs[0].Status, "The oldest paused occurrence runs")
		assert.Equal(t, cron.RunMissed, runs[1].Status, "The rest of the paused gap is journaled as missed")
		assert.Positive(t, runs[1].MissedCount, "The missed row must count the paused occurrences")
	})

	t.Run("TriggerNowRunsEvenUnderMisfireSkip", func(t *testing.T) {
		// The regression behind a manual fire that silently did nothing: an
		// overdue skip schedule journalled the request as missed.
		db := newStoreDB(t)
		now := base.Add(time.Hour)
		manager := newTestManager(db, registry, fixedNow(now))

		fixture := scheduleFixture("overdue", "orders.sync", base)
		fixture.MisfirePolicy = cron.MisfireSkip
		schedule := insertSchedule(t, db, fixture)

		require.NoError(t, manager.TriggerNow(context.Background(), "overdue"), "Triggering should succeed")

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(now)).ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		require.Len(t, claimed, 1, "A manual fire must execute, never be journaled as missed")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 1, "The manual fire is the only journal row")
		assert.Equal(t, cron.RunRunning, runs[0].Status, "The manual fire runs")
	})

	t.Run("MisfireFireNowJournalsCatchUpAndMissed", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s3", "orders.sync", base))

		now := base.Add(5*time.Minute + 30*time.Second)

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(now)).ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		require.Len(t, claimed, 1, "Fire_now must run one catch-up")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 2, "The claim journals the catch-up and the missed gap")
		assert.Equal(t, cron.RunRunning, runs[0].Status, "The oldest due occurrence runs")
		assert.Equal(t, cron.RunMissed, runs[1].Status, "The rest of the gap is missed")
		assert.Equal(t, 5, runs[1].MissedCount, "Five further occurrences were overdue")
	})

	t.Run("ConcurrencyForbidSkips", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s4", "orders.sync", base))

		clock := fixedNow(base.Add(time.Second))
		c := newTestClaimer(db, registry, "node-a", clock)

		first, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "First claim should succeed")
		require.Len(t, first, 1, "The first occurrence fires")

		// The first run is still running when the next occurrence comes due.
		c.now = fixedNow(base.Add(time.Minute + time.Second))

		second, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Second claim should succeed")
		assert.Empty(t, second, "Forbid must suppress the overlapping fire")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 2, "The suppression is journaled")
		assert.Equal(t, cron.RunSkipped, runs[1].Status, "The overlapping occurrence is skipped")
		assert.Empty(t, runs[1].NodeID, "A skipped row never executed anywhere")
	})

	t.Run("ConcurrencyAllowOverlaps", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := scheduleFixture("s5", "orders.sync", base)
		schedule.ConcurrencyPolicy = cron.ConcurrencyAllow
		insertSchedule(t, db, schedule)

		c := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second)))

		first, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "First claim should succeed")
		require.Len(t, first, 1, "The first occurrence fires")

		c.now = fixedNow(base.Add(time.Minute + time.Second))

		second, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Second claim should succeed")
		assert.Len(t, second, 1, "Allow lets the fires overlap")
	})

	t.Run("ForeignJobsAreLeftAlone", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s6", "job.not.here", base))

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		assert.Empty(t, claimed, "A node without the handler must not claim the schedule")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAt, "The schedule must stay due for capable nodes")
		assert.True(t, after.NextFireAt.AsLocal().Equal(base), "The fire must not advance")
	})

	t.Run("OneShotDisarmsAfterFiring", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := scheduleFixture("s7", "orders.sync", base)
		schedule.Kind = cron.TriggerOnce
		schedule.EveryMs = 0
		fireAt := timex.DateTime(base)
		schedule.FireAt = &fireAt
		insertSchedule(t, db, schedule)

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		require.Len(t, claimed, 1, "The one-shot fires once")

		after := reloadSchedule(t, db, schedule.ID)
		assert.Nil(t, after.NextFireAt, "A spent one-shot has no next fire")
		assert.True(t, after.IsEnabled, "Enablement stays operator-owned")
	})
}

// TestClaimContention proves the exactly-once claim across every supported
// dialect: two nodes race for the same occurrences round after round, and
// each occurrence must be claimed by exactly one of them.
func TestClaimContention(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind),
			"Cron store migration should provision the tables")

		base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
		registry := mustRegistry(t, noopHandler("orders.sync"))

		schedule := scheduleFixture("contended", "orders.sync", base)
		schedule.EveryMs = time.Second.Milliseconds()
		schedule.ConcurrencyPolicy = cron.ConcurrencyAllow
		insertSchedule(t, env.DB, schedule)

		const rounds = 20

		var (
			mu      sync.Mutex
			claimed []cron.Run
		)

		for round := range rounds {
			now := base.Add(time.Duration(round)*time.Second + 100*time.Millisecond)

			var wg sync.WaitGroup

			for _, node := range []string{"node-a", "node-b"} {
				wg.Go(func() {
					fires, err := newTestClaimer(env.DB, registry, node, fixedNow(now)).
						ClaimDue(env.Ctx, 10)
					assert.NoError(t, err, "Contended claiming must not error")

					mu.Lock()
					defer mu.Unlock()

					for _, fire := range fires {
						claimed = append(claimed, *fire.run)
					}
				})
			}

			wg.Wait()
		}

		require.Len(t, claimed, rounds, "Every occurrence must be claimed exactly once")

		seen := make(map[string]string, rounds)
		for _, run := range claimed {
			key := run.ScheduledAt.String()
			previous, duplicated := seen[key]
			require.False(t, duplicated,
				"occurrence %s was claimed by both %s and %s", key, previous, run.NodeID)
			seen[key] = run.NodeID
		}

		runs := loadRuns(t, env.DB, schedule.ID)
		assert.Len(t, runs, rounds, "The journal must hold exactly one row per occurrence")
	})
}
