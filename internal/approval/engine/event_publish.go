package engine

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// PublishEventsTx publishes domain events through the bus, enrolled in the
// caller transaction, projecting each payload's OccurredTime onto the
// envelope so downstream consumers see "when the thing happened" rather
// than "when we got around to publishing". Returns nil when bus is nil or
// events is empty.
//
// This is the shared primitive used by sites outside the CQRS pipeline
// (timeout scanner, engine fallback when no EventCollector is bound, node
// service auto-CC). The CQRS pipeline itself uses EventCollector +
// EventPublishBehavior to batch the publish at the end of the handler.
func PublishEventsTx(ctx context.Context, bus event.Bus, db orm.DB, events ...approval.DomainEvent) error {
	if bus == nil || len(events) == 0 {
		return nil
	}

	for _, evt := range events {
		opts := []event.PublishOption{event.WithTx(db)}
		if t := approval.PayloadOccurredAt(evt); !t.IsZero() {
			opts = append(opts, event.WithOccurredAt(t.Unwrap()))
		}

		if err := bus.Publish(ctx, evt, opts...); err != nil {
			return err
		}
	}

	return nil
}
