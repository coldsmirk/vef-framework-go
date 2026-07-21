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
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var logger = logx.Named("cron:store")

const (
	// stopTimeout is the graceful drain window on shutdown: how long running
	// handlers get to finish before they are canceled.
	stopTimeout = 30 * time.Second
	// minIdleDelay floors the adaptive sleep so a past-due fire that could
	// not be claimed (full slots, lost race) never spins the loop hot.
	minIdleDelay = 50 * time.Millisecond
	// completionTimeout bounds the journal write after a run finishes. It is
	// deliberately independent of the run's own context — a canceled run
	// still gets journaled.
	completionTimeout = 10 * time.Second
	// outcomeRetryInterval paces outcome-write retries after a lost lock
	// race; completionTimeout bounds the total wait.
	outcomeRetryInterval = 25 * time.Millisecond
	// publisherStopGrace is the event queue's own flush budget on shutdown,
	// reserved even when draining handlers consumed the caller's whole stop
	// window.
	publisherStopGrace = time.Second
	// pruneInterval is the journal retention sweep cadence.
	pruneInterval = time.Hour
	// maxErrorBytes caps journaled failure messages.
	maxErrorBytes = 2000
	// maxClaimBatchesPerTick bounds journal-only progress so recovery and
	// pruning still get a turn under a continuously replenished request queue.
	maxClaimBatchesPerTick = 32
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
	// loopDone closes when the claim loop has left its last tick, so Stop
	// knows no further run can be dispatched.
	loopDone chan struct{}
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
	lastSweep        time.Time
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
		loopDone:     make(chan struct{}),
		slots:        make(chan struct{}, cfg.EffectiveMaxConcurrent()),
		heartbeats:   newHeartbeatTracker(),
		drainTimeout: stopTimeout,
	}
	engine.claimer = &claimer{db: db, config: cfg, registry: registry, nodeID: nodeID, now: now}

	// The engine's own bookkeeping queries (claiming, sweeping, heartbeats,
	// pruning) repeat every poll interval; they log at Debug so steady-state
	// operation stays quiet. Handlers run under runCtx, which is deliberately
	// unmarked — business queries keep their normal log level.
	quiet := orm.WithQuietSQLLog(context.Background())
	engine.loopCtx, engine.stopLoop = context.WithCancel(quiet)
	engine.runCtx, engine.stopRuns = context.WithCancel(context.Background())
	engine.heartbeatCtx, engine.stopHeartbeats = context.WithCancel(quiet)

	return engine
}

// Start launches the claim loop and the heartbeat runner.
func (e *Engine) Start() {
	e.publisher.Start()
	e.background.Add(2)

	go e.loop()
	go e.heartbeatLoop()

	logger.Infof("Durable schedule engine started as node %s (%d handlers)", e.nodeID, len(e.registry.Names()))
}

// Stop drains gracefully: claiming stops immediately, running handlers get
// the drain window to finish, stragglers are canceled and journaled as
// canceled (recoverable schedules re-queue them; see complete). Heartbeats
// outlive handlers so draining runs stay owned. The caller's deadline bounds
// the whole sequence; when necessary, graceful drain is shortened to reserve
// the journal completion window.
func (e *Engine) Stop(ctx context.Context) (stopErr error) {
	stopCtx, cancel := context.WithTimeout(ctx, e.drainTimeout+completionTimeout)
	defer cancel()
	defer func() {
		// The publisher gets a small budget of its own: a drain that consumed
		// the whole stop window must not turn the queue flush into a no-op.
		publisherCtx, cancel := context.WithTimeout(context.WithoutCancel(stopCtx), publisherStopGrace)
		defer cancel()

		stopErr = errors.Join(stopErr, e.publisher.Stop(publisherCtx))
	}()

	// Wait for the loop to actually leave its tick before waiting on the
	// executor group: a claim transaction that has already committed still
	// dispatches its runs, and a group waited on while the counter is zero
	// would return before those runs are even registered.
	e.stopLoop()

	select {
	case <-e.loopDone:
	case <-stopCtx.Done():
		e.stopRuns()
		e.stopHeartbeats()

		return stopCtx.Err()
	}

	drainTimeout := e.drainTimeout
	if deadline, ok := stopCtx.Deadline(); ok {
		drainTimeout = min(drainTimeout, max(time.Until(deadline)-completionTimeout, 0))
	}

	drainCtx, stopDrain := context.WithTimeout(stopCtx, drainTimeout)
	drained := waitWithContext(drainCtx, &e.executors)

	stopDrain()

	if !drained {
		logger.Warn("Drain window elapsed; canceling remaining runs")
		e.stopRuns()

		if !waitWithContext(stopCtx, &e.executors) {
			logger.Error("Shutdown budget elapsed before canceled runs completed; peers will recover them")
			e.stopHeartbeats()

			return stopCtx.Err()
		}
	}

	e.stopRuns()
	e.stopHeartbeats()

	if !waitWithContext(stopCtx, &e.background) {
		return stopCtx.Err()
	}

	logger.Info("Durable schedule engine stopped")

	return nil
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
	defer close(e.loopDone)

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
		timer.Reset(e.nextDelay())
	}
}

