package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	imemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
)

// makeFrame is a helper that returns a Frame with the given event type.
func makeFrame(eventType string) transport.Frame {
	return transport.Frame{ID: "1", Type: eventType, Body: []byte(`{}`)}
}

// drainOne waits for a single signal on ch, failing the test on timeout.
func drainOne(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal(msg)
	}
}

// TestStopCancelsInFlightHandler asserts the transport-scoped context
// added to the memory transport reaches an in-flight handler. Before
// the fix the handler received context.Background(), so Stop had no
// way to unwind a handler that was blocked on downstream I/O — the
// only escape was the per-subscription stopCh which fires only between
// loop iterations, not inside the consume callback.
func TestStopCancelsInFlightHandler(t *testing.T) {
	tp := imemory.New(memory.Config{QueueSize: 1, FullPolicy: memory.FullPolicyError})
	require.NoError(t, tp.Start(context.Background()), "Start should succeed")

	entered := make(chan struct{}, 1)
	exited := make(chan error, 1)

	_, err := tp.Subscribe("memory.cancel", "g", func(ctx context.Context, _ transport.Delivery) error {
		entered <- struct{}{}

		<-ctx.Done()

		exited <- ctx.Err()

		return nil
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err, "Subscribe should succeed")

	require.NoError(t, tp.Publish(context.Background(), []transport.Frame{{
		ID:   "1",
		Type: "memory.cancel",
		Body: []byte(`{}`),
	}}), "Publish should succeed")

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("Handler should have entered the consume callback")
	}

	require.NoError(t, tp.Stop(context.Background()), "Stop should drain handler cleanly")

	select {
	case ctxErr := <-exited:
		require.ErrorIs(t, ctxErr, context.Canceled,
			"Handler must observe the transport-scoped cancel signal")
	case <-time.After(time.Second):
		t.Fatal("Handler should have unwound once the transport canceled its context")
	}
}

