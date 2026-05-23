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
