package transporttest

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/id"
)

// Factory constructs a fresh transport for one contract scenario. It
// returns the transport plus a cleanup func invoked via t.Cleanup so
// per-test resources (databases, containers, …) are reclaimed.
type Factory func(t *testing.T) (transport.Transport, func())

// Suite is the public entry point. Pass in a Factory; the suite runs
// every contract scenario that the transport's Capabilities permit.
func Suite(t *testing.T, name string, factory Factory) {
	t.Run(name+"/RoundTrip", func(t *testing.T) { testRoundTrip(t, factory) })
	t.Run(name+"/MultipleSubscribers", func(t *testing.T) { testMultipleSubscribers(t, factory) })
	t.Run(name+"/UnsubscribeStopsDelivery", func(t *testing.T) { testUnsubscribe(t, factory) })
	t.Run(name+"/ConcurrentPublish", func(t *testing.T) { testConcurrentPublish(t, factory) })
}

func testRoundTrip(t *testing.T, factory Factory) {
	tp, cleanup := factory(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	require.NoError(t, tp.Start(ctx), "Start should succeed")

	received := make(chan transport.Frame, 1)
	unsub, err := tp.Subscribe("contract.roundtrip", "group-rt", func(_ context.Context, d transport.Delivery) error {
		received <- d.Frame()

		return d.Ack(ctx)
	}, transport.SubscribeConfig{Group: "group-rt", Concurrency: 1})
	require.NoError(t, err, "Subscribe should succeed")
	t.Cleanup(unsub)

	frame := transport.Frame{
		ID:          id.GenerateUUID(),
		Type:        "contract.roundtrip",
		Source:      "contract-test",
		OccurredAt:  time.Now(),
		PublishedAt: time.Now(),
		Body:        []byte(`{"payload":"ok"}`),
	}
	require.NoError(t, tp.Publish(ctx, []transport.Frame{frame}), "Publish should succeed")

	select {
	case got := <-received:
		require.Equal(t, frame.ID, got.ID, "received frame should preserve ID")
		require.Equal(t, frame.Type, got.Type, "received frame should preserve Type")
		require.JSONEq(t, string(frame.Body), string(got.Body), "payload should round-trip")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for round-trip delivery")
	}

	require.NoError(t, tp.Stop(ctx), "Stop should succeed")
}

func testMultipleSubscribers(t *testing.T, factory Factory) {
	tp, cleanup := factory(t)
	t.Cleanup(cleanup)

	caps := tp.Capabilities()
	if caps.SupportsGroups {
		// With consumer groups, two same-group subscribers split load
		// rather than both receive. Use distinct groups to verify
		// fan-out semantics.
		t.Skip("transport uses consumer groups; fan-out is exercised by multi-group subscriptions outside the contract suite")
	}

	ctx := context.Background()
	require.NoError(t, tp.Start(ctx))

	var wg sync.WaitGroup
	wg.Add(2)

	sub := func(name string) {
		_, err := tp.Subscribe("contract.fanout", "group-fan-"+name, func(_ context.Context, d transport.Delivery) error {
			defer wg.Done()

			return d.Ack(ctx)
		}, transport.SubscribeConfig{Concurrency: 1})
		require.NoError(t, err, "subscribe %s", name)
	}
	sub("a")
	sub("b")

	frame := transport.Frame{
		ID:   id.GenerateUUID(),
		Type: "contract.fanout",
		Body: []byte(`{}`),
	}
	require.NoError(t, tp.Publish(ctx, []transport.Frame{frame}))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fan-out timed out")
	}

	require.NoError(t, tp.Stop(ctx))
}

func testUnsubscribe(t *testing.T, factory Factory) {
	tp, cleanup := factory(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	require.NoError(t, tp.Start(ctx))

	var (
		calls int
		mu    sync.Mutex
	)

	unsub, err := tp.Subscribe("contract.unsubscribe", "group-unsub", func(_ context.Context, d transport.Delivery) error {
		mu.Lock()
		calls++
		mu.Unlock()

		return d.Ack(ctx)
	}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err)

	first := transport.Frame{ID: id.GenerateUUID(), Type: "contract.unsubscribe", Body: []byte(`{}`)}
	require.NoError(t, tp.Publish(ctx, []transport.Frame{first}))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()

		return calls == 1
	}, 5*time.Second, 50*time.Millisecond, "first publish should deliver")

	unsub()
	mu.Lock()
	calls = 0
	mu.Unlock()

	// Use a fresh ID for the second publish so durable transports
	// that uniquely index event IDs don't reject the insert.
	second := transport.Frame{ID: id.GenerateUUID(), Type: "contract.unsubscribe", Body: []byte(`{}`)}
	require.NoError(t, tp.Publish(ctx, []transport.Frame{second}))

	// Assert no further deliveries occur within a bounded window.
	// require.Never polls — far more reliable on slow CI runners than
	// a fixed sleep.
	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()

		return calls > 0
	}, 500*time.Millisecond, 50*time.Millisecond, "unsubscribed handler must not receive further deliveries")

	require.NoError(t, tp.Stop(ctx))
}

func testConcurrentPublish(t *testing.T, factory Factory) {
	tp, cleanup := factory(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	require.NoError(t, tp.Start(ctx))

	const total = 32

	received := make(chan string, total)
	_, err := tp.Subscribe("contract.concurrent", "group-conc", func(_ context.Context, d transport.Delivery) error {
		var body struct{ ID string }
		if err := json.Unmarshal(d.Frame().Body, &body); err != nil {
			return err
		}

		received <- body.ID

		return d.Ack(ctx)
	}, transport.SubscribeConfig{Concurrency: 4})
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(total)

	for range total {
		go func() {
			defer wg.Done()

			payload := id.GenerateUUID()
			body, _ := json.Marshal(struct{ ID string }{ID: payload})
			frame := transport.Frame{ID: payload, Type: "contract.concurrent", Body: body}
			_ = tp.Publish(ctx, []transport.Frame{frame})
		}()
	}

	wg.Wait()

	seen := make(map[string]struct{}, total)

	deadline := time.After(5 * time.Second)
	for len(seen) < total {
		select {
		case id := <-received:
			seen[id] = struct{}{}
		case <-deadline:
			t.Fatalf("only received %d of %d concurrent messages", len(seen), total)
		}
	}

	require.NoError(t, tp.Stop(ctx))
}
