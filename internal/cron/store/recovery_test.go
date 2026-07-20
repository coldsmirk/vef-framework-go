package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func newSweepEngine(t *testing.T, db orm.DB, registry *Registry, bus *CaptureBus, now time.Time) *Engine {
	t.Helper()

	publisher := NewRunEventPublisher(bus)
	publisher.Start()
	t.Cleanup(func() {
		require.NoError(t, publisher.Stop(context.Background()), "The run event publisher should stop during cleanup")
	})

	engine := NewEngine(db, fastStoreConfig(), registry, publisher)
	engine.now = fixedNow(now)

	return engine
}

func TestSweepAbandoned(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	t.Run("MarksStaleRunsAndQueuesRecoverable", func(t *testing.T) {
		db := newStoreDB(t)
		bus := new(CaptureBus)

		recoverable := scheduleFixture("recoverable", "orders.sync", base.Add(time.Hour))
		recoverable.Recover = true
		insertSchedule(t, db, recoverable)

		disposable := scheduleFixture("disposable", "orders.sync", base.Add(time.Hour))
		insertSchedule(t, db, disposable)

		// Both runs went silent long past the abandoned window.
		staleBeat := base.Add(-time.Minute)
		orphanA := insertRunningRun(t, db, recoverable, base.Add(-2*time.Minute), staleBeat)
		orphanB := insertRunningRun(t, db, disposable, base.Add(-2*time.Minute), staleBeat)

		engine := newSweepEngine(t, db, registry, bus, base)
		engine.sweepAbandoned(context.Background())

		for _, orphan := range []*cron.Run{orphanA, orphanB} {
			runs := loadRuns(t, db, orphan.ScheduleID)
			require.Len(t, runs, 1, "The orphan must stay journaled")
			assert.Equal(t, cron.RunAbandoned, runs[0].Status, "A stale heartbeat turns the run abandoned")
			assert.NotNil(t, runs[0].FinishedAtUnixMs, "An abandoned run is terminal")
		}

		require.Eventually(t, func() bool { return len(bus.Published()) == 2 },
			time.Second, 10*time.Millisecond, "Every abandoned run must publish a notification")

		events := bus.Published()
		require.Len(t, events, 2, "Every abandoned run must publish a notification")

		abandoned, ok := events[0].(*cron.RunAbandonedEvent)
		require.True(t, ok, "The notification must be a run-abandoned event")
		assert.Equal(t, "node-dead", abandoned.NodeID, "The event must name the silent node")

		after := reloadSchedule(t, db, recoverable.ID)
		require.NotNil(t, after.NextFireAtUnixMs, "The regular cursor should remain armed")
		assert.Equal(t, base.Add(time.Hour).UnixMilli(), *after.NextFireAtUnixMs,
			"Recovery should not move the regular cursor")

		requests := loadFireRequests(t, db, recoverable.ID)
		require.Len(t, requests, 1, "The recoverable orphan should create one durable request")
		assert.Equal(t, fireRequestRecovery, requests[0].Kind, "The request should be a recovery")
		assert.Equal(t, orphanA.ID, requests[0].SourceRunID, "The request should identify its orphan")
		assert.Equal(t, base.Add(-2*time.Minute).UnixMilli(), requests[0].ScheduledAtUnixMs,
			"The recovery should retain the orphan occurrence time")

		untouched := reloadSchedule(t, db, disposable.ID)
		require.NotNil(t, untouched.NextFireAtUnixMs, "The non-recoverable schedule should remain armed")
		assert.Equal(t, base.Add(time.Hour).UnixMilli(), *untouched.NextFireAtUnixMs,
			"A schedule without Recover keeps its regular fire")
		assert.Empty(t, loadFireRequests(t, db, disposable.ID),
			"A schedule without Recover should not queue a request")
	})

	t.Run("RecoveryBypassesMisfireSkip", func(t *testing.T) {
		db := newStoreDB(t)

		schedule := scheduleFixture("skipper", "orders.sync", base.Add(-time.Hour))
		schedule.MisfirePolicy = cron.MisfireSkip
		schedule.Recover = true
		insertSchedule(t, db, schedule)

		orphan := insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(t, db, registry, new(CaptureBus), base)
		engine.sweepAbandoned(context.Background())

		afterSweep := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, afterSweep.NextFireAtUnixMs, "The overdue regular cursor should remain present")
		assert.Equal(t, base.Add(-time.Hour).UnixMilli(), *afterSweep.NextFireAtUnixMs,
			"Recovery should not overwrite the overdue regular occurrence")

		claimed, err := newTestClaimer(db, registry, "node-live", fixedNow(base)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming recovery should succeed")
		require.Len(t, claimed, 1, "Recovery should execute despite the regular skip policy")
		assert.Equal(t, orphan.ScheduledAtUnixMs, claimed[0].run.ScheduledAtUnixMs,
			"The retry should keep the orphan occurrence identity")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID), "The claimed recovery should be consumed")

		runs := loadRuns(t, db, schedule.ID)

		statuses := make([]cron.RunStatus, len(runs))
		for i := range runs {
			statuses[i] = runs[i].Status
		}

		assert.Contains(t, statuses, cron.RunMissed, "The regular overdue gap should still be journaled")
		assert.Contains(t, statuses, cron.RunRunning, "The recovery should have its own running row")
	})

	t.Run("QueuesWithoutMovingAnOverdueCatchUp", func(t *testing.T) {
		db := newStoreDB(t)

		overdue := base.Add(-time.Hour)
		schedule := scheduleFixture("catcher", "orders.sync", overdue)
		schedule.Recover = true
		insertSchedule(t, db, schedule)

		insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(t, db, registry, new(CaptureBus), base)
		engine.sweepAbandoned(context.Background())

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAtUnixMs, "The schedule must stay armed")
		assert.Equal(t, overdue.UnixMilli(), *after.NextFireAtUnixMs,
			"The regular catch-up occurrence should retain its logical time")
		assert.Len(t, loadFireRequests(t, db, schedule.ID), 1,
			"The orphan should have a separate recovery request")
	})

	t.Run("QueuesEveryOrphanOfOneSchedule", func(t *testing.T) {
		db := newStoreDB(t)

		schedule := scheduleFixture("parallel", "orders.sync", base.Add(time.Hour))
		schedule.ConcurrencyPolicy = cron.ConcurrencyAllow
		schedule.Recover = true
		insertSchedule(t, db, schedule)

		first := insertRunningRun(t, db, schedule, base.Add(-3*time.Minute), base.Add(-time.Minute))
		second := insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(t, db, registry, new(CaptureBus), base)
		engine.sweepAbandoned(context.Background())

		requests := loadFireRequests(t, db, schedule.ID)
		require.Len(t, requests, 2, "Every orphan should create its own durable request")
		assert.Equal(t, first.ID, requests[0].SourceRunID, "The older orphan should be queued first")
		assert.Equal(t, second.ID, requests[1].SourceRunID, "The newer orphan should be queued second")

		claimed, err := newTestClaimer(db, registry, "node-live", fixedNow(base)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming both recoveries should succeed")
		require.Len(t, claimed, 2, "ConcurrencyAllow should dispatch every orphan recovery")
		assert.Equal(t, first.ScheduledAtUnixMs, claimed[0].run.ScheduledAtUnixMs,
			"The older orphan recovery should dispatch first")
		assert.Equal(t, second.ScheduledAtUnixMs, claimed[1].run.ScheduledAtUnixMs,
			"The newer orphan recovery should dispatch second")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID), "Both claimed requests should be consumed")
	})

	t.Run("PausedRecoveryWaitsWithoutBecomingAMisfire", func(t *testing.T) {
		db := newStoreDB(t)

		schedule := scheduleFixture("paused-recovery", "orders.sync", base.Add(time.Hour))
		schedule.MisfirePolicy = cron.MisfireSkip
		schedule.Recover = true
		insertSchedule(t, db, schedule)
		orphan := insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-time.Minute))

		manager := newTestManager(db, registry, fixedNow(base))
		require.NoError(t, manager.Pause(context.Background(), schedule.Name), "Pausing should succeed")

		engine := newSweepEngine(t, db, registry, new(CaptureBus), base)
		engine.sweepAbandoned(context.Background())

		claimed, err := newTestClaimer(db, registry, "node-live", fixedNow(base)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming while paused should not error")
		assert.Empty(t, claimed, "A paused schedule should not dispatch recovery")
		assert.Len(t, loadFireRequests(t, db, schedule.ID), 1,
			"The paused recovery should remain durable")

		resumeAt := base.Add(2 * time.Minute)
		manager.now = fixedNow(resumeAt)
		require.NoError(t, manager.Resume(context.Background(), schedule.Name), "Resuming should succeed")

		claimed, err = newTestClaimer(db, registry, "node-live", fixedNow(resumeAt)).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "Claiming after resume should succeed")
		require.Len(t, claimed, 1, "Recovery should bypass MisfireSkip after resume")
		assert.Equal(t, orphan.ScheduledAtUnixMs, claimed[0].run.ScheduledAtUnixMs,
			"The delayed retry should keep the orphan occurrence time")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID), "The resumed recovery should be consumed")
	})

	t.Run("FreshHeartbeatsAreLeftAlone", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("alive", "orders.sync", base.Add(time.Hour)))
		insertRunningRun(t, db, schedule, base.Add(-time.Minute), base.Add(-10*time.Millisecond))

		engine := newSweepEngine(t, db, registry, new(CaptureBus), base)
		engine.sweepAbandoned(context.Background())

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 1, "The live run should remain journaled")
		assert.Equal(t, cron.RunRunning, runs[0].Status, "A live run must survive the sweep")
	})

	t.Run("FinalizesAStaleRunAfterItsScheduleWasDeleted", func(t *testing.T) {
		db := newStoreDB(t)
		registry := mustRegistry(t, noopHandler("orders.sync"))
		bus := new(CaptureBus)
		schedule := scheduleFixture("deleted-owner", "orders.sync", base.Add(time.Hour))
		schedule.Recover = true
		insertSchedule(t, db, schedule)
		orphan := insertRunningRun(t, db, schedule, base.Add(-time.Hour), base.Add(-time.Hour))
		manager := newTestManager(db, registry, fixedNow(base))
		require.NoError(t, manager.Delete(context.Background(), schedule.Name),
			"Deleting the owning schedule should succeed")

		engine := newSweepEngine(t, db, registry, bus, base)
		engine.sweepAbandoned(context.Background())

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 1, "The scheduleless run should remain journaled")
		assert.Equal(t, orphan.ID, runs[0].ID, "The orphan should retain its journal identity")
		assert.Equal(t, cron.RunAbandoned, runs[0].Status,
			"A deleted schedule must not leave a stale run permanently running")
		assert.Empty(t, loadFireRequests(t, db, schedule.ID),
			"A deleted schedule must not receive a recovery request")
	})
}

