package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var logger = logx.Named("cron:store")

const (
	// stopTimeout is the graceful drain window on shutdown: first for
	// running handlers to finish, then — after cancellation — to observe it.
	stopTimeout = 30 * time.Second
	// minIdleDelay floors the adaptive sleep so a past-due fire that could
	// not be claimed (full slots, lost race) never spins the loop hot.
	minIdleDelay = 50 * time.Millisecond
	// completionTimeout bounds the journal write after a run finishes. It is
	// deliberately independent of the run's own context — a canceled run
	// still gets journaled.
	completionTimeout = 10 * time.Second
	// pruneInterval is the journal retention sweep cadence.
	pruneInterval = time.Hour
	// maxErrorBytes caps journaled failure messages.
	maxErrorBytes = 2000
)

// Engine is the durable scheduler: it polls the store adaptively, claims due
// fires, executes them on a bounded pool, heartbeats running rows, recovers
// abandoned ones, and prunes the journal by retention. Every node runs one
// engine; the claim transaction is what keeps each fire single-noded.
type Engine struct {
	db        orm.DB
	config    *config.CronStoreConfig
	registry  *Registry
	claimer   *claimer
	publisher *RunEventPublisher
	nodeID    string
	now       func() time.Time

	// wake nudges the loop out of its adaptive sleep after a local schedule
	// mutation; buffered so signaling never blocks.
	wake chan struct{}
	// slots is the executor pool: one token per in-flight run.
	slots chan struct{}

	heartbeats *heartbeatTracker

	// loopCtx bounds claiming and maintenance; runCtx bounds handlers and is
	// canceled only after the drain grace; heartbeatCtx outlives both so
	// draining runs keep heartbeating.
	loopCtx          context.Context
	stopLoop         context.CancelFunc
	runCtx           context.Context
	stopRuns         context.CancelFunc
	heartbeatCtx     context.Context
	stopHeartbeats   context.CancelFunc
	executors        sync.WaitGroup
	background       sync.WaitGroup
	drainTimeout     time.Duration
	lastJournalPrune time.Time
}

// NewEngine builds the engine; Start launches it.
func NewEngine(db orm.DB, cfg *config.CronStoreConfig, registry *Registry, publisher *RunEventPublisher) *Engine {
	nodeID := newNodeID()
	now := func() time.Time { return timex.Now().Unwrap() }

	engine := &Engine{
		db:           db,
		config:       cfg,
		registry:     registry,
		publisher:    publisher,
		nodeID:       nodeID,
		now:          now,
		wake:         make(chan struct{}, 1),
		slots:        make(chan struct{}, cfg.EffectiveMaxConcurrent()),
		heartbeats:   newHeartbeatTracker(),
		drainTimeout: stopTimeout,
	}
	engine.claimer = &claimer{db: db, config: cfg, registry: registry, nodeID: nodeID, now: now}
	engine.loopCtx, engine.stopLoop = context.WithCancel(context.Background())
	engine.runCtx, engine.stopRuns = context.WithCancel(context.Background())
	engine.heartbeatCtx, engine.stopHeartbeats = context.WithCancel(context.Background())

	return engine
}

// Start launches the claim loop and the heartbeat runner.
func (e *Engine) Start() {
	e.background.Add(2)

	go e.loop()
	go e.heartbeatLoop()

	logger.Infof("Durable schedule engine started as node %s (%d handlers)", e.nodeID, len(e.registry.Names()))
}

// Stop drains gracefully: claiming stops immediately, running handlers get
// the drain window to finish, stragglers are canceled and journaled as
// canceled. Heartbeats outlive handlers so draining runs stay owned.
func (e *Engine) Stop() {
	e.stopLoop()

	if !waitWithTimeout(&e.executors, e.drainTimeout) {
		logger.Warn("Drain window elapsed; canceling remaining runs")
		e.stopRuns()

		if !waitWithTimeout(&e.executors, e.drainTimeout) {
			logger.Error("Runs ignored cancellation; abandoning wait — peers will recover them")
		}
	}

	e.stopRuns()
	e.stopHeartbeats()
	e.background.Wait()

	logger.Info("Durable schedule engine stopped")
}

// Wake nudges the loop to re-read the store now — called after local
// schedule mutations so a nearer fire does not wait out the current sleep.
func (e *Engine) Wake() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// loop is the scheduling heart: claim everything due, maintain, then sleep
// until the nearest known fire (capped by the poll interval, which is also
// the visibility bound for schedules created on other nodes).
func (e *Engine) loop() {
	defer e.background.Done()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-e.loopCtx.Done():
			return

		case <-timer.C:

		case <-e.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}

		e.tick()
		e.maintain()
		timer.Reset(e.idleDelay())
	}
}

