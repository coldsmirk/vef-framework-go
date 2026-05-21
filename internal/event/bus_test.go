package event

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// ---------- test event + transports ----------

type BusTestEvent struct {
	Value string `json:"value"`
}

func (*BusTestEvent) EventType() string { return "bus.test" }

// RecordingTransport collects publishes and dispatches each frame to
// the registered consumer. Tests inspect the captured frames to make
// assertions about middleware mutation, batching, and routing.
type RecordingTransport struct {
	name string
	caps transport.Capabilities

	mu           sync.Mutex
	frames       []transport.Frame
	consumer     transport.ConsumeFunc
	subscribeErr error
}

func newRecordingTransport(name string, caps transport.Capabilities) *RecordingTransport {
	return &RecordingTransport{name: name, caps: caps}
}

func (r *RecordingTransport) Name() string                         { return r.name }
func (r *RecordingTransport) Capabilities() transport.Capabilities { return r.caps }
func (*RecordingTransport) Start(context.Context) error            { return nil }
func (*RecordingTransport) Stop(context.Context) error             { return nil }

func (r *RecordingTransport) Publish(ctx context.Context, frames []transport.Frame) error {
	r.mu.Lock()
	r.frames = append(r.frames, frames...)
	consumer := r.consumer
	r.mu.Unlock()

	if consumer == nil {
		return nil
	}

	for _, f := range frames {
		if err := consumer(ctx, &RecordingDelivery{frame: f}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RecordingTransport) Subscribe(_, _ string, fn transport.ConsumeFunc, _ transport.SubscribeConfig) (transport.Unsubscribe, error) {
	if r.subscribeErr != nil {
		return nil, r.subscribeErr
	}

	r.mu.Lock()
	r.consumer = fn
	r.mu.Unlock()

	return func() {
		r.mu.Lock()
		r.consumer = nil
		r.mu.Unlock()
	}, nil
}

func (r *RecordingTransport) capturedFrames() []transport.Frame {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]transport.Frame(nil), r.frames...)
}

// RecordingDelivery satisfies transport.Delivery with no-op Ack/Nack.
type RecordingDelivery struct{ frame transport.Frame }

func (d *RecordingDelivery) Frame() transport.Frame                         { return d.frame }
func (*RecordingDelivery) Attempt() int                                     { return 1 }
func (*RecordingDelivery) Ack(context.Context) error                        { return nil }
func (*RecordingDelivery) Nack(context.Context, time.Duration, error) error { return nil }

// PublishOnlyTransport mimics the outbox: Publish accepts frames but
// Subscribe is unsupported.
type PublishOnlyTransport struct{ RecordingTransport }

func newPublishOnlyTransport(name string) *PublishOnlyTransport {
	return &PublishOnlyTransport{
		RecordingTransport: RecordingTransport{
			name: name,
			caps: transport.Capabilities{PublishOnly: true, AtLeastOnce: true, Transactional: true},
		},
	}
}

func (*PublishOnlyTransport) Subscribe(string, string, transport.ConsumeFunc, transport.SubscribeConfig) (transport.Unsubscribe, error) {
	return nil, transport.ErrSubscribeUnsupported
}

// FailingStopTransport returns an error from Stop, used to verify error
// aggregation in Bus.Stop.
type FailingStopTransport struct{ RecordingTransport }

func newFailingStopTransport(name string) *FailingStopTransport {
	return &FailingStopTransport{
		RecordingTransport: RecordingTransport{
			name: name,
			caps: transport.Capabilities{},
		},
	}
}

var errStopBoom = errors.New("transport stop boom")

func (*FailingStopTransport) Stop(context.Context) error { return errStopBoom }

// ---------- helpers ----------