// nextDelay bounds the adaptive fire sleep by the sweep cadence, so a distant
// poll interval or a sweep skipped on an explicit wake cannot starve recovery.
func (e *Engine) nextDelay() time.Duration {
	sweepDue := e.sweepInterval() - e.now().Sub(e.lastSweep)

	return max(min(e.idleDelay(), sweepDue), minIdleDelay)
}

// tick claims and dispatches due fires until the store is drained or the
// executor pool is full.
func (e *Engine) tick() {
	for range maxClaimBatchesPerTick {
		free := cap(e.slots) - len(e.slots)

		limit := min(e.config.EffectiveBatchSize(), free)
		if limit == 0 {
			return
		}

		batch, err := e.claimer.claimDueBatch(e.loopCtx, limit)
		if err != nil {
			if e.loopCtx.Err() == nil {
				logger.Errorf("Claim due schedules: %v", err)
			}

			return
		}

		for _, fire := range batch.fires {
			// Never blocks: limit was bounded by the free slots and only
			// this loop acquires them.
			e.slots <- struct{}{}

			e.executors.Add(1)

			go e.execute(fire)
		}

		e.reportAbandoned(batch.abandoned)

		if !batch.progressed {
			return
		}
	}

	// More durable work may remain. Yield to maintenance, then make the next
	// loop iteration immediate instead of waiting for the poll interval.
	e.Wake()
}

// maintain runs the recovery sweep and the journal prune, each on its own
// cadence — a wake storm (short tasks completing, request backlogs) must not
// multiply maintenance queries.
func (e *Engine) maintain() {
	if now := e.now(); now.Sub(e.lastSweep) >= e.sweepInterval() {
		e.lastSweep = now
		e.sweepAbandoned(e.loopCtx)
	}

	if e.config.RunRetention > 0 && e.now().Sub(e.lastJournalPrune) >= pruneInterval {
		e.lastJournalPrune = e.now()
		e.pruneJournal(e.loopCtx)
	}
}

// sweepInterval is the recovery sweep cadence: half the abandoned window,
// which keeps takeover latency well inside the staleness contract. Sweeping
// faster finds nothing — a run cannot be stale before the whole window
// elapsed — so the poll interval deliberately does not shorten it; nextDelay
// consumes this cadence to keep a distant poll interval from starving
// recovery, never to accelerate it.
func (e *Engine) sweepInterval() time.Duration {
	return e.config.EffectiveAbandonedAfter() / 2
}

// idleDelay computes the adaptive sleep: until the nearest known fire,
// floored against hot-looping and capped by the poll interval.
func (e *Engine) idleDelay() time.Duration {
	poll := e.config.EffectivePollInterval()
	if e.registry.IsEmpty() {
		return poll
	}

	// With every executor slot taken there is nothing to claim: sleep the
	// full poll interval instead of spinning on the minimum floor — a slot
	// release wakes the loop explicitly (see execute).
	if len(e.slots) == cap(e.slots) {
		return poll
	}

	var nextUnixMs int64

	err := e.db.NewSelect().
		Model((*cron.Schedule)(nil)).
		Select("next_fire_at_unix_ms").
		Where(func(cb orm.ConditionBuilder) {
			cb.IsTrue("is_enabled").
				IsNotNull("next_fire_at_unix_ms").
				In("job_name", e.registry.Names())
		}).
		OrderBy("next_fire_at_unix_ms").
		Limit(1).
		Scan(e.loopCtx, &nextUnixMs)
	if err != nil {
		if e.loopCtx.Err() == nil && !result.IsRecordNotFound(err) {
			logger.Errorf("Read nearest fire: %v", err)
		}

		return poll
	}

	delay := unixTime(nextUnixMs).Sub(e.now())

	return max(min(delay, poll), minIdleDelay)
}

