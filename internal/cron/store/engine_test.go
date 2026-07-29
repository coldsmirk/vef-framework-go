package store

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// EngineHarness runs a live engine against a migrated store with a bounded
// lifetime.
type EngineHarness struct {
	db      orm.DB
	engine  *Engine
	manager cron.ScheduleManager
	bus     *eventtest.FakeBus
}

func startEngine(t *testing.T, handlers ...cron.JobHandler) *EngineHarness {
	t.Helper()

	db := newStoreDB(t)
	registry := mustRegistry(t, handlers...)
	bus := eventtest.NewFakeBus()
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(bus))
	engine.drainTimeout = 200 * time.Millisecond

	engine.Start()
	t.Cleanup(func() {
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	return &EngineHarness{
		db:      db,
		engine:  engine,
		manager: NewScheduleManager(db, true, registry, engine),
		bus:     bus,
	}
}

// awaitRun polls the journal until a run of the schedule reaches the wanted
// status.
func awaitRun(t *testing.T, db orm.DB, scheduleID string, status cron.RunStatus) cron.Run {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		for _, run := range loadRuns(t, db, scheduleID) {
			if run.Status == status {
				return run
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf("A %s run of schedule %s must appear", status, scheduleID)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func (h *EngineHarness) awaitRun(t *testing.T, scheduleID string, status cron.RunStatus) cron.Run {
	t.Helper()

	return awaitRun(t, h.db, scheduleID, status)
}

// BusyError mimics the SQLite driver's write-lock contention error: the
// message plus the result code the shared classifier trusts.
type BusyError struct{}

func (*BusyError) Error() string { return "database is locked (5) (SQLITE_BUSY)" }

func (*BusyError) Code() int { return 5 }

// FlakyDB makes the first failures RunInTx calls report write-lock
// contention. commitFirst decides whether those calls still run the
// transaction to completion — the case where the write landed and only the
// caller's report of it failed.
type FlakyDB struct {
	orm.DB

	failures    int32
	commitFirst bool
	attempts    atomic.Int32
}

func (d *FlakyDB) RunInTx(ctx context.Context, fn func(context.Context, orm.DB) error) error {
	if d.attempts.Add(1) > d.failures {
		return d.DB.RunInTx(ctx, fn)
	}

	if d.commitFirst {
		if err := d.DB.RunInTx(ctx, fn); err != nil {
			return err
		}
	}

	return new(BusyError)
}

// canceledFire builds the claimed fire and journal row a graceful shutdown
// hands to writeOutcome: a recoverable schedule whose run was canceled.
func canceledFire(t *testing.T, db orm.DB) (claimedFire, *cron.Run) {
	t.Helper()

	at := time.Now().Add(-time.Minute)
	fixture := scheduleFixture("outcome-retry", "orders.sync", at)
	fixture.Recover = true
	schedule := insertSchedule(t, db, fixture)
	run := insertRunningRun(t, db, schedule, at, at)
	run.Status = cron.RunCanceled
	run.Error = "canceled by shutdown"
	run.FinishedAtUnixMs = unixMsPtr(time.Now())

	return claimedFire{run: run, schedule: *schedule}, run
}

func TestEngineMarksMaintenanceContextsQuiet(t *testing.T) {
	db := newStoreDB(t)
	registry := mustRegistry(t, noopHandler("orders.sync"))
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus()))

	assert.True(t, orm.IsQuietSQLLog(engine.loopCtx),
		"Claim and maintenance queries must log quietly")
	assert.True(t, orm.IsQuietSQLLog(engine.heartbeatCtx),
		"Heartbeat renewals must log quietly")
	assert.False(t, orm.IsQuietSQLLog(engine.runCtx),
		"Handler business queries must keep their normal log level")
}

func TestEngineSweepIntervalTracksTheStalenessWindow(t *testing.T) {
	db := newStoreDB(t)
	registry := mustRegistry(t, noopHandler("orders.sync"))
	cfg := fastStoreConfig()
	cfg.AbandonedAfter = time.Hour
	cfg.PollInterval = 5 * time.Second
	engine := NewEngine(db, cfg, registry, NewRunEventPublisher(eventtest.NewFakeBus()))

	assert.Equal(t, 30*time.Minute, engine.sweepInterval(),
		"A long abandoned window must not be swept on the far shorter poll rhythm: nothing can go stale sooner")
}

func TestEngineExecutesFires(t *testing.T) {
	t.Run("SucceededRunWithParams", func(t *testing.T) {
		type Payload struct {
			Region string `json:"region"`
		}

		var (
			executions atomic.Int32
			seenRegion atomic.Value
		)

		harness := startEngine(t, cron.NewTypedJobHandler("orders.sync",
			func(_ context.Context, params Payload) error {
				seenRegion.Store(params.Region)
				executions.Add(1)

				return nil
			}))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-east",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
			Params:  Payload{Region: "east"},
		})
		require.NoError(t, err, "Creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunSucceeded)
		assert.Equal(t, int32(1), executions.Load(), "The handler must run exactly once")
		assert.Equal(t, "east", seenRegion.Load(), "The schedule params must reach the handler")
		assert.NotNil(t, run.FinishedAtUnixMs, "The journal must close the run")
		assert.Empty(t, harness.bus.Captured(), "A successful run publishes nothing")

		spent, err := harness.manager.Get(context.Background(), "sync-east")
		require.NoError(t, err, "The schedule must load")
		assert.Nil(t, spent.NextFireAtUnixMs, "The one-shot must be spent")
	})

	t.Run("FailedRunPublishesEvent", func(t *testing.T) {
		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(context.Context, cron.Execution) error { return errors.New("upstream exploded") }))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-broken",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
		})
		require.NoError(t, err, "Creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "upstream exploded", "The journal must carry the failure")

		require.Eventually(t, func() bool { return len(harness.bus.Captured()) == 1 },
			2*time.Second, 20*time.Millisecond, "The failure must publish an event")

		event, ok := harness.bus.Captured()[0].(*cron.RunFailedEvent)
		require.True(t, ok, "The published event must be a run-failed event")
		assert.Equal(t, "sync-broken", event.ScheduleName, "The event must name the schedule")
		assert.Contains(t, event.Error, "upstream exploded", "The event must carry the failure")
	})

	t.Run("ExecutionUsesUTC", func(t *testing.T) {
		scheduled := make(chan time.Time, 1)
		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(_ context.Context, execution cron.Execution) error {
				scheduled <- execution.ScheduledAt

				return nil
			}))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "utc-execution",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(30 * time.Millisecond)),
		})
		require.NoError(t, err, "Creating the UTC execution fixture should succeed")

		harness.awaitRun(t, schedule.ID, cron.RunSucceeded)

		select {
		case executionTime := <-scheduled:
			assert.Same(t, time.UTC, executionTime.Location(),
				"The handler should receive the persisted fire instant in the canonical UTC location")
		case <-time.After(time.Second):
			t.Fatal("The handler must receive its execution metadata")
		}
	})

	t.Run("PanicIsJournaledAsFailed", func(t *testing.T) {
		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(context.Context, cron.Execution) error { panic("boom") }))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-panicky",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
		})
		require.NoError(t, err, "Creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "panicked", "The journal must record the panic")
		assert.Contains(t, run.Error, "boom", "The journal must carry the panic value")
	})

	t.Run("TimeoutFailsTheRun", func(t *testing.T) {
		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(ctx context.Context, _ cron.Execution) error {
				<-ctx.Done()

				return ctx.Err()
			}))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-slow",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
			Timeout: 100 * time.Millisecond,
		})
		require.NoError(t, err, "Creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "timed out", "The journal must record the timeout")
	})

	t.Run("TimeoutOutranksASwallowedDeadline", func(t *testing.T) {
		// A handler that returns nil once its context dies is claiming
		// success it did not achieve; the run's own deadline is the truth.
		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(ctx context.Context, _ cron.Execution) error {
				<-ctx.Done()

				return nil
			}))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-swallows",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
			Timeout: 100 * time.Millisecond,
		})
		require.NoError(t, err, "Creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "timed out",
			"A nil return after the deadline must still journal as a timeout, never as success")
	})

	t.Run("TriggerNowFiresAgain", func(t *testing.T) {
		var executions atomic.Int32

		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(context.Context, cron.Execution) error {
				executions.Add(1)

				return nil
			}))

		_, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-manual",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
		})
		require.NoError(t, err, "Creating the schedule should succeed")

		require.Eventually(t, func() bool { return executions.Load() == 1 },
			5*time.Second, 20*time.Millisecond, "The scheduled fire must execute")

		require.NoError(t, harness.manager.TriggerNow(context.Background(), "sync-manual"),
			"The manual fire should be accepted")

		require.Eventually(t, func() bool { return executions.Load() == 2 },
			5*time.Second, 20*time.Millisecond, "The manual fire must execute")
	})
}

