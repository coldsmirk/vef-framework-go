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

// NewEventPublishBehavior buffers domain events produced by a command
// handler and publishes them, in registration order, after the handler
// succeeds. Publishing runs inside the surrounding transaction so the
// framework's event Bus can enroll via event.WithTx(db); each event also
// projects its payload OccurredTime onto Envelope.OccurredAt so downstream
// consumers see business time rather than publish time.
//
// Order positions the behavior as the innermost approval behavior so events
// only emit after the handler and ActionLog have both succeeded.
func NewEventPublishBehavior(db orm.DB, bus event.Bus) cqrs.Behavior {
	return &collectorBehavior[approval.DomainEvent]{
		order: 200,
		name:  "event publish",
		flush: func(ctx context.Context, events []approval.DomainEvent) error {
			db := contextx.DB(ctx, db)

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
