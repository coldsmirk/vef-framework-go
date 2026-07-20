package store

import (
	"context"
	"errors"
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