// tick claims and dispatches due fires until the store is drained or the
// executor pool is full.
func (e *Engine) tick() {
	for {
		free := cap(e.slots) - len(e.slots)

		limit := min(e.config.EffectiveBatchSize(), free)
		if limit == 0 {
			return
		}

		claimed, err := e.claimer.ClaimDue(e.loopCtx, limit)
		if err != nil {
			if e.loopCtx.Err() == nil {
				logger.Errorf("Claim due schedules: %v", err)
			}

			return
		}

		for _, fire := range claimed {
			// Never blocks: limit was bounded by the free slots and only
			// this loop acquires them.
			e.slots <- struct{}{}

			e.executors.Add(1)

			go e.execute(fire)
		}

		if len(claimed) < limit {
			return
		}
	}
}

// maintain runs the recovery sweep every tick and the journal prune on its
// hourly cadence.
func (e *Engine) maintain() {
	e.sweepAbandoned(e.loopCtx)

	if e.config.RunRetention > 0 && e.now().Sub(e.lastJournalPrune) >= pruneInterval {
		e.lastJournalPrune = e.now()
		e.pruneJournal(e.loopCtx)
	}
}

// idleDelay computes the adaptive sleep: until the nearest known fire,
// floored against hot-looping and capped by the poll interval.
func (e *Engine) idleDelay() time.Duration {
	poll := e.config.EffectivePollInterval()
	if e.registry.IsEmpty() {
		return poll
	}

	var next timex.DateTime

	err := e.db.NewSelect().
		Model((*cron.Schedule)(nil)).
		Select("next_fire_at").
		Where(func(cb orm.ConditionBuilder) {
			cb.IsTrue("is_enabled").
				IsNotNull("next_fire_at").
				In("job_name", e.registry.Names())
		}).
		OrderBy("next_fire_at").
		Limit(1).
		Scan(e.loopCtx, &next)
	if err != nil {
		if e.loopCtx.Err() == nil && !result.IsRecordNotFound(err) && !errors.Is(err, context.Canceled) {
			logger.Errorf("Read nearest fire: %v", err)
		}

		return poll
	}

	delay := next.AsLocal().Sub(e.now())

	return max(min(delay, poll), minIdleDelay)
}

// execute runs one claimed fire on the executor pool.
func (e *Engine) execute(fire claimedFire) {
	defer e.executors.Done()
	defer func() { <-e.slots }()

	e.heartbeats.Track(fire.run.ID)
	defer e.heartbeats.Untrack(fire.run.ID)

	ctx := e.runCtx

	if fire.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, fire.timeout)

		defer cancel()
	}

	e.complete(fire, e.invoke(ctx, fire))
}

// invoke calls the handler, converting a panic into an error so one bad job
// can never take the executor down.
func (*Engine) invoke(ctx context.Context, fire claimedFire) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: job %q: %v", ErrJobPanicked, fire.schedule.JobName, recovered)
		}
	}()

	return fire.handler.Execute(ctx, cron.Execution{
		RunID:        fire.run.ID,
		ScheduleID:   fire.schedule.ID,
		ScheduleName: fire.schedule.Name,
		JobName:      fire.schedule.JobName,
		ScheduledAt:  fire.run.ScheduledAt.Unwrap(),
		Params:       fire.schedule.Params,
	})
}

// complete journals the run's outcome. The write context is independent of
// the run's — a canceled run still gets journaled — and guarded on the row
// still being running, so a recovery sweep that already took the run over
// wins and the late completion is only logged.
func (e *Engine) complete(fire claimedFire, runErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()

	run := fire.run
	now := e.now()
	finished := timex.DateTime(now)
	run.FinishedAt = &finished

	if run.StartedAt != nil {
		run.DurationMs = now.Sub(run.StartedAt.Unwrap()).Milliseconds()
	}

	switch {
	case runErr == nil:
		run.Status = cron.RunSucceeded

	case e.runCtx.Err() != nil && errors.Is(runErr, context.Canceled):
		run.Status = cron.RunCanceled
		run.Error = "canceled by shutdown"

	default:
		run.Status = cron.RunFailed
		run.Error = trimError(runErr)
	}

	updated, err := e.db.NewUpdate().
		Model(run).
		Select("status", "finished_at", "duration_ms", "error").
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(run.ID).Equals("status", cron.RunRunning)
		}).
		Exec(ctx)
	if err != nil {
		logger.Errorf("Journal run %s of schedule %q: %v", run.ID, run.ScheduleName, err)

		return
	}

	if affected, _ := updated.RowsAffected(); affected == 0 {
		logger.Warnf("Run %s of schedule %q finished after being recovered; outcome discarded", run.ID, run.ScheduleName)

		return
	}

	if run.Status == cron.RunFailed {
		logger.Errorf("Run %s of schedule %q failed: %s", run.ID, run.ScheduleName, run.Error)
		e.publisher.RunFailed(ctx, run)
	}
}

// trimError normalizes a failure into a bounded, valid-UTF-8 journal message.
func trimError(err error) string {
	message := strings.TrimSpace(strings.ToValidUTF8(err.Error(), ""))
	if len(message) > maxErrorBytes {
		message = strings.ToValidUTF8(message[:maxErrorBytes], "")
	}

	return message
}

// waitWithTimeout waits for the group up to the given duration.
func waitWithTimeout(group *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})

	go func() {
		group.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
