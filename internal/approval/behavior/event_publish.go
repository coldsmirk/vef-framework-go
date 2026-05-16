package behavior

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// EventCollector accumulates domain events produced by a command handler
// while it runs inside the CQRS pipeline. Handlers append events through
// the context-bound collector instead of owning an event.Bus directly;
// EventPublishBehavior then flushes the buffer in one PublishBatch after
// the handler returns, enrolled in the caller transaction.
type EventCollector struct {
	events []approval.DomainEvent
}

// Append records additional events. Nil entries are ignored. Safe to call
// multiple times throughout the handler.
func (c *EventCollector) Append(events ...approval.DomainEvent) {
	for _, e := range events {
		if e == nil {
			continue
		}

		c.events = append(c.events, e)
	}
}

// Events returns the buffered events. Callers should not mutate the slice.
func (c *EventCollector) Events() []approval.DomainEvent {
	return c.events
}

type eventCollectorKey struct{}

// CollectorFromContext returns the request-scoped EventCollector. Returns a
// detached (no-op publish target) collector when called outside the CQRS
// pipeline so unit tests that bypass the bus don't crash on nil receivers.
func CollectorFromContext(ctx context.Context) *EventCollector {
	if c, ok := ctx.Value(eventCollectorKey{}).(*EventCollector); ok {
		return c
	}

	return new(EventCollector)
}

// EventPublishBehavior installs a per-command EventCollector into the
// context and publishes whatever the handler accumulated when the handler
// returns successfully. Queries pass through untouched. Errors short-
// circuit the publish so callers never see partial event streams.
type EventPublishBehavior struct {
	bus event.Bus
}

// NewEventPublishBehavior constructs the behavior.
func NewEventPublishBehavior(bus event.Bus) cqrs.Behavior {
	return &EventPublishBehavior{bus: bus}
}

// Handle wraps the handler with EventCollector lifecycle management.
func (b *EventPublishBehavior) Handle(ctx context.Context, action cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	if action.Kind() == cqrs.Query {
		return next(ctx)
	}

	collector := &EventCollector{}
	ctx = context.WithValue(ctx, eventCollectorKey{}, collector)

	result, err := next(ctx)
	if err != nil {
		return nil, err
	}

	if len(collector.events) == 0 {
		return result, nil
	}

	db := contextx.DB(ctx)
	if err := b.bus.PublishBatch(ctx, event.AsEvents(collector.events), event.WithTx(db)); err != nil {
		return nil, fmt.Errorf("publish collected events: %w", err)
	}

	return result, nil
}
