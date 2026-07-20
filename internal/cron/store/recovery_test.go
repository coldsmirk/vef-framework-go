package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// insertRunningRun persists a running journal row heartbeated at the given
// instant.
func insertRunningRun(t *testing.T, db orm.DB, schedule *cron.Schedule, scheduledAt, heartbeatAt time.Time) *cron.Run {
	t.Helper()

	started := timex.DateTime(scheduledAt)
	heartbeat := timex.DateTime(heartbeatAt)
	run := &cron.Run{
		ScheduleID:   schedule.ID,
		ScheduleName: schedule.Name,
		JobName:      schedule.JobName,
		ScheduledAt:  timex.DateTime(scheduledAt),
		Status:       cron.RunRunning,
		NodeID:       "node-dead",
		StartedAt:    &started,
		HeartbeatAt:  &heartbeat,
	}

	_, err := db.NewInsert().Model(run).Exec(context.Background())
	require.NoError(t, err, "running run fixture insert should succeed")

	return run
}

func newSweepEngine(db orm.DB, registry *Registry, bus *captureBus, now time.Time) *Engine {
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(bus))
	engine.now = fixedNow(now)

	return engine
}

func TestSweepAbandoned(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	t.Run("MarksStaleRunsAndRefiresRecoverable", func(t *testing.T) {
		db := newStoreDB(t)
		bus := new(captureBus)

		recoverable := scheduleFixture("recoverable", "orders.sync", base.Add(time.Hour))
		recoverable.Recover = true
		insertSchedule(t, db, recoverable)

		disposable := scheduleFixture("disposable", "orders.sync", base.Add(time.Hour))
		insertSchedule(t, db, disposable)

		// Both runs went silent long past the abandoned window.
		staleBeat := base.Add(-time.Minute)
		orphanA := insertRunningRun(t, db, recoverable, base.Add(-2*time.Minute), staleBeat)
		orphanB := insertRunningRun(t, db, disposable, base.Add(-2*time.Minute), staleBeat)

		engine := newSweepEngine(db, registry, bus, base)
		engine.sweepAbandoned(context.Background())

		for _, orphan := range []*cron.Run{orphanA, orphanB} {
			runs := loadRuns(t, db, orphan.ScheduleID)
			require.Len(t, runs, 1, "the orphan must stay journaled")
			assert.Equal(t, cron.RunAbandoned, runs[0].Status, "a stale heartbeat turns the run abandoned")
			assert.NotNil(t, runs[0].FinishedAt, "an abandoned run is terminal")
		}

		events := bus.Published()
		require.Len(t, events, 2, "every abandoned run must publish a notification")

		abandoned, ok := events[0].(*cron.RunAbandonedEvent)
		require.True(t, ok, "the notification must be a run-abandoned event")
		assert.Equal(t, "node-dead", abandoned.NodeID, "the event must name the silent node")

		refired := reloadSchedule(t, db, recoverable.ID)
		require.NotNil(t, refired.NextFireAt, "the recoverable schedule must be re-armed")
		assert.True(t, refired.NextFireAt.AsLocal().Equal(base), "recovery pulls the fire to now")

		untouched := reloadSchedule(t, db, disposable.ID)
		assert.True(t, untouched.NextFireAt.AsLocal().Equal(base.Add(time.Hour)),
			"a schedule without Recover keeps its regular fire")
	})

	t.Run("RefiresAnOverdueSkipSchedule", func(t *testing.T) {
		// An already-due fire is normally left alone because the imminent
		// claim covers the recovery — but under MisfireSkip that claim
		// journals the fire as missed and runs nothing, taking the recovery
		// down with it.
		db := newStoreDB(t)

		schedule := scheduleFixture("skipper", "orders.sync", base.Add(-time.Hour))
		schedule.MisfirePolicy = cron.MisfireSkip
		schedule.Recover = true
		insertSchedule(t, db, schedule)

		insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(db, registry, new(captureBus), base)
		engine.sweepAbandoned(context.Background())

		refired := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, refired.NextFireAt, "the recoverable schedule must stay armed")
		assert.True(t, refired.NextFireAt.AsLocal().Equal(base),
			"recovery must pull an overdue skip schedule to now, or the re-fire is journaled as missed")
	})

	t.Run("LeavesAnOverdueCatchUpScheduleAlone", func(t *testing.T) {
		// Under fire_now the pending overdue fire already produces a catch-up
		// run, so recovery must not overwrite its logical time.
		db := newStoreDB(t)

		overdue := base.Add(-time.Hour)
		schedule := scheduleFixture("catcher", "orders.sync", overdue)
		schedule.Recover = true
		insertSchedule(t, db, schedule)

		insertRunningRun(t, db, schedule, base.Add(-2*time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(db, registry, new(captureBus), base)
		engine.sweepAbandoned(context.Background())

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAt, "the schedule must stay armed")
		assert.True(t, after.NextFireAt.AsLocal().Equal(overdue),
			"an overdue catch-up fire keeps its logical time; the imminent claim already recovers it")
	})

	t.Run("FreshHeartbeatsAreLeftAlone", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("alive", "orders.sync", base.Add(time.Hour)))
		insertRunningRun(t, db, schedule, base.Add(-time.Minute), base.Add(-10*time.Millisecond))

		engine := newSweepEngine(db, registry, new(captureBus), base)
		engine.sweepAbandoned(context.Background())

		runs := loadRuns(t, db, schedule.ID)
		assert.Equal(t, cron.RunRunning, runs[0].Status, "a live run must survive the sweep")
	})
}

