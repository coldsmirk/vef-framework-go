package jsevents_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jsevents"
)

// StubBus captures published events for tests.
type StubBus struct {
	published []event.Event
}

func (b *StubBus) Publish(_ context.Context, evt event.Event, _ ...event.PublishOption) error {
	b.published = append(b.published, evt)

	return nil
}

func (b *StubBus) PublishBatch(_ context.Context, evts []event.Event, _ ...event.PublishOption) error {
	b.published = append(b.published, evts...)

	return nil
}

func (*StubBus) Subscribe(string, event.Handler, ...event.SubscribeOption) (event.Unsubscribe, error) {
	return func() {}, nil
}

// newEventsRuntime builds a bare runtime with the events library enabled over
// a capturing bus.
func newEventsRuntime(t *testing.T, opts ...jsevents.Option) (*js.Runtime, *StubBus) {
	t.Helper()

	bus := new(StubBus)

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(jsevents.New(bus, opts...)))
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(js.EnableLibs(jsevents.Name))
	require.NoError(t, err, "NewRuntime should succeed")

	return rt, bus
}

// TestPublish tests event publication through the bus.
func TestPublish(t *testing.T) {
	t.Run("CarriesTypeAndPayload", func(t *testing.T) {
		rt, bus := newEventsRuntime(t)

		_, err := rt.RunString(t.Context(), `events.publish('report.generated', { reportId: 'r1', pages: 3 })`)
		require.NoError(t, err, "Script should execute successfully")

		require.Len(t, bus.published, 1, "One event should be published")

		raw, ok := bus.published[0].(event.RawPayload)
		require.True(t, ok, "The published event should be a RawPayload")
		assert.Equal(t, "report.generated", raw.EventType(), "Event type should match")
		assert.JSONEq(t, `{"reportId":"r1","pages":3}`, string(raw.Body), "Payload should be JSON-encoded")
	})

	t.Run("MissingPayloadPublishesNull", func(t *testing.T) {
		rt, bus := newEventsRuntime(t)

		_, err := rt.RunString(t.Context(), `events.publish('ping.sent')`)
		require.NoError(t, err, "Script should execute successfully")

		require.Len(t, bus.published, 1, "One event should be published")

		raw, ok := bus.published[0].(event.RawPayload)
		require.True(t, ok, "The published event should be a RawPayload")
		assert.JSONEq(t, `null`, string(raw.Body), "A missing payload should encode as JSON null")
	})

	t.Run("EmptyTypeRejected", func(t *testing.T) {
		rt, bus := newEventsRuntime(t)

		result, err := rt.RunString(t.Context(), `
			try {
				events.publish('', { a: 1 });
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Script should handle the thrown error")
		assert.Contains(t, result.String(), "empty event type", "An empty type should throw a catchable error")
		assert.Empty(t, bus.published, "Nothing should be published")
	})
}

// TestAllowedTypes tests the opt-in type allowlist.
func TestAllowedTypes(t *testing.T) {
	t.Run("UnrestrictedByDefault", func(t *testing.T) {
		rt, bus := newEventsRuntime(t)

		_, err := rt.RunString(t.Context(), `events.publish('vef.storage.file.claimed', {})`)
		require.NoError(t, err, "Every type should be publishable without an allowlist")
		assert.Len(t, bus.published, 1, "The event should be published")
	})

	t.Run("ExactMatch", func(t *testing.T) {
		rt, bus := newEventsRuntime(t, jsevents.WithAllowedTypes("report.generated"))

		_, err := rt.RunString(t.Context(), `events.publish('report.generated', {})`)
		require.NoError(t, err, "An exact allowlist entry should pass")
		assert.Len(t, bus.published, 1, "The event should be published")
	})

	t.Run("WildcardMatchesDescendants", func(t *testing.T) {
		rt, bus := newEventsRuntime(t, jsevents.WithAllowedTypes("script.*"))

		_, err := rt.RunString(t.Context(), `events.publish('script.report.done', {})`)
		require.NoError(t, err, "A wildcard entry should match descendants")
		assert.Len(t, bus.published, 1, "The event should be published")
	})

	t.Run("OutsideAllowlistRejected", func(t *testing.T) {
		rt, bus := newEventsRuntime(t, jsevents.WithAllowedTypes("script.*"))

		result, err := rt.RunString(t.Context(), `
			try {
				events.publish('approval.instance.created', {});
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Script should handle the thrown error")
		assert.Contains(t, result.String(), "not allowed", "A type outside the allowlist should throw a catchable error")
		assert.Empty(t, bus.published, "Nothing should be published")
	})
}