func TestEngineWakesWhenAnExecutorSlotIsReleased(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	second := make(chan struct{})

	var executions atomic.Int32

	handler := cron.NewJobHandler("orders.sync", func(context.Context, cron.Execution) error {
		switch executions.Add(1) {
		case 1:
			close(started)
			<-release
		case 2:
			close(second)
		}

		return nil
	})

	db := newStoreDB(t)
	registry := mustRegistry(t, handler)
	config := fastStoreConfig()
	config.PollInterval = time.Hour
	config.BatchSize = 1
	config.MaxConcurrent = 1
	// This test waits on the recovery sweep, whose cadence is half the
	// abandoned window, so it shortens the window the harness deliberately
	// keeps long. Judging the blocked handler's own run abandoned is harmless
	// here: the executor releases its slot when the handler returns either way,
	// which is the behavior under test.
	config.AbandonedAfter = 50 * time.Millisecond
	engine := NewEngine(db, config, registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	engine.drainTimeout = 200 * time.Millisecond
	manager := NewScheduleManager(db, true, registry, engine)
	engine.Start()

	var releaseOnce sync.Once

	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() {
		releaseHandler()
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "slot-release",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(20 * time.Millisecond)),
	})
	require.NoError(t, err, "Creating the first fire should succeed")

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("The first handler must occupy the only executor slot")
	}

	firstMarker := insertSchedule(t, db, scheduleFixture("first-tick-marker", "orders.sync", time.Now().Add(time.Hour)))
	insertRunningRun(t, db, firstMarker, time.Now().Add(-time.Minute), time.Now().Add(-time.Minute))
	engine.Wake()
	awaitRun(t, db, firstMarker.ID, cron.RunAbandoned)

	secondMarker := insertSchedule(t, db, scheduleFixture("full-slot-marker", "orders.sync", time.Now().Add(time.Hour)))
	insertRunningRun(t, db, secondMarker, time.Now().Add(-time.Minute), time.Now().Add(-time.Minute))
	require.NoError(t, manager.TriggerNow(context.Background(), schedule.Name),
		"Queuing a manual fire while the slot is full should succeed")
	awaitRun(t, db, secondMarker.ID, cron.RunAbandoned)

	releaseHandler()

	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("Releasing the slot must wake the engine without waiting for the one-hour poll interval")
	}
}