// TestPublishFanOutContinuesOnFullQueue verifies that when one subscriber's
// queue is full (FullPolicyError), Publish still delivers to all other
// subscribers and returns a non-nil error that IsQueueFull detects.
func TestPublishFanOutContinuesOnFullQueue(t *testing.T) {
	const eventType = "memory.fanout"

	// Queue size of 1 so the "full" subscriber saturates deterministically:
	// its single worker dequeues the first frame and blocks in the handler,
	// the next frame fills the 1-slot queue, and the third overflows.
	tp := imemory.New(memory.Config{QueueSize: 1, FullPolicy: memory.FullPolicyError})
	require.NoError(t, tp.Start(context.Background()), "Start should succeed")

	unblock := make(chan struct{})
	// Always release the blocked handler before Stop drains, even on a
	// failed assertion, so the test can never deadlock on shutdown.
	t.Cleanup(func() {
		close(unblock)

		_ = tp.Stop(context.Background())
	})

	// Healthy subscriber — large buffer, drains immediately.
	healthyReceived := make(chan struct{}, 10)
	_, err := tp.Subscribe(eventType, "", func(_ context.Context, _ transport.Delivery) error {
		healthyReceived <- struct{}{}

		return nil
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err, "Subscribe healthy should succeed")

	// Full subscriber — its single worker blocks inside the handler until
	// unblocked, but also honors ctx so Stop can always drain it.
	entered := make(chan struct{}, 4)
	_, err = tp.Subscribe(eventType, "", func(ctx context.Context, _ transport.Delivery) error {
		entered <- struct{}{}

		select {
		case <-unblock:
		case <-ctx.Done():
		}

		return nil
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err, "Subscribe full should succeed")

	// Frame 1: the full subscriber's worker dequeues it and blocks in the
	// handler, leaving its 1-slot queue empty again.
	require.NoError(t, tp.Publish(context.Background(), []transport.Frame{makeFrame(eventType)}),
		"first publish should succeed")
	drainOne(t, entered, "full subscriber handler should have started")
	drainOne(t, healthyReceived, "healthy subscriber did not receive frame 1")

	// Frame 2: fills the full subscriber's 1-slot queue (worker still blocked).
	require.NoError(t, tp.Publish(context.Background(), []transport.Frame{makeFrame(eventType)}),
		"second publish should succeed and fill the full subscriber's queue")
	drainOne(t, healthyReceived, "healthy subscriber did not receive frame 2")

	// Frame 3: the full subscriber's queue is saturated, so its enqueue
	// fails — but fan-out must still reach the healthy subscriber, and the
	// returned error must be detectable as queue-full.
	pubErr := tp.Publish(context.Background(), []transport.Frame{makeFrame(eventType)})
	require.Error(t, pubErr, "Publish should return an error when one subscriber's queue is full")
	require.True(t, imemory.IsQueueFull(pubErr), "error must be detectable as queue-full via IsQueueFull")
	drainOne(t, healthyReceived, "healthy subscriber must still receive frame 3 despite the full subscriber")
}

// TestPublishContinuesToLaterFramesAcrossBatch verifies that a hard
// enqueue failure on the FIRST frame of a multi-frame batch does not
// suppress delivery of the LATER frames to other, healthy subscribers.
// Before the fix, Publish returned after the first frame's errors.Join,
// so one saturated subscriber head-of-line-blocked the rest of the batch.
//
// The three frames target distinct event types so each healthy
// subscriber receives exactly one frame (a 1-slot queue is sufficient),
// isolating the cross-frame-continuation behavior from per-subscriber
// queue saturation.
func TestPublishContinuesToLaterFramesAcrossBatch(t *testing.T) {
	const (
		blockedType = "memory.batch.blocked"
		typeA       = "memory.batch.a"
		typeB       = "memory.batch.b"
	)

	tp := imemory.New(memory.Config{QueueSize: 1, FullPolicy: memory.FullPolicyError})
	require.NoError(t, tp.Start(context.Background()), "Start should succeed")

	unblock := make(chan struct{})
	t.Cleanup(func() {
		close(unblock)

		_ = tp.Stop(context.Background())
	})

	// Saturated subscriber on blockedType: its single worker blocks in the
	// handler so its 1-slot queue fills and the next enqueue hard-fails.
	entered := make(chan struct{}, 2)
	_, err := tp.Subscribe(blockedType, "", func(ctx context.Context, _ transport.Delivery) error {
		entered <- struct{}{}

		select {
		case <-unblock:
		case <-ctx.Done():
		}

		return nil
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err, "Subscribe blocked should succeed")

	// Healthy subscribers on the later frames' types.
	gotA := make(chan struct{}, 1)
	_, err = tp.Subscribe(typeA, "", func(_ context.Context, _ transport.Delivery) error {
		gotA <- struct{}{}

		return nil
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err, "Subscribe A should succeed")

	gotB := make(chan struct{}, 1)
	_, err = tp.Subscribe(typeB, "", func(_ context.Context, _ transport.Delivery) error {
		gotB <- struct{}{}

		return nil
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err, "Subscribe B should succeed")

	// Saturate the blocked subscriber: one frame parks its worker, a second
	// fills its 1-slot queue so any further enqueue fails immediately.
	require.NoError(t, tp.Publish(context.Background(), []transport.Frame{makeFrame(blockedType)}),
		"priming publish should park the blocked subscriber's worker")
	drainOne(t, entered, "blocked subscriber handler should have started")
	require.NoError(t, tp.Publish(context.Background(), []transport.Frame{makeFrame(blockedType)}),
		"priming publish should fill the blocked subscriber's 1-slot queue")

	// Single batch: frame[0] (blockedType) hard-fails, but frames[1:] must
	// still reach the healthy A/B subscribers and the error stays queue-full.
	pubErr := tp.Publish(context.Background(), []transport.Frame{
		makeFrame(blockedType),
		makeFrame(typeA),
		makeFrame(typeB),
	})
	require.Error(t, pubErr, "Publish must surface the saturated subscriber's failure")
	require.True(t, imemory.IsQueueFull(pubErr), "Batch error must remain detectable as queue-full")

	drainOne(t, gotA, "later frame A must be delivered despite the earlier frame's failure")
	drainOne(t, gotB, "later frame B must be delivered despite the earlier frame's failure")
}

// TestSubscribeAfterStopReturnsBusStopped verifies that Subscribe called
// after Stop returns ErrBusStopped and does not leak subscription goroutines.
func TestSubscribeAfterStopReturnsBusStopped(t *testing.T) {
	tp := imemory.New(memory.Config{})
	require.NoError(t, tp.Start(context.Background()), "Start should succeed")
	require.NoError(t, tp.Stop(context.Background()), "Stop should succeed")

	unsub, err := tp.Subscribe("memory.after_stop", "", func(context.Context, transport.Delivery) error {
		return nil
	}, transport.SubscribeConfig{})

	require.ErrorIs(t, err, imemory.ErrBusStopped,
		"Subscribe after Stop must return ErrBusStopped")
	require.Nil(t, unsub, "unsubscribe func must be nil when Subscribe fails")
}
