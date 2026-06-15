package behavior

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// EventCollector is the request-scoped buffer for approval domain events.
// It's just Collector[approval.DomainEvent]; the alias gives call sites a
// stable name even if the generic plumbing is reshaped later.
type EventCollector = Collector[approval.DomainEvent]

// PublishEventsTx publishes domain events through the bus enrolled in the
// caller's transaction (event.WithTx(db)), projecting each payload's
// OccurredTime onto Envelope.OccurredAt so downstream consumers see business
// time rather than publish time. A failed publish is wrapped with the
// offending event's type for context. Returns nil when bus is nil or events
// is empty.
//
// This is the shared publish primitive for the approval module: the CQRS
// EventPublishBehavior flush delegates here, and the engine's PublishEventsTx
// (used by sites outside the CQRS pipeline) wraps it too, so the
// option-building and publish loop live in exactly one place.
func PublishEventsTx(ctx context.Context, bus event.Bus, db orm.DB, events ...approval.DomainEvent) error {
	if bus == nil || len(events) == 0 {
		return nil
	}

	for _, e := range events {
		opts := []event.PublishOption{event.WithTx(db)}
		if t := approval.PayloadOccurredAt(e); !t.IsZero() {
			opts = append(opts, event.WithOccurredAt(t.Unwrap()))
		}

		if err := bus.Publish(ctx, e, opts...); err != nil {
			return fmt.Errorf("publish %s: %w", e.EventType(), err)
		}
	}

	return nil
}

// NewEventPublishBehavior buffers domain events produced by a command
// handler and publishes them, in registration order, after the handler
// succeeds. Publishing runs inside the surrounding transaction so the
// framework's event Bus can enroll via event.WithTx(db); each event also
// projects its payload OccurredTime onto Envelope.OccurredAt so downstream
// consumers see business time rather than publish time.
//
// Order 200 makes this the innermost approval behavior, so among the
// collectors it flushes FIRST — events publish before the outer ActionLog
// inserts its audit rows. Both flushes run inside the same Transaction tx,
// so the events are visible iff that transaction commits.
func NewEventPublishBehavior(db orm.DB, bus event.Bus) cqrs.Behavior {
	return &collectorBehavior[approval.DomainEvent]{
		order: 200,
		name:  "event publish",
		flush: func(ctx context.Context, events []approval.DomainEvent) error {
			return PublishEventsTx(ctx, bus, contextx.DB(ctx, db), events...)
		},
	}
}

// EventCollectorFromContext returns the request-scoped event collector or
// a detached no-op collector (with a warning) when called outside the CQRS
// pipeline so unit tests that bypass the bus don't crash on nil receivers.
func EventCollectorFromContext(ctx context.Context) *EventCollector {
	return collectorFromContextOrWarn[approval.DomainEvent](ctx, "EventCollector", "EventPublishBehavior")
}

// TryEventCollectorFromContext returns the collector silently when missing,
// for callers (engine helpers, saga drivers) that have a sensible fallback.
func TryEventCollectorFromContext(ctx context.Context) (*EventCollector, bool) {
	return TryCollectorFromContext[approval.DomainEvent](ctx)
}