func TestEngineTickDrainsJournalOnlyProgress(t *testing.T) {
	base := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	db := newStoreDB(t)
	registry := mustRegistry(t, noopHandler("orders.sync"))
	config := fastStoreConfig()
	config.BatchSize = 1
	config.MaxConcurrent = 1
	engine := NewEngine(db, config, registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	stubEngineClock(engine, base)

	schedule := insertSchedule(t, db, scheduleFixture("skip-backlog", "orders.sync", base.Add(time.Hour)))
	insertRunningRun(t, db, schedule, base.Add(-time.Minute), base)

	for i := range 3 {
		insertFireRequest(t, db, &fireRequest{
			ScheduleID:        schedule.ID,
			Kind:              fireRequestManual,
			ScheduledAtUnixMs: base.Add(time.Duration(i) * time.Millisecond).UnixMilli(),
		})
	}

	engine.tick()

	assert.Empty(t, loadFireRequests(t, db, schedule.ID),
		"One tick should drain every request whose skipped journal is durable progress")
	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 4, "The active run and all three skipped requests should remain journaled")

	for _, run := range runs[1:] {
		assert.Equal(t, cron.RunSkipped, run.Status, "Every blocked manual request should be journaled as skipped")
	}
}

func TestEngineTickYieldsAfterBoundedJournalProgress(t *testing.T) {
	base := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	db := newStoreDB(t)
	registry := mustRegistry(t, noopHandler("orders.sync"))
	config := fastStoreConfig()
	config.BatchSize = 1
	config.MaxConcurrent = 1
	engine := NewEngine(db, config, registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	stubEngineClock(engine, base)

	schedule := insertSchedule(t, db, scheduleFixture("bounded-backlog", "orders.sync", base.Add(time.Hour)))
	insertRunningRun(t, db, schedule, base.Add(-time.Minute), base)

	for i := range maxClaimBatchesPerTick + 1 {
		insertFireRequest(t, db, &fireRequest{
			ScheduleID:        schedule.ID,
			Kind:              fireRequestManual,
			ScheduledAtUnixMs: base.Add(-time.Duration(i) * time.Millisecond).UnixMilli(),
		})
	}

	engine.tick()

	assert.Len(t, loadFireRequests(t, db, schedule.ID), 1,
		"One request should remain after the tick yields to maintenance")
	assert.Len(t, engine.wake, 1, "The engine should schedule an immediate continuation for the remaining backlog")
}

func TestBlockedFailurePublishDoesNotHoldExecutorSlot(t *testing.T) {
	bus := newBlockingBus(false)
	secondStarted := make(chan struct{})

	var executions atomic.Int32

	handler := cron.NewJobHandler("orders.sync", func(context.Context, cron.Execution) error {
		if executions.Add(1) == 1 {
			return errors.New("first execution failed")
		}

		close(secondStarted)

		return nil
	})

	db := newStoreDB(t)
	registry := mustRegistry(t, handler)
	config := fastStoreConfig()
	config.PollInterval = time.Hour
	config.BatchSize = 1
	config.MaxConcurrent = 1
	engine := NewEngine(db, config, registry, NewRunEventPublisher(bus))
	manager := NewScheduleManager(db, true, registry, engine)
	engine.Start()

	t.Cleanup(func() {
		bus.releasePublish()
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "blocked-failure-event",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(20 * time.Millisecond)),
	})
	require.NoError(t, err, "Creating the failure fixture should succeed")

	awaitRun(t, db, schedule.ID, cron.RunFailed)

	select {
	case <-bus.entered:
	case <-time.After(time.Second):
		t.Fatal("The event worker must enter the blocking publish")
	}

	require.NoError(t, manager.TriggerNow(context.Background(), schedule.Name),
		"Queuing a second fire should succeed while notification delivery is blocked")

	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("A blocked failure notification must not retain the only executor slot")
	}
}

