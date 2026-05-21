package event

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
)

type AsyncTestEvent struct{ ID int }

func (*AsyncTestEvent) EventType() string { return "async.test" }

func TestAsyncFanInDrainsQueueOnShutdown(t *testing.T) {
	var (
		processed atomic.Int64
		ready     = make(chan struct{})
		release   = make(chan struct{})
	)

	publish := func(_ context.Context, _ event.Event, _ []event.PublishOption) error {
		// Park the first worker so the queue fills up before shutdown.
		select {
		case <-ready:
		default:
			close(ready)
			<-release
		}

		processed.Add(1)

		return nil
	}

	a := newAsyncFanIn(8, 1, publish, nil)
	a.start()

	for range 5 {
		require.True(t, a.Enqueue(asyncJob{ctx: context.Background(), evt: &AsyncTestEvent{}}),
			"queue capacity should easily hold 5 jobs")
	}

	<-ready
	close(release)

	require.NoError(t, a.shutdown(context.Background()), "shutdown should drain the queue")
	require.EqualValues(t, 5, processed.Load(), "every enqueued job must be processed before shutdown returns")
}

func TestAsyncFanInEnqueueAfterShutdownReturnsFalse(t *testing.T) {
	a := newAsyncFanIn(4, 1, func(context.Context, event.Event, []event.PublishOption) error { return nil }, nil)
	a.start()

	require.NoError(t, a.shutdown(context.Background()))
	require.False(t, a.Enqueue(asyncJob{ctx: context.Background(), evt: &AsyncTestEvent{}}),
		"Enqueue must refuse new work once shutdown completes")
}

func TestAsyncFanInShutdownIsIdempotent(t *testing.T) {
	a := newAsyncFanIn(4, 2, func(context.Context, event.Event, []event.PublishOption) error { return nil }, nil)
	a.start()

	require.NoError(t, a.shutdown(context.Background()))
	require.NoError(t, a.shutdown(context.Background()), "second shutdown must be a no-op")
}

func TestAsyncFanInSinkInvokedOnPublishError(t *testing.T) {
	var sinkCount atomic.Int64

	expected := errors.New("publish boom")
	publish := func(context.Context, event.Event, []event.PublishOption) error { return expected }
	sink := func(err error, _ event.Envelope) {
		if errors.Is(err, expected) {
			sinkCount.Add(1)
		}
	}

	a := newAsyncFanIn(4, 1, publish, sink)
	a.start()

	require.True(t, a.Enqueue(asyncJob{ctx: context.Background(), evt: &AsyncTestEvent{}}))

	require.NoError(t, a.shutdown(context.Background()))
	require.EqualValues(t, 1, sinkCount.Load(), "ErrorSink should observe one publish failure")
}

func TestAsyncFanInConcurrentEnqueueAndShutdown(t *testing.T) {
	// Race regression test: the fix replaced atomic.Bool with mutex
	// coverage of the (check-closed → send) window. Hammer Enqueue while
	// shutdown runs to prove the channel close cannot race with a send.
	a := newAsyncFanIn(64, 4, func(context.Context, event.Event, []event.PublishOption) error { return nil }, nil)
	a.start()

	var wg sync.WaitGroup

	wg.Add(8)

	for range 8 {
		go func() {
			defer wg.Done()

			deadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(deadline) {
				_ = a.Enqueue(asyncJob{ctx: context.Background(), evt: &AsyncTestEvent{}})
			}
		}()
	}

	// Give producers a head start, then shutdown concurrently.
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, a.shutdown(context.Background()))

	wg.Wait()
}
