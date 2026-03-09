package dispatcher

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
)

// OutboxEvent represents an approval outbox event published to the event bus.
type OutboxEvent struct {
	event.BaseEvent

	EventID string         `json:"eventId"`
	Payload map[string]any `json:"payload"`
}

// NewOutboxEvent creates a new OutboxEvent from an EventOutbox record.
func NewOutboxEvent(record approval.EventOutbox) *OutboxEvent {
	return &OutboxEvent{
		BaseEvent: event.NewBaseEvent(record.EventType, event.WithSource("approval")),
		EventID:   record.EventID,
		Payload:   record.Payload,
	}
}

// BusDispatcher is the default EventDispatcher that forwards outbox events to event.Bus.
type BusDispatcher struct {
	publisher event.Publisher
}

// NewBusDispatcher creates a new BusDispatcher.
func NewBusDispatcher(publisher event.Publisher) approval.EventDispatcher {
	return &BusDispatcher{publisher: publisher}
}

// Dispatch converts an EventOutbox record to an OutboxEvent and publishes it to the event bus.
func (d *BusDispatcher) Dispatch(_ context.Context, record approval.EventOutbox) error {
	d.publisher.Publish(NewOutboxEvent(record))

	return nil
}