func TestBlockedAbandonedPublishDoesNotStopClaiming(t *testing.T) {
	base := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	bus := newBlockingBus(false)
	started := make(chan string, 2)
	handler := cron.NewJobHandler("orders.sync", func(_ context.Context, execution cron.Execution) error {
		started <- execution.ScheduleName

		return nil
	})

	db := newStoreDB(t)
	registry := mustRegistry(t, handler)
	first := insertSchedule(t, db, scheduleFixture("abandoned-first", "orders.sync", base))
	insertRunningRun(t, db, first, base.Add(-time.Minute), base.Add(-time.Minute))
	insertSchedule(t, db, scheduleFixture("ready-second", "orders.sync", base))

	config := fastStoreConfig()
	config.PollInterval = time.Hour
	config.BatchSize = 1
	config.MaxConcurrent = 2
	engine := NewEngine(db, config, registry, NewRunEventPublisher(bus))
	stubEngineClock(engine, base)
	engine.Start()

	t.Cleanup(func() {
		bus.releasePublish()
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	select {
	case <-bus.entered:
	case <-time.After(time.Second):
		t.Fatal("The event worker must enter the abandoned-run publish")
	}

	seen := make(map[string]struct{}, 2)
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = struct{}{}
		case <-time.After(time.Second):
			t.Fatal("A blocked abandoned notification must not stop the next claim batch")
		}
	}

	assert.Contains(t, seen, first.Name, "The schedule whose stale run was taken over should execute")
	assert.Contains(t, seen, "ready-second", "The following due schedule should execute in the same tick")
}

