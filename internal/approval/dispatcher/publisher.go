package dispatcher

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/mapx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// EventPublisher publishes domain events to the EventOutbox table.
type EventPublisher struct{}

// NewEventPublisher creates a new EventPublisher.
func NewEventPublisher() *EventPublisher {
	return new(EventPublisher)
}

// PublishAll marshals each event and inserts into the event outbox table.
func (*EventPublisher) PublishAll(ctx context.Context, db orm.DB, events []approval.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	outboxRecords := make([]approval.EventOutbox, len(events))

	for i, evt := range events {
		payloadMap, err := mapx.ToMap(evt)
		if err != nil {
			return fmt.Errorf("failed to convert event %q to map: %w", evt.EventName(), err)
		}

		outboxRecords[i] = approval.EventOutbox{
			EventID:   id.GenerateUUID(),
			EventType: evt.EventName(),
			Payload:   payloadMap,
			Status:    approval.EventOutboxPending,
		}
	}

	_, err := db.NewInsert().Model(&outboxRecords).Exec(ctx)

	return err
}
