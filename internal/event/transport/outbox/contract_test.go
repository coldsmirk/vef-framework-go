package outbox_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/id"
	imemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
	ioutbox "github.com/coldsmirk/vef-framework-go/internal/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// The outbox transport is publish-only — it persists records that a
// relay later forwards to a sink. The standard transporttest.Suite
// requires Subscribe on the transport under test, so we exercise the
// outbox's contract here via the publish → relay → sink path instead.

// TestOutboxSubscribeUnsupported pins the publish-only contract: any
// Subscribe call against the outbox must surface
// transport.ErrSubscribeUnsupported.
func TestOutboxSubscribeUnsupported(t *testing.T) {
	repo := ioutbox.NewRepository(testx.NewTestDB(t))
	tp := ioutbox.NewTransport(repo, outbox.Config{})
	tp.SetSink(imemory.New(memory.Config{}))

	_, err := tp.Subscribe("anything", "g", func(context.Context, transport.Delivery) error { return nil }, transport.SubscribeConfig{})
	require.ErrorIs(t, err, transport.ErrSubscribeUnsupported,
		"outbox subscribe must surface ErrSubscribeUnsupported so the bus skips it during fan-out routing")
}

// TestOutboxEndToEndRoundTrip verifies the publish → relay → sink path:
// frames written to the outbox are delivered to subscribers of the
// configured sink transport within bounded time.
func TestOutboxEndToEndRoundTrip(t *testing.T) {
	ctx := context.Background()

	db := testx.NewTestDB(t)
	require.NoError(t, ioutbox.Migrate(ctx, db, config.SQLite))

	repo := ioutbox.NewRepository(db)
	sink := imemory.New(memory.Config{QueueSize: 16, FullPolicy: memory.FullPolicyError})
	require.NoError(t, sink.Start(ctx))
	t.Cleanup(func() { _ = sink.Stop(ctx) })

	cfg := outbox.Config{
		RelayInterval:   50 * time.Millisecond,
		MaxRetries:      3,
		BatchSize:       16,
		LeaseMultiplier: 4,
		MinLease:        time.Second,
	}
	tp := ioutbox.NewTransport(repo, cfg)
	tp.SetSink(sink)
	require.NoError(t, tp.Start(ctx))

	received := make(chan transport.Frame, 1)
	_, err := sink.Subscribe("contract.outbox.roundtrip", "g-rt",
		func(_ context.Context, d transport.Delivery) error {
			received <- d.Frame()

			return nil
		}, transport.SubscribeConfig{Concurrency: 1})
	require.NoError(t, err)

	frame := transport.Frame{
		ID:          id.GenerateUUID(),
		Type:        "contract.outbox.roundtrip",
		Source:      "contract-test",
		OccurredAt:  time.Now(),
		PublishedAt: time.Now(),
		Body:        []byte(`{"hello":"outbox"}`),
	}
	require.NoError(t, tp.Publish(ctx, []transport.Frame{frame}))

	// One relay cycle is enough since the test owns the schedule.
	relay := ioutbox.NewRelay(repo, tp.Sink, cfg, nil, nil)
	relay.RelayPending(ctx)

	select {
	case got := <-received:
		require.Equal(t, frame.ID, got.ID)
		require.JSONEq(t, string(frame.Body), string(got.Body))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for relay → sink delivery")
	}
}

// TestOutboxRelayConcurrentPublish exercises the relay batching path
// under concurrent publishes — equivalent intent to the contract
// ConcurrentPublish scenario but rerouted through the publish→relay→sink
// pipeline that the outbox actually owns.
func TestOutboxRelayConcurrentPublish(t *testing.T) {
	ctx := context.Background()

	db := testx.NewTestDB(t)
	require.NoError(t, ioutbox.Migrate(ctx, db, config.SQLite))

	repo := ioutbox.NewRepository(db)
	sink := imemory.New(memory.Config{QueueSize: 128, FullPolicy: memory.FullPolicyError})
	require.NoError(t, sink.Start(ctx))
	t.Cleanup(func() { _ = sink.Stop(ctx) })

	cfg := outbox.Config{
		RelayInterval:   50 * time.Millisecond,
		MaxRetries:      3,
		BatchSize:       64,
		LeaseMultiplier: 4,
		MinLease:        time.Second,
	}
	tp := ioutbox.NewTransport(repo, cfg)
	tp.SetSink(sink)
	require.NoError(t, tp.Start(ctx))

	const total = 32

	received := make(chan string, total)
	_, err := sink.Subscribe("contract.outbox.concurrent", "g-conc",
		func(_ context.Context, d transport.Delivery) error {
			var body struct{ ID string }
			if err := json.Unmarshal(d.Frame().Body, &body); err != nil {
				return err
			}

			received <- body.ID

			return nil
		}, transport.SubscribeConfig{Concurrency: 4})
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(total)

	var publishErrors atomic.Int32

	for range total {
		go func() {
			defer wg.Done()

			payload := id.GenerateUUID()
			body, _ := json.Marshal(struct{ ID string }{ID: payload})
			frame := transport.Frame{ID: payload, Type: "contract.outbox.concurrent", Body: body}

			if err := tp.Publish(ctx, []transport.Frame{frame}); err != nil {
				publishErrors.Add(1)
			}
		}()
	}

	wg.Wait()
	require.Zero(t, publishErrors.Load(), "every publish should accept the frame")

	// Drain the outbox in a single relay cycle; BatchSize >= total.
	relay := ioutbox.NewRelay(repo, tp.Sink, cfg, nil, nil)
	relay.RelayPending(ctx)

	seen := make(map[string]struct{}, total)

	deadline := time.After(5 * time.Second)
	for len(seen) < total {
		select {
		case got := <-received:
			seen[got] = struct{}{}
		case <-deadline:
			t.Fatalf("only received %d of %d concurrent messages", len(seen), total)
		}
	}
}