func TestEngineStopCancelsStragglers(t *testing.T) {
	blocked := make(chan struct{})

	db := newStoreDB(t)
	registry := mustRegistry(t, cron.NewJobHandler("orders.sync",
		func(ctx context.Context, _ cron.Execution) error {
			close(blocked)
			<-ctx.Done()

			return ctx.Err()
		}))
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	engine.drainTimeout = 100 * time.Millisecond
	manager := NewScheduleManager(db, true, registry, engine)

	engine.Start()
	t.Cleanup(func() {
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "sync-blocked",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(30 * time.Millisecond)),
	})
	require.NoError(t, err, "Creating the schedule should succeed")

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("The handler must start before the engine stops")
	}

	require.NoError(t, engine.Stop(context.Background()), "The engine should cancel and journal the blocked run")

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 1, "The interrupted fire must stay journaled")
	assert.Equal(t, cron.RunCanceled, runs[0].Status, "Shutdown interruption journals as canceled")
	assert.Equal(t, "canceled by shutdown", runs[0].Error, "The journal must name the shutdown")
	assert.Empty(t, loadFireRequests(t, db, schedule.ID),
		"A canceled run of a non-recoverable schedule must not queue a re-fire")
}

func TestEngineStopQueuesRecoveryForCanceledRecoverableRun(t *testing.T) {
	blocked := make(chan struct{})

	db := newStoreDB(t)
	registry := mustRegistry(t, cron.NewJobHandler("orders.sync",
		func(ctx context.Context, _ cron.Execution) error {
			close(blocked)
			<-ctx.Done()

			return ctx.Err()
		}))
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	engine.drainTimeout = 100 * time.Millisecond
	manager := NewScheduleManager(db, true, registry, engine)

	engine.Start()
	t.Cleanup(func() {
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "sync-recoverable",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(30 * time.Millisecond)),
		Recover: true,
	})
	require.NoError(t, err, "Creating the schedule should succeed")

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("The handler must start before the engine stops")
	}

	require.NoError(t, engine.Stop(context.Background()), "The engine should cancel and journal the blocked run")

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 1, "The interrupted fire must stay journaled")
	assert.Equal(t, cron.RunCanceled, runs[0].Status, "Shutdown interruption journals as canceled")

	requests := loadFireRequests(t, db, schedule.ID)
	require.Len(t, requests, 1,
		"Graceful shutdown must queue the same durable re-fire a crash would produce")
	assert.Equal(t, fireRequestRecovery, requests[0].Kind, "The re-fire must be a recovery request")
	assert.Equal(t, runs[0].ID, requests[0].SourceRunID, "The canceled run must fence its re-queue")
	assert.Equal(t, runs[0].ScheduledAtUnixMs, requests[0].ScheduledAtUnixMs,
		"The recovery must retain the canceled occurrence time")
}

