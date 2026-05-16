package behavior

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var logger = logx.Named("approval:behavior")

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
//
// Detached collectors warn — outside test fixtures, missing context means a
// handler ran without EventPublishBehavior in the pipeline and its events
// will silently disappear. Set up a behavior chain (or call from inside the
// CQRS bus) to silence the warning.
func CollectorFromContext(ctx context.Context) *EventCollector {
	if c, ok := TryCollectorFromContext(ctx); ok {
		return c
	}

	logger.Warnf("approval: EventCollector missing from context — events will be discarded; ensure EventPublishBehavior is registered")

	return new(EventCollector)
}

// TryCollectorFromContext returns the request-scoped EventCollector if one
// is installed, else (nil, false). Use this when the absence of the
// collector should not produce a warning — for example, engine helpers
// that need to fall back to direct bus.Publish when invoked from a cron
// or saga outside the CQRS pipeline.
func TryCollectorFromContext(ctx context.Context) (*EventCollector, bool) {
	c, ok := ctx.Value(eventCollectorKey{}).(*EventCollector)
	if !ok {
		return nil, false
	}

	return c, true
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

// Order positions EventPublishBehavior as the innermost in the approval
// pipeline so events publish only after the handler and ActionLog have
// succeeded. Still inside the tx (Order 0 Transaction wraps everything),
// so PublishBatch can enroll via event.WithTx(db).
func (*EventPublishBehavior) Order() int { return 200 }

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
	for _, e := range collector.events {
		opts := []event.PublishOption{event.WithTx(db)}
		if t := approval.PayloadOccurredAt(e); !t.IsZero() {
			// Project the payload's business time onto the envelope so
			// downstream consumers see "when the thing happened" rather
			// than "when we got around to publishing".
			opts = append(opts, event.WithOccurredAt(t.Unwrap()))
		}

		if err := b.bus.Publish(ctx, e, opts...); err != nil {
			return nil, fmt.Errorf("publish collected event %s: %w", e.EventType(), err)
		}
	}

	return result, nil
}
