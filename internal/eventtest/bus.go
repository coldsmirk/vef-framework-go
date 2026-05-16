// Package eventtest provides synchronous in-memory test doubles for
// the framework's event Bus. The default FakeBus exercises the same
// Subscribe → Publish flow as the production Bus but skips routing,
// async fan-in, and middleware composition; suitable for unit tests
// of resolvers, caches, and other handlers that subscribe to events.
package eventtest

import (
	"context"
	"sync"

	"github.com/coldsmirk/vef-framework-go/event"
)

// FakeBus is a synchronous in-memory Bus suitable for unit tests.
// Subscribe registers a handler indexed by event type; Publish invokes
// all matching handlers in registration order and returns the first
// non-nil error. PublishOption and SubscribeOption are accepted for
// signature compatibility but otherwise ignored.
//
// FakeBus also records every published event for later inspection via
// Captured / CapturedByType so tests that previously queried an outbox
// table can verify behavior without database round-trips.
type FakeBus struct {
	mu       sync.Mutex
	handlers map[string][]event.Handler
	captured []event.Event
}

// NewFakeBus returns a fresh FakeBus with no subscriptions.
func NewFakeBus() *FakeBus {
	return &FakeBus{handlers: make(map[string][]event.Handler)}
}

// Subscribe implements event.Bus.
func (b *FakeBus) Subscribe(eventType string, h event.Handler, _ ...event.SubscribeOption) (event.Unsubscribe, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], h)
	idx := len(b.handlers[eventType]) - 1

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		hs := b.handlers[eventType]
		if idx < len(hs) {
			hs[idx] = nil
		}
	}, nil
}

// Publish implements event.Bus by synchronously invoking all
// subscribers for the event type. The event is recorded into the
// internal capture log before dispatch.
func (b *FakeBus) Publish(ctx context.Context, evt event.Event, _ ...event.PublishOption) error {
	b.mu.Lock()
	b.captured = append(b.captured, evt)
	handlers := append([]event.Handler(nil), b.handlers[evt.EventType()]...)
	b.mu.Unlock()

	env := event.Envelope{Type: evt.EventType(), Payload: evt}
	for _, h := range handlers {
		if h == nil {
			continue
		}

		if err := h(ctx, env); err != nil {
			return err
		}
	}

	return nil
}

// Captured returns a snapshot of all events seen by the bus, in
// publication order.
func (b *FakeBus) Captured() []event.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]event.Event, len(b.captured))
	copy(out, b.captured)

	return out
}

// CapturedByType returns the subset of captured events whose
// EventType() matches the supplied topic.
func (b *FakeBus) CapturedByType(eventType string) []event.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]event.Event, 0, len(b.captured))
	for _, evt := range b.captured {
		if evt.EventType() == eventType {
			out = append(out, evt)
		}
	}

	return out
}

// Reset clears the capture log so suites can reuse a single bus
// across subtests.
func (b *FakeBus) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.captured = nil
}

// PublishBatch implements event.Bus by publishing each event in order,
// stopping on the first error.
func (b *FakeBus) PublishBatch(ctx context.Context, evts []event.Event, opts ...event.PublishOption) error {
	for _, evt := range evts {
		if err := b.Publish(ctx, evt, opts...); err != nil {
			return err
		}
	}

	return nil
}