func TestEngineWriteOutcomeRetriesLostLockRaces(t *testing.T) {
	newEngineOver := func(db orm.DB) *Engine {
		return NewEngine(db, fastStoreConfig(), mustRegistry(t, noopHandler("orders.sync")),
			NewRunEventPublisher(eventtest.NewFakeBus()))
	}

	t.Run("RetriesUntilTheOutcomeLands", func(t *testing.T) {
		db := newStoreDB(t)
		fire, run := canceledFire(t, db)
		flaky := &FlakyDB{DB: db, failures: 2}

		journaled, err := newEngineOver(flaky).writeOutcome(context.Background(), fire, run)
		require.NoError(t, err, "A lost lock race must be retried until the outcome lands")
		assert.True(t, journaled, "The retried write reached the journal, so the caller may report it")
		assert.Equal(t, int32(3), flaky.attempts.Load(), "The two refused attempts must both be retried")

		runs := loadRuns(t, db, run.ScheduleID)
		require.Len(t, runs, 1, "The retry must journal exactly one outcome row")
		assert.Equal(t, cron.RunCanceled, runs[0].Status, "The retried write must reach the terminal status")

		requests := loadFireRequests(t, db, run.ScheduleID)
		require.Len(t, requests, 1, "The retried transaction must queue exactly one recovery request")
		assert.Equal(t, run.ID, requests[0].SourceRunID, "The canceled run must fence its re-queue")
	})

	t.Run("DiscardsTheRetryOfACommittedAttempt", func(t *testing.T) {
		db := newStoreDB(t)
		fire, run := canceledFire(t, db)
		flaky := &FlakyDB{DB: db, failures: 1, commitFirst: true}

		journaled, err := newEngineOver(flaky).writeOutcome(context.Background(), fire, run)
		require.NoError(t, err, "An attempt that committed before reporting contention must still resolve")
		assert.True(t, journaled,
			"The lost report must not cost the run its outcome notification: the write did land")
		assert.Equal(t, int32(2), flaky.attempts.Load(), "The reported failure must be retried once")

		runs := loadRuns(t, db, run.ScheduleID)
		require.Len(t, runs, 1, "The journal must hold exactly one outcome row")
		assert.Equal(t, cron.RunCanceled, runs[0].Status, "The committed outcome must stand")

		requests := loadFireRequests(t, db, run.ScheduleID)
		require.Len(t, requests, 1,
			"The retry must find the row no longer running and skip a second recovery request")
		assert.Equal(t, run.ID, requests[0].SourceRunID, "The surviving request must carry the run fence")
	})

	t.Run("GivesUpWhenTheCompletionWindowCloses", func(t *testing.T) {
		db := newStoreDB(t)
		fire, run := canceledFire(t, db)
		flaky := &FlakyDB{DB: db, failures: math.MaxInt32}
		engine := newEngineOver(flaky)

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		outcome := make(chan error, 1)
		go func() {
			_, err := engine.writeOutcome(ctx, fire, run)
			outcome <- err
		}()

		select {
		case err := <-outcome:
			require.Error(t, err, "Unending contention must surface as the journal write's failure")
		case <-time.After(5 * time.Second):
			t.Fatal("The outcome write must give up with its context instead of retrying forever")
		}

		runs := loadRuns(t, db, run.ScheduleID)
		require.Len(t, runs, 1, "The unwritten outcome must leave the claimed row alone")
		assert.Equal(t, cron.RunRunning, runs[0].Status,
			"A surrendered outcome write leaves the run running for abandoned-run recovery")
		assert.Empty(t, loadFireRequests(t, db, run.ScheduleID),
			"A transaction that never committed must queue no recovery request")
	})
}

// TestEngineCompleteStaysSilentWhenRecoveryTookTheRunOver pins the split
// between journaling an outcome and reporting one. A handler that finishes
// after the recovery sweep already declared its run abandoned has nothing left
// to say: the journal is the truth these events only notify about, and the
// sweep published the terminal event for this run already.
func TestEngineCompleteStaysSilentWhenRecoveryTookTheRunOver(t *testing.T) {
	db := newStoreDB(t)
	bus := eventtest.NewFakeBus()
	publisher := NewRunEventPublisher(bus)
	engine := NewEngine(db, fastStoreConfig(), mustRegistry(t, noopHandler("orders.sync")), publisher)

	at := time.Now().Add(-time.Minute)
	schedule := insertSchedule(t, db, scheduleFixture("taken-over", "orders.sync", at))
	run := insertRunningRun(t, db, schedule, at, at)

	_, err := db.NewUpdate().
		Model((*cron.Run)(nil)).
		Set("status", cron.RunAbandoned).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(run.ID) }).
		Exec(context.Background())
	require.NoError(t, err, "Simulating the recovery takeover should succeed")

	engine.complete(claimedFire{run: run, schedule: *schedule}, errors.New("upstream exploded"), nil)

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 1, "The takeover row must remain the only journal entry")
	assert.Equal(t, cron.RunAbandoned, runs[0].Status,
		"A late completion must never overwrite the status recovery already journaled")

	require.NoError(t, publisher.Stop(context.Background()), "Flushing the publisher should succeed")
	assert.Empty(t, bus.Captured(),
		"A discarded outcome must publish nothing: reporting it would contradict the journal and "+
			"hand subscribers two terminal events for one run")
}

