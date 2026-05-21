package event

import (
	"context"
	"sync"

	"github.com/coldsmirk/vef-framework-go/event"
)

// asyncJob carries a single async publish through the fan-in queue.
// The caller's context is detached (WithoutCancel) so cancellation of
// the originating request does not abort an enqueued event.
type asyncJob struct {
	ctx  context.Context
	evt  event.Event
	opts []event.PublishOption
}

// asyncFanIn drains async publishes through a worker pool. The worker
// count and queue capacity come from config.EventConfig.
//
// Shutdown semantics: shutdown() acquires mu, flips closed, closes the
// queue, and releases mu. Enqueue holds mu while inspecting closed and
// performing the non-blocking send, so the queue cannot be closed
// between the two operations. This is the classic "guard the close +
// send window with a single lock" pattern; the alternative atomic flag
// has an unavoidable check-then-send race.
type asyncFanIn struct {
	queue   chan asyncJob
	workers int
	publish func(ctx context.Context, evt event.Event, opts []event.PublishOption) error
	sink    event.ErrorSink
	wg      sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

func newAsyncFanIn(
	queueSize, workers int,
	publish func(ctx context.Context, evt event.Event, opts []event.PublishOption) error,
	sink event.ErrorSink,
) *asyncFanIn {
	if queueSize <= 0 {
		queueSize = 4096
	}

	if workers <= 0 {
		workers = 4
	}

	return &asyncFanIn{
		queue:   make(chan asyncJob, queueSize),
		workers: workers,
		publish: publish,
		sink:    sink,
	}
}

func (a *asyncFanIn) start() {
	for range a.workers {
		a.wg.Add(1)
		go a.worker()
	}
}

func (a *asyncFanIn) worker() {
	defer a.wg.Done()

	for job := range a.queue {
		if err := a.publish(job.ctx, job.evt, job.opts); err != nil && a.sink != nil {
			a.sink(err, event.Envelope{Type: job.evt.EventType()})
		}
	}
}

// Enqueue offers a job to the fan-in queue without blocking. Returns
// false when the queue is full or the fan-in has been shut down; the
// caller is expected to report the drop via ErrorSink.
func (a *asyncFanIn) Enqueue(job asyncJob) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return false
	}

	select {
	case a.queue <- job:
		return true
	default:
		return false
	}
}

// shutdown stops accepting new jobs, closes the queue so workers drain
// the buffered backlog, and waits for all workers to exit. Honoring
// ctx allows the caller to bound graceful shutdown.
func (a *asyncFanIn) shutdown(ctx context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()

		return nil
	}

	a.closed = true
	close(a.queue)
	a.mu.Unlock()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
