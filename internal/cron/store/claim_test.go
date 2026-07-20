package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store/migration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func newTestClaimer(db orm.DB, registry *Registry, nodeID string, now func() time.Time) *claimer {
	config := fastStoreConfig()
	config.AbandonedAfter = 24 * time.Hour

	return &claimer{
		db:       db,
		config:   config,
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
		assert.Equal(t, base.UnixMilli(), fire.run.ScheduledAtUnixMs, "The run carries the logical fire time")
		assert.NotEmpty(t, fire.run.ID, "The journal row must have been inserted with an id")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAtUnixMs, "The schedule must advance")
		assert.Equal(t, base.Add(time.Minute).UnixMilli(), *after.NextFireAtUnixMs,
			"The next fire advances one interval")
		require.NotNil(t, after.LastFireAtUnixMs, "The executed fire is recorded")
		assert.Equal(t, base.UnixMilli(), *after.LastFireAtUnixMs, "LastFireAtUnixMs carries the logical fire time")

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
		require.NotNil(t, runs[0].FinishedAtUnixMs, "A missed row is terminal")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAtUnixMs, "The schedule must advance past the gap")
		assert.Greater(t, *after.NextFireAtUnixMs, now.UnixMilli(), "The next fire is strictly future")
		assert.Nil(t, after.LastFireAtUnixMs, "Nothing executed, so LastFireAtUnixMs stays unset")
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

	t.Run("TriggerNowPreservesAnOverdueSkipGap", func(t *testing.T) {
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
		require.Len(t, runs, 2, "The regular gap and manual request should be journaled independently")
		assert.Equal(t, cron.RunMissed, runs[0].Status, "The overdue regular occurrence should be missed")
		assert.Equal(t, cron.RunRunning, runs[1].Status, "The manual request should run")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID), "The manual request should be consumed")
	})

	t.Run("OrdersARegularOccurrenceBeforeANewerManualRequest", func(t *testing.T) {
		db := newStoreDB(t)
		now := base.Add(time.Second)
		manager := newTestManager(db, registry, fixedNow(now))

		schedule := scheduleFixture("regular-and-manual", "orders.sync", base)
		schedule.ConcurrencyPolicy = cron.ConcurrencyAllow
		insertSchedule(t, db, schedule)
		require.NoError(t, manager.TriggerNow(context.Background(), schedule.Name),
			"Queuing the manual request should succeed")

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(now)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming regular and manual fires should succeed")
		require.Len(t, claimed, 2, "Both independent occurrences should dispatch")
		assert.Equal(t, base.UnixMilli(), claimed[0].run.ScheduledAtUnixMs,
			"The older regular occurrence should dispatch first")
		assert.Equal(t, now.UnixMilli(), claimed[1].run.ScheduledAtUnixMs,
			"The newer manual request should dispatch second")
	})

	t.Run("OrdersMultipleManualRequestsByLogicalTime", func(t *testing.T) {
		db := newStoreDB(t)
		clock := base
		manager := newTestManager(db, registry, func() time.Time { return clock })

		schedule := scheduleFixture("manual-order", "orders.sync", base.Add(time.Hour))
		schedule.ConcurrencyPolicy = cron.ConcurrencyAllow
		insertSchedule(t, db, schedule)

		require.NoError(t, manager.TriggerNow(context.Background(), schedule.Name),
			"Queuing the first manual request should succeed")

		clock = base.Add(time.Second)

		require.NoError(t, manager.TriggerNow(context.Background(), schedule.Name),
			"Queuing the second manual request should succeed")

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(clock)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming both manual requests should succeed")
		require.Len(t, claimed, 2, "Every manual request should dispatch independently")
		assert.Equal(t, base.UnixMilli(), claimed[0].run.ScheduledAtUnixMs,
			"The first logical request should dispatch first")
		assert.Equal(t, clock.UnixMilli(), claimed[1].run.ScheduledAtUnixMs,
			"The second logical request should dispatch second")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID), "Both manual requests should be consumed")
	})

	t.Run("KeepsBlockedRecoveryRequestsPending", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("blocked-recovery", "orders.sync", base.Add(time.Hour)))
		active := insertRunningRun(t, db, schedule, base.Add(-time.Minute), base)
		request := insertFireRequest(t, db, &fireRequest{
			ScheduleID:        schedule.ID,
			Kind:              fireRequestRecovery,
			ScheduledAtUnixMs: base.Add(-2 * time.Minute).UnixMilli(),
			SourceRunID:       "orphan-blocked",
		})

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Blocked recovery claiming should not error")
		assert.Empty(t, claimed, "ConcurrencyForbid should defer the recovery")
		pending := loadFireRequests(t, db, schedule.ID)
		require.Len(t, pending, 1, "The blocked recovery should stay pending")
		assert.Equal(t, request.ID, pending[0].ID, "The same request should remain durable")

		_, err = db.NewUpdate().
			Model((*cron.Run)(nil)).
			Set("status", cron.RunSucceeded).
			Set("finished_at_unix_ms", base.UnixMilli()).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(active.ID) }).
			Exec(context.Background())
		require.NoError(t, err, "Finishing the active run should succeed")

		claimed, err = newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Retrying recovery claiming should succeed")
		require.Len(t, claimed, 1, "The recovery should dispatch after the overlap ends")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID), "The dispatched recovery should be consumed")
	})

	t.Run("ConsumesABlockedManualRequestAsSkipped", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("blocked-manual", "orders.sync", base.Add(time.Hour)))
		insertRunningRun(t, db, schedule, base.Add(-time.Minute), base)
		manager := newTestManager(db, registry, fixedNow(base))

		require.NoError(t, manager.TriggerNow(context.Background(), schedule.Name),
			"Queuing the manual request should succeed")
		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming the blocked manual request should succeed")
		assert.Empty(t, claimed, "ConcurrencyForbid should not dispatch the manual request")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID), "The skipped manual request should be consumed")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 2, "The active and skipped runs should both stay journaled")
		assert.Equal(t, cron.RunSkipped, runs[1].Status, "The manual request should respect concurrency")
	})

	t.Run("SerializesRecoveryRequestsWithinOneBatch", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("serial-recovery", "orders.sync", base.Add(time.Hour)))

		first := insertFireRequest(t, db, &fireRequest{
			ScheduleID:        schedule.ID,
			Kind:              fireRequestRecovery,
			ScheduledAtUnixMs: base.Add(-2 * time.Minute).UnixMilli(),
			SourceRunID:       "orphan-first",
		})
		second := insertFireRequest(t, db, &fireRequest{
			ScheduleID:        schedule.ID,
			Kind:              fireRequestRecovery,
			ScheduledAtUnixMs: base.Add(-time.Minute).UnixMilli(),
			SourceRunID:       "orphan-second",
		})

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming serial recoveries should succeed")
		require.Len(t, claimed, 1, "ConcurrencyForbid should dispatch only one recovery")
		assert.Equal(t, first.ScheduledAtUnixMs, claimed[0].run.ScheduledAtUnixMs,
			"The older recovery should dispatch first")
		pending := loadFireRequests(t, db, schedule.ID)
		require.Len(t, pending, 1, "The second recovery should remain pending")
		assert.Equal(t, second.ID, pending[0].ID, "The newer recovery should remain queued")
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

	t.Run("TakesOverAStaleRunBeforeApplyingConcurrency", func(t *testing.T) {
		db := newStoreDB(t)
		registry := mustRegistry(t, noopHandler("orders.sync"))
		now := base.Add(time.Second)
		schedule := insertSchedule(t, db, scheduleFixture("stale-gate", "orders.sync", base))
		orphan := insertRunningRun(t, db, schedule, base.Add(-time.Hour), base.Add(-time.Hour))

		claimer := newTestClaimer(db, registry, "node-live", fixedNow(now))
		claimer.config.AbandonedAfter = 50 * time.Millisecond
		claimed, err := claimer.ClaimDue(context.Background(), 1)
		require.NoError(t, err, "Claiming behind a stale run should succeed")
		require.Len(t, claimed, 1, "The due occurrence should execute after stale takeover")
		assert.Equal(t, base.UnixMilli(), claimed[0].run.ScheduledAtUnixMs,
			"The stale run must not suppress the current occurrence")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 2, "The abandoned and replacement runs should both remain journaled")
		statuses := []cron.RunStatus{runs[0].Status, runs[1].Status}
		assert.Contains(t, statuses, cron.RunAbandoned, "The stale run should be finalized as abandoned")
		assert.Contains(t, statuses, cron.RunRunning, "The due occurrence should own a running row")
		assert.NotContains(t, statuses, cron.RunSkipped, "Stale ownership must never produce a skipped occurrence")
		assert.Equal(t, orphan.ID, runs[0].ID, "The original orphan should retain its journal identity")
	})

	t.Run("PrioritizesARecoveryCreatedDuringClaim", func(t *testing.T) {
		db := newStoreDB(t)
		registry := mustRegistry(t, noopHandler("orders.sync"))
		now := base.Add(time.Second)
		schedule := scheduleFixture("recover-before-regular", "orders.sync", base)
		schedule.Recover = true
		insertSchedule(t, db, schedule)
		orphan := insertRunningRun(t, db, schedule, base.Add(-time.Hour), base.Add(-time.Hour))

		claimer := newTestClaimer(db, registry, "node-live", fixedNow(now))
		claimer.config.AbandonedAfter = 50 * time.Millisecond
		claimed, err := claimer.ClaimDue(context.Background(), 1)
		require.NoError(t, err, "Claiming should atomically queue and take the stale recovery")
		require.Len(t, claimed, 1, "The oldest recovery should fill the only claim slot")
		assert.Equal(t, orphan.ScheduledAtUnixMs, claimed[0].run.ScheduledAtUnixMs,
			"Recovery should run before the newer regular cursor")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAtUnixMs, "The regular occurrence should remain pending")
		assert.Equal(t, base.UnixMilli(), *after.NextFireAtUnixMs,
			"Claiming the recovery must not advance the regular cursor")
	})

	t.Run("ForeignJobsAreLeftAlone", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s6", "job.not.here", base))

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		assert.Empty(t, claimed, "A node without the handler must not claim the schedule")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAtUnixMs, "The schedule must stay due for capable nodes")
		assert.Equal(t, base.UnixMilli(), *after.NextFireAtUnixMs, "The fire must not advance")
	})

	t.Run("OneShotDisarmsAfterFiring", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := scheduleFixture("s7", "orders.sync", base)
		schedule.Kind = cron.TriggerOnce
		schedule.EveryMs = 0
		schedule.FireAtUnixMs = unixMillisPtr(base)
		insertSchedule(t, db, schedule)

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming should succeed")
		require.Len(t, claimed, 1, "The one-shot fires once")

		after := reloadSchedule(t, db, schedule.ID)
		assert.Nil(t, after.NextFireAtUnixMs, "A spent one-shot has no next fire")
		assert.True(t, after.IsEnabled, "Enablement stays operator-owned")
	})

	t.Run("ClaimsBothOccurrencesOfADSTFallback", func(t *testing.T) {
		db := newStoreDB(t)
		first := time.Date(2012, time.November, 4, 5, 30, 0, 0, time.UTC)
		second := first.Add(time.Hour)
		created := timex.DateTime(first.Add(-24 * time.Hour).In(time.Local))
		schedule := &cron.Schedule{
			Name:              "fallback",
			JobName:           "orders.sync",
			Kind:              cron.TriggerCron,
			Expr:              "30 1 * * *",
			Timezone:          "America/New_York",
			MisfirePolicy:     cron.MisfireFireNow,
			ConcurrencyPolicy: cron.ConcurrencyAllow,
			IsEnabled:         true,
			AnchorAtUnixMs:    first.Add(-24 * time.Hour).UnixMilli(),
			NextFireAtUnixMs:  unixMillisPtr(first),
		}
		schedule.CreatedAt = created
		schedule.UpdatedAt = created
		insertSchedule(t, db, schedule)

		claimer := newTestClaimer(db, registry, "node-a", fixedNow(first.Add(time.Second)))
		claimed, err := claimer.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "The first fallback occurrence should claim")
		require.Len(t, claimed, 1, "The EDT occurrence should dispatch once")
		assert.Equal(t, first.UnixMilli(), claimed[0].run.ScheduledAtUnixMs,
			"The first claim should journal the EDT instant")

		afterFirst := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, afterFirst.NextFireAtUnixMs, "The EST occurrence should remain armed")
		assert.Equal(t, second.UnixMilli(), *afterFirst.NextFireAtUnixMs,
			"The repeated wall-clock label must advance to its distinct absolute instant")

		claimer.now = fixedNow(second.Add(time.Second))
		claimed, err = claimer.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "The second fallback occurrence should claim")
		require.Len(t, claimed, 1, "The EST occurrence should dispatch once")
		assert.Equal(t, second.UnixMilli(), claimed[0].run.ScheduledAtUnixMs,
			"The second claim should journal the EST instant")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 2, "Both repeated wall times must remain distinct journal rows")
		assert.Equal(t, []int64{first.UnixMilli(), second.UnixMilli()},
			[]int64{runs[0].ScheduledAtUnixMs, runs[1].ScheduledAtUnixMs},
			"The journal should preserve both fallback instants in order")
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
			claimCtx, cancel := context.WithTimeout(env.Ctx, 5*time.Second)

			var wg sync.WaitGroup

			claimErrors := make(chan error, 2)

			for _, node := range []string{"node-a", "node-b"} {
				wg.Go(func() {
					fires, err := newTestClaimer(env.DB, registry, node, fixedNow(now)).
						ClaimDue(claimCtx, 10)
					claimErrors <- err

					if err != nil {
						return
					}

					mu.Lock()
					defer mu.Unlock()

					for _, fire := range fires {
						claimed = append(claimed, *fire.run)
					}
				})
			}

			wg.Wait()
			cancel()
			close(claimErrors)

			for err := range claimErrors {
				require.NoError(t, err, "Contended claiming must not error")
			}
		}

		require.Len(t, claimed, rounds, "Every occurrence must be claimed exactly once")

		seen := make(map[int64]string, rounds)
		for _, run := range claimed {
			key := run.ScheduledAtUnixMs
			previous, duplicated := seen[key]
			require.False(t, duplicated,
				"Occurrence %d should not be claimed by both %s and %s", key, previous, run.NodeID)
			seen[key] = run.NodeID
		}

		runs := loadRuns(t, env.DB, schedule.ID)
		assert.Len(t, runs, rounds, "The journal must hold exactly one row per occurrence")
	})
}