// execute runs one claimed fire on the executor pool.
func (e *Engine) execute(fire claimedFire) {
	defer e.executors.Done()
	defer func() {
		<-e.slots
		e.Wake()
	}()

	e.heartbeats.Track(fire.run.ID)
	defer e.heartbeats.Untrack(fire.run.ID)

	ctx := e.runCtx

	if fire.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, fire.timeout)

		defer cancel()
	}

	runErr := e.invoke(ctx, fire)

	// The context error is sampled here, before the deferred cancel fires:
	// the question the journal needs answered is whether the run's own
	// context was already dead when the handler returned.
	e.complete(fire, runErr, ctx.Err())
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
		ScheduledAt:  unixTime(fire.run.ScheduledAtUnixMs),
		Params:       fire.schedule.Params,
	})
}

// complete journals the run's outcome. The write context is independent of
// the run's — a canceled run still gets journaled — and guarded on the row
// still being running, so a recovery sweep that already took the run over
// wins and the late completion is only logged. A canceled run of a
// recoverable schedule re-queues in the same transaction: graceful shutdown
// must not deliver weaker semantics than a crash, whose abandoned rows the
// sweep re-fires.
//
// ctxErr is the run context's state at the moment the handler returned. It
// outranks the handler's own return value: a handler that returns nil after
// its deadline passed did not finish the work it was given, and journaling
// that as success would hide every timeout the operator configured.
func (e *Engine) complete(fire claimedFire, runErr, ctxErr error) {
	ctx, cancel := context.WithTimeout(orm.WithQuietSQLLog(context.Background()), completionTimeout)
	defer cancel()

	run := fire.run
	now := e.now()
	run.FinishedAtUnixMs = unixMillisPtr(now)

	if run.StartedAtUnixMs != nil {
		run.DurationMs = now.Sub(unixTime(*run.StartedAtUnixMs)).Milliseconds()
	}

	switch {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		run.Status = cron.RunFailed
		run.Error = trimError(fmt.Errorf("%w after %s", ErrRunTimedOut, fire.timeout))

	// The run context derives only from the shutdown context and the optional
	// deadline, so any remaining error is a shutdown cancellation.
	case ctxErr != nil:
		run.Status = cron.RunCanceled
		run.Error = "canceled by shutdown"

	case runErr != nil:
		run.Status = cron.RunFailed
		run.Error = trimError(runErr)

	default:
		run.Status = cron.RunSucceeded
	}

	if err := e.writeOutcome(ctx, fire, run); err != nil {
		logger.Errorf("Journal run %s of schedule %q: %v", run.ID, run.ScheduleName, err)

		return
	}

	if run.Status == cron.RunFailed {
		logger.Errorf("Run %s of schedule %q failed: %s", run.ID, run.ScheduleName, run.Error)
		e.publisher.RunFailed(run)
	}
}

// writeOutcome persists the run's terminal state, retrying lost lock races
// until the completion window closes. The transaction is idempotent — the
// status CAS guards the update and the recovery re-queue is fenced by the
// run ID — and a run left running because its outcome write gave up would
// wrongly resurface through abandoned-run recovery, so unlike claiming
// (where the next tick retries naturally) this write must not surrender to
// a transient writer collision on SQLite's single-writer path.
func (e *Engine) writeOutcome(ctx context.Context, fire claimedFire, run *cron.Run) error {
	for {
		err := e.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
			updated, err := tx.NewUpdate().
				Model(run).
				Select("status", "finished_at_unix_ms", "duration_ms", "error").
				Where(func(cb orm.ConditionBuilder) {
					cb.PKEquals(run.ID).Equals("status", cron.RunRunning)
				}).
				Exec(ctx)
			if err != nil {
				return err
			}

			if affected, _ := updated.RowsAffected(); affected == 0 {
				logger.Warnf("Run %s of schedule %q finished after being recovered; outcome discarded", run.ID, run.ScheduleName)

				return nil
			}

			if run.Status == cron.RunCanceled && fire.schedule.Recover {
				// The run ID fences the re-queue exactly like an abandoned
				// takeover's, so the request stays unique however often this
				// path could repeat.
				request := &fireRequest{
					ScheduleID:        run.ScheduleID,
					Kind:              fireRequestRecovery,
					ScheduledAtUnixMs: run.ScheduledAtUnixMs,
					SourceRunID:       run.ID,
				}
				if _, err := tx.NewInsert().Model(request).Exec(ctx); err != nil {
					return fmt.Errorf("queue recovery for canceled run: %w", err)
				}
			}

			return nil
		})
		if err == nil || !sqlmigration.IsBusyContention(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return err
		case <-time.After(outcomeRetryInterval):
		}
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

// waitWithContext waits for the group until the caller's shutdown budget ends.
func waitWithContext(ctx context.Context, group *sync.WaitGroup) bool {
	done := make(chan struct{})

	go func() {
		group.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}
