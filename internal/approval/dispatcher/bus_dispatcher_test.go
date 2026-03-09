package dispatcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
)

// MockPublisher captures published events for test assertions.
type MockPublisher struct {
	Events []event.Event
}

func (m *MockPublisher) Publish(evt event.Event) {
	m.Events = append(m.Events, evt)
}

func TestBusDispatcherDispatch(t *testing.T) {
	newRecord := func(eventID, eventType string, payload map[string]any) approval.EventOutbox {
		return approval.EventOutbox{
			EventID:   eventID,
			EventType: eventType,
			Payload:   payload,
			Status:    approval.EventOutboxPending,
		}
	}

	t.Run("PublishesOutboxEventWithCorrectFields", func(t *testing.T) {
		pub := &MockPublisher{}
		dispatcher := NewBusDispatcher(pub)

		record := newRecord("evt-001", "approval.instance.created", map[string]any{
			"instanceId":  "inst-1",
			"flowId":      "flow-1",
			"applicantId": "user-1",
		})

		err := dispatcher.Dispatch(t.Context(), record)
		require.NoError(t, err, "Should dispatch without error")
		require.Len(t, pub.Events, 1, "Should publish one event")

		evt, ok := pub.Events[0].(*OutboxEvent)
		require.True(t, ok, "Should publish an *OutboxEvent")
		assert.Equal(t, "approval.instance.created", evt.Type(), "Should set correct event type")
		assert.Equal(t, "approval", evt.Source(), "Should set source to 'approval'")
		assert.Equal(t, "evt-001", evt.EventID, "Should set EventID")
		assert.Equal(t, "inst-1", evt.Payload["instanceId"], "Should preserve payload data")
		assert.Equal(t, "flow-1", evt.Payload["flowId"], "Should preserve payload data")
		assert.Equal(t, "user-1", evt.Payload["applicantId"], "Should preserve payload data")
	})

	t.Run("EmptyPayload", func(t *testing.T) {
		pub := &MockPublisher{}
		dispatcher := NewBusDispatcher(pub)

		err := dispatcher.Dispatch(t.Context(), newRecord("evt-002", "approval.flow.published", map[string]any{}))
		require.NoError(t, err, "Should dispatch empty payload without error")
		require.Len(t, pub.Events, 1, "Should publish one event")

		evt := pub.Events[0].(*OutboxEvent)
		assert.Empty(t, evt.Payload, "Payload should be empty")
	})

	t.Run("NilPayload", func(t *testing.T) {
		pub := &MockPublisher{}
		dispatcher := NewBusDispatcher(pub)

		err := dispatcher.Dispatch(t.Context(), newRecord("evt-003", "approval.test", nil))
		require.NoError(t, err, "Should dispatch nil payload without error")
		require.Len(t, pub.Events, 1, "Should publish one event")

		evt := pub.Events[0].(*OutboxEvent)
		assert.Nil(t, evt.Payload, "Payload should be nil")
	})
}
