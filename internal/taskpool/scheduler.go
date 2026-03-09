package taskpool

import (
	"context"
	"time"
)

// Scheduler manages task submission and execution.
type Scheduler[TIn, TOut any] interface {
	// Submit blocks until task completes or context is canceled.
	Submit(ctx context.Context, payload TIn, opts ...SubmitOption) (Result[TOut], error)

	// SubmitAsync returns immediately with a result channel.
	SubmitAsync(ctx context.Context, payload TIn, opts ...SubmitOption) (<-chan Result[TOut], error)

	// Stats returns current scheduler statistics (total submitted, completed, and failed counts).
	Stats() SchedulerStats

	// Shutdown waits for running tasks to complete.
	Shutdown(ctx context.Context) error
}

type SchedulerStats struct {
	TotalSubmitted uint64
	TotalCompleted uint64
	TotalFailed    uint64
	ActiveWorkers  int
	IdleWorkers    int
	TotalWorkers   int
	QueuedTasks    int
}

type SubmitOption func(*submitOptions)

type submitOptions struct {
	priority Priority
}

func WithPriority(p Priority) SubmitOption {
	return func(o *submitOptions) {
		o.priority = p
	}
}

func parseSubmitOptions(opts []SubmitOption) submitOptions {
	options := submitOptions{priority: PriorityMedium}
	for _, opt := range opts {
		opt(&options)
	}

	return options
}

type DefaultScheduler[TIn, TOut any] struct {
	pool *WorkerPool[TIn, TOut]
}

func New[TIn, TOut any](config Config[TIn, TOut]) (Scheduler[TIn, TOut], error) {
	pool, err := newWorkerPool(config)
	if err != nil {
		return nil, err
	}

	return &DefaultScheduler[TIn, TOut]{pool: pool}, nil
}

func (s *DefaultScheduler[TIn, TOut]) Submit(ctx context.Context, payload TIn, opts ...SubmitOption) (Result[TOut], error) {
	options := parseSubmitOptions(opts)
	resultCh := make(chan Result[TOut], 1)
	doneCh := make(chan struct{})

	task := &Task[TIn, TOut]{
		ID:          generateTaskID(),
		Context:     ctx,
		Priority:    options.priority,
		Payload:     payload,
		Result:      resultCh,
		Done:        doneCh,
		SubmittedAt: time.Now(),
	}

	if err := s.pool.submit(task); err != nil {
		close(resultCh)
		close(doneCh)

		return Result[TOut]{}, err
	}

	select {
	case result := <-resultCh:
		return result, nil
	case <-ctx.Done():
		return Result[TOut]{}, ctx.Err()
	}
}

func (s *DefaultScheduler[TIn, TOut]) SubmitAsync(ctx context.Context, payload TIn, opts ...SubmitOption) (<-chan Result[TOut], error) {
	options := parseSubmitOptions(opts)
	resultCh := make(chan Result[TOut], 1)
	doneCh := make(chan struct{})

	task := &Task[TIn, TOut]{
		ID:          generateTaskID(),
		Context:     ctx,
		Priority:    options.priority,
		Payload:     payload,
		Result:      resultCh,
		Done:        doneCh,
		SubmittedAt: time.Now(),
	}

	if err := s.pool.submit(task); err != nil {
		close(resultCh)
		close(doneCh)

		return nil, err
	}

	return resultCh, nil
}

func (s *DefaultScheduler[TIn, TOut]) Stats() SchedulerStats {
	return s.pool.getStats()
}

func (s *DefaultScheduler[TIn, TOut]) Shutdown(ctx context.Context) error {
	return s.pool.Shutdown(ctx)
}