func TestRenewHeartbeats(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	t.Run("StampsTrackedRunningRows", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("beating", "orders.sync", base.Add(time.Hour)))
		run := insertRunningRun(t, db, schedule, base.Add(-time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(db, registry, new(captureBus), base)
		engine.heartbeats.Track(run.ID)
		engine.renewHeartbeats()

		runs := loadRuns(t, db, schedule.ID)
		assert.True(t, runs[0].HeartbeatAt.AsLocal().After(base.Add(-time.Second)),
			"the tracked run's heartbeat must be freshly stamped")
	})

	t.Run("NeverResurrectsRecoveredRows", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("taken", "orders.sync", base.Add(time.Hour)))
		run := insertRunningRun(t, db, schedule, base.Add(-time.Minute), base.Add(-time.Minute))

		engine := newSweepEngine(db, registry, new(captureBus), base)
		engine.heartbeats.Track(run.ID)

		// A peer's sweep took the run over between two beats.
		engine.sweepAbandoned(context.Background())
		engine.renewHeartbeats()

		runs := loadRuns(t, db, schedule.ID)
		assert.Equal(t, cron.RunAbandoned, runs[0].Status, "a recovered run must stay abandoned")
	})
}

func TestPruneJournal(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	db := newStoreDB(t)
	schedule := insertSchedule(t, db, scheduleFixture("journaled", "orders.sync", base.Add(time.Hour)))

	insertFinishedRun := func(scheduledAt, finishedAt time.Time, status cron.RunStatus) {
		finished := timex.DateTime(finishedAt)
		run := &cron.Run{
			ScheduleID:   schedule.ID,
			ScheduleName: schedule.Name,
			JobName:      schedule.JobName,
			ScheduledAt:  timex.DateTime(scheduledAt),
			Status:       status,
			FinishedAt:   &finished,
		}
		_, err := db.NewInsert().Model(run).Exec(context.Background())
		require.NoError(t, err, "journal fixture insert should succeed")
	}

	// Beyond retention, inside retention, and an ancient-but-running row.
	insertFinishedRun(base.Add(-48*time.Hour), base.Add(-48*time.Hour), cron.RunSucceeded)
	insertFinishedRun(base.Add(-30*time.Minute), base.Add(-30*time.Minute), cron.RunFailed)
	insertRunningRun(t, db, schedule, base.Add(-72*time.Hour), base.Add(-time.Second))

	config := fastStoreConfig()
	config.RunRetention = 24 * time.Hour

	engine := NewEngine(db, config, registry, NewRunEventPublisher(new(captureBus)))
	engine.now = fixedNow(base)
	engine.pruneJournal(context.Background())

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 2, "only the terminal row beyond retention must be pruned")

	statuses := []cron.RunStatus{runs[0].Status, runs[1].Status}
	assert.Contains(t, statuses, cron.RunRunning, "running rows are never pruned regardless of age")
	assert.Contains(t, statuses, cron.RunFailed, "terminal rows inside retention must survive")
}
