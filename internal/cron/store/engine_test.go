package store

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// engineHarness runs a live engine against a migrated store with a bounded
// lifetime.
type engineHarness struct {
	db      orm.DB
	engine  *Engine
	manager cron.ScheduleManager
	bus     *captureBus
}

func startEngine(t *testing.T, handlers ...cron.JobHandler) *engineHarness {
	t.Helper()

	db := newStoreDB(t)
	registry := mustRegistry(t, handlers...)
	bus := new(captureBus)
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(bus))
	engine.drainTimeout = 200 * time.Millisecond

	engine.Start()
	t.Cleanup(engine.Stop)

	return &engineHarness{
		db:      db,
		engine:  engine,
		manager: NewScheduleManager(db, true, registry, engine),
		bus:     bus,
	}
}

// awaitRun polls the journal until a run of the schedule reaches the wanted
// status.
func (h *engineHarness) awaitRun(t *testing.T, scheduleID string, status cron.RunStatus) cron.Run {
	t.Helper()

	var found cron.Run

	require.Eventually(t, func() bool {
		for _, run := range loadRuns(t, h.db, scheduleID) {
			if run.Status == status {
				found = run

				return true
			}
		}

		return false
	}, 5*time.Second, 20*time.Millisecond, "a %s run of schedule %s must appear", status, scheduleID)

	return found
}

func TestEngineExecutesFires(t *testing.T) {
	t.Run("SucceededRunWithParams", func(t *testing.T) {
		type payload struct {
			Region string `json:"region"`
		}

		var (
			executions atomic.Int32
			seenRegion atomic.Value
		)

		harness := startEngine(t, cron.NewTypedJobHandler("orders.sync",
			func(_ context.Context, params payload) error {
				seenRegion.Store(params.Region)
				executions.Add(1)

				return nil
			}))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-east",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
			Params:  payload{Region: "east"},
		})
		require.NoError(t, err, "creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunSucceeded)
		assert.Equal(t, int32(1), executions.Load(), "the handler must run exactly once")
		assert.Equal(t, "east", seenRegion.Load(), "the schedule params must reach the handler")
		assert.NotNil(t, run.FinishedAt, "the journal must close the run")
		assert.Empty(t, harness.bus.Published(), "a successful run publishes nothing")

		spent, err := harness.manager.Get(context.Background(), "sync-east")
		require.NoError(t, err, "the schedule must load")
		assert.Nil(t, spent.NextFireAt, "the one-shot must be spent")
	})

	t.Run("FailedRunPublishesEvent", func(t *testing.T) {
		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(context.Context, cron.Execution) error { return errors.New("upstream exploded") }))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-broken",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
		})
		require.NoError(t, err, "creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "upstream exploded", "the journal must carry the failure")

		require.Eventually(t, func() bool { return len(harness.bus.Published()) == 1 },
			2*time.Second, 20*time.Millisecond, "the failure must publish an event")

		event, ok := harness.bus.Published()[0].(*cron.RunFailedEvent)
		require.True(t, ok, "the published event must be a run-failed event")
		assert.Equal(t, "sync-broken", event.ScheduleName, "the event must name the schedule")
		assert.Contains(t, event.Error, "upstream exploded", "the event must carry the failure")
	})

	t.Run("PanicIsJournaledAsFailed", func(t *testing.T) {
		harness := startEngine(t, cron.NewJobHandler("orders.sync",
			func(context.Context, cron.Execution) error { panic("boom") }))

		schedule, err := harness.manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "sync-panicky",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(50 * time.Millisecond)),
		})
		require.NoError(t, err, "creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "panicked", "the journal must record the panic")
		assert.Contains(t, run.Error, "boom", "the journal must carry the panic value")
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
		require.NoError(t, err, "creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "timed out", "the journal must record the timeout")
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
		require.NoError(t, err, "creating the schedule should succeed")

		run := harness.awaitRun(t, schedule.ID, cron.RunFailed)
		assert.Contains(t, run.Error, "timed out",
			"a nil return after the deadline must still journal as a timeout, never as success")
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
		require.NoError(t, err, "creating the schedule should succeed")

		require.Eventually(t, func() bool { return executions.Load() == 1 },
			5*time.Second, 20*time.Millisecond, "the scheduled fire must execute")

		require.NoError(t, harness.manager.TriggerNow(context.Background(), "sync-manual"),
			"the manual fire should be accepted")

		require.Eventually(t, func() bool { return executions.Load() == 2 },
			5*time.Second, 20*time.Millisecond, "the manual fire must execute")
	})
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
	engine := NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(new(captureBus)))
	engine.drainTimeout = 100 * time.Millisecond
	manager := NewScheduleManager(db, true, registry, engine)

	engine.Start()

	schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
		Name:    "sync-blocked",
		JobName: "orders.sync",
		Trigger: cron.Once(time.Now().Add(30 * time.Millisecond)),
	})
	require.NoError(t, err, "creating the schedule should succeed")

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler must start before the engine stops")
	}

	engine.Stop()

	runs := loadRuns(t, db, schedule.ID)
	require.Len(t, runs, 1, "the interrupted fire must stay journaled")
	assert.Equal(t, cron.RunCanceled, runs[0].Status, "shutdown interruption journals as canceled")
	assert.Equal(t, "canceled by shutdown", runs[0].Error, "the journal must name the shutdown")
}
