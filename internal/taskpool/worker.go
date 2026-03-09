package taskpool

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coldsmirk/vef-framework-go/log"
)

// WorkerDelegate defines pluggable task execution logic.
// Each worker owns its delegate instance, all methods run in worker's OS thread.
type WorkerDelegate[TIn, TOut any] interface {
	// Init is called once when worker starts.
	Init(ctx context.Context, config any) error

	// Execute runs for each task. Must respect context cancellation.
	Execute(ctx context.Context, payload TIn) (TOut, error)

	// Destroy is called once when worker stops.
	Destroy() error

	// HealthCheck is called periodically.
	HealthCheck() error
}

type workerState int

const (
	workerStateIdle     workerState = 0
	workerStateRunning  workerState = 1
	workerStateStopping workerState = 2
	workerStateStopped  workerState = 3
)

// Worker executes tasks in a dedicated OS thread.
type Worker[TIn, TOut any] struct {
	id       int
	pool     *WorkerPool[TIn, TOut]
	delegate WorkerDelegate[TIn, TOut]
	logger   log.Logger

	state      workerState
	stateMu    sync.RWMutex
	lastActive atomic.Value

	stopCh   chan struct{}
	initDone chan error

	tasksExecuted atomic.Uint64
}

func newWorker[TIn, TOut any](id int, pool *WorkerPool[TIn, TOut]) *Worker[TIn, TOut] {
	w := &Worker[TIn, TOut]{
		id:       id,
		pool:     pool,
		delegate: pool.config.DelegateFactory(),
		logger:   pool.logger.Named(fmt.Sprintf("worker-%d", id)),
		state:    workerStateIdle,
		stopCh:   make(chan struct{}),
		initDone: make(chan error, 1),
	}
	w.lastActive.Store(time.Now())

	return w
}

// run is the main worker loop, locks to OS thread for delegate compatibility.
func (w *Worker[TIn, TOut]) run() {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	if err := w.initDelegate(); err != nil {
		return
	}

	w.logger.Debug("worker started")
	defer w.cleanup()

	w.processLoop()
}

func (w *Worker[TIn, TOut]) initDelegate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.delegate.Init(ctx, w.pool.config.DelegateConfig); err != nil {
		w.logger.Errorf("delegate init failed: %v", err)
		w.setState(workerStateStopped)

		w.initDone <- err

		return err
	}

	w.initDone <- nil

	return nil
}

func (w *Worker[TIn, TOut]) cleanup() {
	if err := w.delegate.Destroy(); err != nil {
		w.logger.Errorf("delegate destroy failed: %v", err)
	}

	w.setState(workerStateStopped)
	w.logger.Debug("worker stopped")
}

func (w *Worker[TIn, TOut]) processLoop() {
	idleCheckTicker := time.NewTicker(1 * time.Second)
	defer idleCheckTicker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-idleCheckTicker.C:
			if w.shouldStopDueToIdle() {
				return
			}
		default:
			if task := w.fetchTask(); task != nil {
				w.executeTask(task)
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

// fetchTask retrieves next task by priority: High -> Medium -> Low.
func (w *Worker[TIn, TOut]) fetchTask() *Task[TIn, TOut] {
	// Try high priority first
	select {
	case task := <-w.pool.highQueue:
		return task
	default:
	}

	// Then medium priority
	select {
	case task := <-w.pool.mediumQueue:
		return task
	default:
	}

	// Finally low priority with timeout
	select {
	case task := <-w.pool.lowQueue:
		return task
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}

func (w *Worker[TIn, TOut]) executeTask(task *Task[TIn, TOut]) {
	w.setState(workerStateRunning)
	defer func() {
		w.setState(workerStateIdle)
		w.lastActive.Store(time.Now())

		if task.Done != nil {
			close(task.Done)
		}
	}()

	start := time.Now()

	w.tasksExecuted.Add(1)

	w.pool.stats.activeWorkers.Add(1)
	defer w.pool.stats.activeWorkers.Add(-1)

	result := w.execute(task)
	result.Duration = time.Since(start)

	if result.Error != nil {
		w.pool.stats.totalFailed.Add(1)
	} else {
		w.pool.stats.totalCompleted.Add(1)
	}

	if task.Result != nil {
		select {
		case task.Result <- result:
		case <-task.Context.Done():
		}
	}

	w.logger.Debugf("task completed: id=%s, duration=%v, error=%v",
		task.ID, result.Duration, result.Error)
}

func (w *Worker[TIn, TOut]) execute(task *Task[TIn, TOut]) Result[TOut] {
	ctx, cancel := w.createTaskContext(task.Context)
	if cancel != nil {
		defer cancel()
	}

	data, err := w.delegate.Execute(ctx, task.Payload)

	return Result[TOut]{
		TaskID: task.ID,
		Data:   data,
		Error:  err,
	}
}

func (w *Worker[TIn, TOut]) createTaskContext(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline, hasDeadline := ctx.Deadline()

	if !hasDeadline {
		return context.WithTimeout(ctx, w.pool.config.TaskTimeout)
	}

	if time.Until(deadline) > w.pool.config.MaxTaskTimeout {
		return context.WithTimeout(context.Background(), w.pool.config.TaskTimeout)
	}

	return ctx, nil
}

func (w *Worker[TIn, TOut]) shouldStopDueToIdle() bool {
	if w.pool.config.IdleTimeout == 0 {
		return false
	}

	lastActive := w.lastActive.Load().(time.Time)
	if time.Since(lastActive) < w.pool.config.IdleTimeout {
		return false
	}

	w.pool.workerMu.RLock()
	canStop := len(w.pool.workers) > w.pool.config.MinWorkers
	w.pool.workerMu.RUnlock()

	if canStop {
		w.logger.Infof("worker stopping due to idle timeout: idle_duration=%v",
			time.Since(lastActive))
		w.pool.removeWorker(w)
	}

	return canStop
}

func (w *Worker[TIn, TOut]) stop() {
	w.setState(workerStateStopping)
	close(w.stopCh)
}

func (w *Worker[TIn, TOut]) setState(state workerState) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()

	w.state = state
}