func TestFireRequestClaimSkipsBlockedRecoveryHead(t *testing.T) {
	base := time.Date(2026, 7, 20, 10, 0, 0, 0, time.Local)
	db := newStoreDB(t)
	registry := mustRegistry(t, noopHandler("orders.sync"))
	blocked := insertSchedule(t, db, scheduleFixture("blocked-oldest", "orders.sync", base.Add(time.Hour)))
	ready := insertSchedule(t, db, scheduleFixture("ready-newer", "orders.sync", base.Add(time.Hour)))
	insertRunningRun(t, db, blocked, base.Add(-time.Minute), base)
	blockedRequest := insertFireRequest(t, db, &fireRequest{
		ScheduleID:        blocked.ID,
		Kind:              fireRequestRecovery,
		ScheduledAtUnixMs: base.Add(-2 * time.Minute).UnixMilli(),
		SourceRunID:       "orphan-fairness",
	})
	insertFireRequest(t, db, &fireRequest{
		ScheduleID:        ready.ID,
		Kind:              fireRequestManual,
		ScheduledAtUnixMs: base.Add(-time.Minute).UnixMilli(),
	})

	claimed, err := newTestClaimer(db, registry, "node-ready", fixedNow(base)).
		ClaimDue(context.Background(), 1)
	require.NoError(t, err, "The blocked recovery should not stop another schedule from claiming")
	require.Len(t, claimed, 1, "The runnable manual request should fill the only claim slot")
	assert.Equal(t, ready.ID, claimed[0].schedule.ID, "The unblocked schedule should dispatch")

	pending := loadFireRequests(t, db, blocked.ID)
	require.Len(t, pending, 1, "The blocked recovery should remain durable")
	assert.Equal(t, blockedRequest.ID, pending[0].ID, "The original recovery request should remain queued")
	assert.Empty(t, loadFireRequests(t, db, ready.ID), "The claimed manual request should be consumed")
}

