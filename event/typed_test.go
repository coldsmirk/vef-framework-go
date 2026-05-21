package event_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
)

// PointerEvent uses a pointer receiver, exercising the SubscribeTyped[*T] path.
type PointerEvent struct {
	Value string `json:"value"`
}

func (*PointerEvent) EventType() string { return "test.pointer" }

// ValueEvent uses a value receiver to exercise SubscribeTyped[ValueT].
type ValueEvent struct {
	Value string `json:"value"`
}

func (ValueEvent) EventType() string { return "test.value" }

func TestSubscribeTypedRejectsNilTypeParameter(t *testing.T) {
	bus := newFakeBus()

	_, err := event.SubscribeTyped[event.Event](bus, func(context.Context, event.Event, event.Envelope) error {
		return nil
	})
	require.ErrorIs(t, err, event.ErrNilTypeParameter,
		"SubscribeTyped instantiated with the bare interface should return ErrNilTypeParameter")
}

func TestSubscribeTypedDecodesPointerFromRawPayload(t *testing.T) {
	bus := newFakeBus()

	received := make(chan *PointerEvent, 1)
	_, err := event.SubscribeTyped[*PointerEvent](bus, func(_ context.Context, evt *PointerEvent, _ event.Envelope) error {
		received <- evt

		return nil
	})
	require.NoError(t, err)

	body, err := json.Marshal(&PointerEvent{Value: "wire"})
	require.NoError(t, err)

	env := event.Envelope{
		ID:      "id-1",
		Type:    "test.pointer",
		Payload: event.RawPayload{Type: "test.pointer", Body: body},
	}
	require.NoError(t, bus.dispatch(context.Background(), env))

	got := <-received
	require.Equal(t, "wire", got.Value, "cross-process payload should decode into pointer event")
}

func TestSubscribeTypedDecodesValueFromRawPayload(t *testing.T) {
	bus := newFakeBus()

	received := make(chan ValueEvent, 1)
	_, err := event.SubscribeTyped[ValueEvent](bus, func(_ context.Context, evt ValueEvent, _ event.Envelope) error {
		received <- evt

		return nil
	})
	require.NoError(t, err)

	body, err := json.Marshal(ValueEvent{Value: "wire-val"})
	require.NoError(t, err)

	env := event.Envelope{
		Type:    "test.value",
		Payload: event.RawPayload{Type: "test.value", Body: body},
	}
	require.NoError(t, bus.dispatch(context.Background(), env))

	got := <-received
	require.Equal(t, "wire-val", got.Value, "cross-process payload should decode into value event")
}

func TestSubscribeTypedAcceptsInProcessPointer(t *testing.T) {
	bus := newFakeBus()

	received := make(chan *PointerEvent, 1)
	_, err := event.SubscribeTyped[*PointerEvent](bus, func(_ context.Context, evt *PointerEvent, _ event.Envelope) error {
		received <- evt

		return nil
	})
	require.NoError(t, err)

	original := &PointerEvent{Value: "in-process"}
	env := event.Envelope{Type: "test.pointer", Payload: original}
	require.NoError(t, bus.dispatch(context.Background(), env))

	got := <-received
	require.Same(t, original, got, "in-process delivery should pass the original pointer through, no copy")
}

func TestSubscribeTypedWrapsUnknownPayload(t *testing.T) {
	bus := newFakeBus()

	_, err := event.SubscribeTyped[*PointerEvent](bus, func(context.Context, *PointerEvent, event.Envelope) error {
		return nil
	})
	require.NoError(t, err)

	// otherEvent satisfies event.Event but is the wrong concrete type
	// for the typed handler — decode should fall through to RawPayload
	// detection and fail with ErrUnknownPayload.
	env := event.Envelope{Type: "test.pointer", Payload: &ValueEvent{Value: "wrong"}}
	err = bus.dispatch(context.Background(), env)
	require.Error(t, err)
	require.ErrorIs(t, err, event.ErrUnknownPayload, "mismatched concrete payload should surface ErrUnknownPayload")
}

func TestSubscribeTypedDecodeFailureWrapsError(t *testing.T) {
	bus := newFakeBus()

	_, err := event.SubscribeTyped[*PointerEvent](bus, func(context.Context, *PointerEvent, event.Envelope) error {
		return nil
	})
	require.NoError(t, err)

	env := event.Envelope{
		Type:    "test.pointer",
		Payload: event.RawPayload{Type: "test.pointer", Body: []byte("not json")},
	}
	err = bus.dispatch(context.Background(), env)
	require.Error(t, err, "invalid JSON body should produce an error")
	require.Contains(t, err.Error(), "decode", "error should mention decoding")
}

func TestAsEventsConvertsTypedSlice(t *testing.T) {
	src := []*PointerEvent{
		{Value: "a"},
		{Value: "b"},
	}

	out := event.AsEvents(src)
	require.Len(t, out, 2, "AsEvents should preserve length")
	require.Same(t, src[0], out[0], "AsEvents should not copy underlying items")
	require.Same(t, src[1], out[1])
}

func TestRawPayloadEventTypeReturnsType(t *testing.T) {
	raw := event.RawPayload{Type: "vef.test"}
	require.Equal(t, "vef.test", raw.EventType(), "RawPayload should expose its Type via EventType")
}

// FakeBus is the bare minimum bus needed to exercise SubscribeTyped.
// eventtest.FakeBus wraps every payload as Envelope{Type, Payload: evt},
// which prevents these tests from injecting RawPayload envelopes — the
// whole point of the decoding suite — so a local stub is kept here.
type FakeBus struct {
	mu       sync.Mutex
	handlers map[string][]event.Handler
}

func newFakeBus() *FakeBus { return &FakeBus{handlers: make(map[string][]event.Handler)} }

func (*FakeBus) Publish(context.Context, event.Event, ...event.PublishOption) error {
	panic("FakeBus.Publish should not be reached from SubscribeTyped tests")
}

func (*FakeBus) PublishBatch(context.Context, []event.Event, ...event.PublishOption) error {
	panic("FakeBus.PublishBatch should not be reached from SubscribeTyped tests")
}

func (b *FakeBus) Subscribe(eventType string, h event.Handler, _ ...event.SubscribeOption) (event.Unsubscribe, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], h)

	return func() {}, nil
}

func (b *FakeBus) dispatch(ctx context.Context, env event.Envelope) error {
	b.mu.Lock()
	handlers := append([]event.Handler(nil), b.handlers[env.Type]...)
	b.mu.Unlock()

	for _, h := range handlers {
		if err := h(ctx, env); err != nil {
			return err
		}
	}

	return nil
}