func TestEngineStopSkipsRecoveryForADeletedSchedule(t *testing.T) {
	blocked := make(chan struct{})

	db := newStoreDB(t)
	registry := mustRegistry(t, cron.NewJobHandler("orders.sync",
		func(ctx context.Context, _ cron.Execution) error {
			close(blocked)
			<-ctx.Done()

			return ctx.Err()
		}))
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	engine.drainTimeout = 100 * time.Millisecond
	manager := NewScheduleManager(db, true, registry, engine)

	engine.Start()
	t.Cleanup(func() {
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "sync-deleted",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(30 * time.Millisecond)),
		Recover: true,
	})
	require.NoError(t, err, "Creating the schedule should succeed")

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("The handler must start before the schedule is deleted")
	}

	require.NoError(t, manager.Delete(context.Background(), "sync-deleted"),
		"Deleting the schedule of a running fire should succeed")
	require.NoError(t, engine.Stop(context.Background()), "The engine should cancel and journal the blocked run")

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 1, "The interrupted fire must stay journaled")
	assert.Equal(t, cron.RunCanceled, runs[0].Status, "Shutdown interruption journals as canceled")

	assert.Empty(t, loadFireRequests(t, db, schedule.ID),
		"A recovery request for a deleted schedule is invisible to every reader and must never be written")

	orphans, err := db.NewSelect().Model((*fireRequest)(nil)).Count(context.Background())
	require.NoError(t, err, "Counting fire requests should succeed")
	assert.Zero(t, orphans, "The store must hold no fire request at all once its schedule is gone")
}

func TestEngineStopDrainsRunningWork(t *testing.T) {
	var (
		started  = make(chan struct{})
		finished atomic.Bool
	)

	db := newStoreDB(t)
	registry := mustRegistry(t, cron.NewJobHandler("orders.sync",
		func(context.Context, cron.Execution) error {
			close(started)
			time.Sleep(150 * time.Millisecond)
			finished.Store(true)

			return nil
		}))
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	manager := NewScheduleManager(db, true, registry, engine)

	engine.Start()
	t.Cleanup(func() {
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "sync-draining",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(30 * time.Millisecond)),
	})
	require.NoError(t, err, "Creating the schedule should succeed")

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("The handler must start before the engine stops")
	}

	require.NoError(t, engine.Stop(context.Background()), "The engine should drain the running work")

	assert.True(t, finished.Load(), "Stop must not return while a claimed run is still executing")

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 1, "The drained fire must stay journaled")
	assert.Equal(t, cron.RunSucceeded, runs[0].Status,
		"A run that finishes inside the drain window must be journaled as succeeded")
}

func TestEngineStopObeysCallerDeadline(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})

	db := newStoreDB(t)
	registry := mustRegistry(t, cron.NewJobHandler("orders.sync",
		func(ctx context.Context, _ cron.Execution) error {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release

			return ctx.Err()
		}))
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus()))
	engine.drainTimeout = time.Second
	manager := NewScheduleManager(db, true, registry, engine)

	engine.Start()

	var releaseOnce sync.Once

	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() {
		releaseHandler()
		require.NoError(t, engine.Stop(context.Background()), "The engine should stop during cleanup")
	})

	_, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "sync-budgeted",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(30 * time.Millisecond)),
	})
	require.NoError(t, err, "Creating the schedule should succeed")

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("The handler must start before the engine stops")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err = engine.Stop(stopCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"A handler that ignores cancellation must not outlive the caller's stop budget")

	select {
	case <-canceled:
	default:
		t.Fatal("The engine must cancel running handlers before returning at the caller deadline")
	}

	releaseHandler()
	require.NoError(t, engine.Stop(context.Background()), "The released handler should finish shutdown")
}