func TestFireRequestClaimRefillsLockedScheduleWindow(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind == config.SQLite {
			t.Skip("SQLite does not support row-level locking")
		}

		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind),
			"The cron store migration should provision the tables")

		base := time.Date(2026, 7, 20, 10, 0, 0, 0, time.Local)
		registry := mustRegistry(t, noopHandler("orders.sync"))
		locked := insertSchedule(t, env.DB, scheduleFixture("locked-oldest", "orders.sync", base.Add(time.Hour)))
		ready := insertSchedule(t, env.DB, scheduleFixture("ready-next", "orders.sync", base.Add(time.Hour)))
		insertFireRequest(t, env.DB, &fireRequest{
			ScheduleID:        locked.ID,
			Kind:              fireRequestManual,
			ScheduledAtUnixMs: base.Add(-2 * time.Minute).UnixMilli(),
		})
		insertFireRequest(t, env.DB, &fireRequest{
			ScheduleID:        ready.ID,
			Kind:              fireRequestManual,
			ScheduledAtUnixMs: base.Add(-time.Minute).UnixMilli(),
		})

		lockReady := make(chan struct{})
		releaseLock := make(chan struct{})
		lockDone := make(chan error, 1)

		go func() {
			lockDone <- env.DB.RunInTx(env.Ctx, func(ctx context.Context, tx orm.DB) error {
				schedule := new(cron.Schedule)
				schedule.ID = locked.ID

				if err := tx.NewSelect().Model(schedule).WherePK().ForUpdate().Scan(ctx); err != nil {
					return err
				}

				close(lockReady)
				<-releaseLock

				return nil
			})
		}()

		var releaseOnce sync.Once

		release := func() { releaseOnce.Do(func() { close(releaseLock) }) }
		defer release()

		select {
		case <-lockReady:
		case err := <-lockDone:
			require.NoError(t, err, "The schedule lock transaction should start")
			t.Fatal("The schedule lock transaction ended before holding the row")
		case <-time.After(5 * time.Second):
			t.Fatal("The schedule lock transaction did not acquire the row")
		}

		claimCtx, cancel := context.WithTimeout(env.Ctx, 5*time.Second)
		claimed, err := newTestClaimer(env.DB, registry, "node-ready", fixedNow(base)).ClaimDue(claimCtx, 1)

		cancel()

		release()
		require.NoError(t, <-lockDone, "The schedule lock transaction should commit")
		require.NoError(t, err, "Claiming should skip the contended schedule without blocking")
		require.Len(t, claimed, 1, "SKIP LOCKED should refill the claim window")
		assert.Equal(t, ready.ID, claimed[0].schedule.ID, "The next unlocked schedule should dispatch")
		assert.Len(t, loadFireRequests(t, env.DB, locked.ID), 1,
			"The locked schedule request should remain pending")
		assert.Empty(t, loadFireRequests(t, env.DB, ready.ID), "The unlocked schedule request should be consumed")
	})
}