func TestRenewHeartbeats(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	t.Run("StampsTrackedRunningRows", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("beating", "orders.sync", base.Add(time.Hour)))
		run := insertRunningRun(t, db, schedule, base.Add(-time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(t, db, registry, new(CaptureBus), base)
		engine.heartbeats.Track(run.ID)
		engine.renewHeartbeats()

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 1, "The tracked run should remain journaled")
		require.NotNil(t, runs[0].HeartbeatAtUnixMs, "The tracked run should retain a heartbeat")
		assert.Greater(t, *runs[0].HeartbeatAtUnixMs, base.Add(-time.Second).UnixMilli(),
			"The tracked run's heartbeat must be freshly stamped")
	})

	t.Run("NeverResurrectsRecoveredRows", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("taken", "orders.sync", base.Add(time.Hour)))
		run := insertRunningRun(t, db, schedule, base.Add(-time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(t, db, registry, new(CaptureBus), base)
		engine.heartbeats.Track(run.ID)

		// A peer's sweep took the run over between two beats.
		engine.sweepAbandoned(context.Background())
		engine.renewHeartbeats()

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 1, "The recovered run should remain journaled")
		assert.Equal(t, cron.RunAbandoned, runs[0].Status, "A recovered run must stay abandoned")
	})
}

func TestPruneJournal(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	db := newStoreDB(t)
	schedule := insertSchedule(t, db, scheduleFixture("journaled", "orders.sync", base.Add(time.Hour)))

	insertFinishedRun := func(scheduledAt, finishedAt time.Time, status cron.RunStatus) {
		run := &cron.Run{
			ScheduleID:        schedule.ID,
			ScheduleName:      schedule.Name,
			JobName:           schedule.JobName,
			ScheduledAtUnixMs: scheduledAt.UnixMilli(),
			ClaimedAtUnixMs:   scheduledAt.UnixMilli(),
			FinishedAtUnixMs:  unixMillisPtr(finishedAt),
			Status:            status,
		}
		_, err := db.NewInsert().Model(run).Exec(context.Background())
		require.NoError(t, err, "Journal fixture insert should succeed")
	}

	// Beyond retention, inside retention, and an ancient-but-running row.
	insertFinishedRun(base.Add(-48*time.Hour), base.Add(-48*time.Hour), cron.RunSucceeded)
	insertFinishedRun(base.Add(-30*time.Minute), base.Add(-30*time.Minute), cron.RunFailed)
	insertRunningRun(t, db, schedule, base.Add(-72*time.Hour), base.Add(-time.Second))

	config := fastStoreConfig()
	config.RunRetention = 24 * time.Hour

	engine := NewEngine(db, config, registry, NewRunEventPublisher(new(CaptureBus)))
	engine.now = fixedNow(base)
	engine.pruneJournal(context.Background())

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 2, "Only the terminal row beyond retention must be pruned")

	statuses := []cron.RunStatus{runs[0].Status, runs[1].Status}
	assert.Contains(t, statuses, cron.RunRunning, "Running rows are never pruned regardless of age")
	assert.Contains(t, statuses, cron.RunFailed, "Terminal rows inside retention must survive")
}