func newTestBus(t *testing.T, transports []transport.Transport, mws ...any) *Bus {
	t.Helper()

	cfg := &config.EventConfig{DefaultTransport: transports[0].Name()}

	var (
		pubMW []middleware.PublishMiddleware
		conMW []middleware.ConsumeMiddleware
	)

	for _, mw := range mws {
		switch m := mw.(type) {
		case middleware.PublishMiddleware:
			pubMW = append(pubMW, m)
		case middleware.ConsumeMiddleware:
			conMW = append(conMW, m)
		default:
			t.Fatalf("unsupported middleware type %T", mw)
		}
	}

	bus := NewBus(cfg, "test-app", transports, pubMW, conMW, nil)
	require.NoError(t, bus.Start(t.Context()))

	t.Cleanup(func() { _ = bus.Stop(context.Background()) })

	return bus
}

// ---------- tests ----------

func TestBusPublishSubscribeRoundTrip(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	bus := newTestBus(t, []transport.Transport{mem})

	received := make(chan event.Envelope, 1)
	_, err := bus.Subscribe("bus.test", func(_ context.Context, env event.Envelope) error {
		received <- env

		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Publish(t.Context(), &BusTestEvent{Value: "hello"}))

	select {
	case env := <-received:
		require.Equal(t, "bus.test", env.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not receive published event")
	}

	require.Len(t, mem.capturedFrames(), 1)
}

func TestBusRequiresGroupForAtLeastOnceTransport(t *testing.T) {
	atLeastOnce := newRecordingTransport("redis_stream", transport.Capabilities{AtLeastOnce: true})

	bus := newTestBus(t, []transport.Transport{atLeastOnce})

	_, err := bus.Subscribe("bus.test", func(context.Context, event.Envelope) error { return nil })
	require.Error(t, err, "subscribing to at-least-once transport without WithGroup must fail")
	require.ErrorIs(t, err, event.ErrGroupRequired)

	_, err = bus.Subscribe("bus.test", func(context.Context, event.Envelope) error { return nil },
		event.WithGroup("explicit"))
	require.NoError(t, err, "explicit WithGroup unblocks the subscription")
}

func TestBusSkipsPublishOnlyTransportOnSubscribe(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	pubOnly := newPublishOnlyTransport("outbox")

	cfg := &config.EventConfig{
		DefaultTransport: "memory",
		Routing: []config.EventRoutingRule{
			{Pattern: "*", Transports: []string{"memory", "outbox"}},
		},
	}

	bus := NewBus(cfg, "test-app", []transport.Transport{mem, pubOnly}, nil, nil, nil)
	require.NoError(t, bus.Start(t.Context()))
	t.Cleanup(func() { _ = bus.Stop(context.Background()) })

	calls := atomic.Int64{}
	_, err := bus.Subscribe("bus.test",
		func(context.Context, event.Envelope) error {
			calls.Add(1)

			return nil
		},
		event.WithGroup("g"),
	)
	require.NoError(t, err, "subscribe should skip publish-only transports and still register on the in-process one")

	require.NoError(t, bus.Publish(t.Context(), &BusTestEvent{Value: "fan-out"}))

	require.Eventually(t, func() bool { return calls.Load() == 1 },
		2*time.Second, 10*time.Millisecond,
		"handler should fire exactly once, not duplicated by the outbox-forwarded path")

	// Both transports should still see the publish (fan-out for publish).
	require.Len(t, mem.capturedFrames(), 1)
	require.Len(t, pubOnly.capturedFrames(), 1)
}

func TestBusActiveMapClearedAfterUnsubscribe(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	bus := newTestBus(t, []transport.Transport{mem})

	unsub, err := bus.Subscribe("bus.test", func(context.Context, event.Envelope) error { return nil })
	require.NoError(t, err)

	bus.mu.Lock()
	require.Len(t, bus.active, 1, "active map should hold the live subscription")
	bus.mu.Unlock()

	unsub()
	unsub() // idempotent — must not panic or double-delete

	bus.mu.Lock()
	require.Empty(t, bus.active, "unsubscribe must remove the entry so Stop cannot double-invoke it")
	bus.mu.Unlock()
}

func TestBusStopAggregatesTransportErrors(t *testing.T) {
	failing := newFailingStopTransport("memory")
	cfg := &config.EventConfig{DefaultTransport: "memory"}

	bus := NewBus(cfg, "test-app", []transport.Transport{failing}, nil, nil, nil)
	require.NoError(t, bus.Start(t.Context()))

	err := bus.Stop(t.Context())
	require.Error(t, err, "Stop must surface transport failures, not swallow them")
	require.ErrorIs(t, err, errStopBoom)
}

func TestBusPublishWithoutStartFails(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	cfg := &config.EventConfig{DefaultTransport: "memory"}

	bus := NewBus(cfg, "test-app", []transport.Transport{mem}, nil, nil, nil)
	err := bus.Publish(context.Background(), &BusTestEvent{Value: "before-start"})
	require.ErrorIs(t, err, event.ErrBusNotStarted)
}

func TestBusStartTwiceFails(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	cfg := &config.EventConfig{DefaultTransport: "memory"}

	bus := NewBus(cfg, "test-app", []transport.Transport{mem}, nil, nil, nil)
	require.NoError(t, bus.Start(t.Context()))
	t.Cleanup(func() { _ = bus.Stop(context.Background()) })

	err := bus.Start(t.Context())
	require.ErrorIs(t, err, event.ErrBusAlreadyStarted)
}

func TestBusTxAndAsyncAreMutuallyExclusive(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	bus := newTestBus(t, []transport.Transport{mem})

	// WithTx requires a non-nil interface value to set cfg.Tx; a literal
	// nil collapses to a nil interface and would skip the mutex check.
	db := testx.NewTestDB(t)
	err := bus.Publish(t.Context(), &BusTestEvent{Value: "x"},
		event.WithTx(db), event.WithAsync())
	require.ErrorIs(t, err, event.ErrTxAsyncMutex)
}

func TestBusPendingSubscriptionFlushedOnStart(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	cfg := &config.EventConfig{DefaultTransport: "memory"}

	bus := NewBus(cfg, "test-app", []transport.Transport{mem}, nil, nil, nil)

	received := make(chan struct{}, 1)
	unsub, err := bus.Subscribe("bus.test", func(context.Context, event.Envelope) error {
		select {
		case received <- struct{}{}:
		default:
		}

		return nil
	})
	require.NoError(t, err, "Subscribe before Start should buffer, not error")
	require.NotNil(t, unsub)

	require.NoError(t, bus.Start(t.Context()))
	t.Cleanup(func() { _ = bus.Stop(context.Background()) })

	require.NoError(t, bus.Publish(t.Context(), &BusTestEvent{Value: "pending"}))

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("pending subscription did not flush during Start")
	}
}

// ---- publish middleware chain shared-build verification ----

type CountingPublishMW struct {
	wraps atomic.Int64
	calls atomic.Int64
}

func (*CountingPublishMW) Name() string { return "counting-pub" }

func (m *CountingPublishMW) WrapPublish(next middleware.PublishHandler) middleware.PublishHandler {
	m.wraps.Add(1)

	return func(ctx context.Context, env *event.Envelope) error {
		m.calls.Add(1)

		return next(ctx, env)
	}
}

func TestPublishMiddlewareChainBuiltOncePerBatch(t *testing.T) {
	mem := newRecordingTransport("memory", transport.Capabilities{})
	mw := new(CountingPublishMW)

	bus := newTestBus(t, []transport.Transport{mem}, mw)

	require.NoError(t, bus.PublishBatch(t.Context(), []event.Event{
		&BusTestEvent{Value: "a"},
		&BusTestEvent{Value: "b"},
		&BusTestEvent{Value: "c"},
	}))

	// WrapPublish should be invoked once per batch (chain build), not once per event.
	require.EqualValues(t, 1, mw.wraps.Load(),
		"publish middleware chain must be built once per PublishBatch, not per event")
	require.EqualValues(t, 3, mw.calls.Load(),
		"wrapped handler should still execute once per event")
}
