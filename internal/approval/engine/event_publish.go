package engine

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// PublishEventsTx publishes domain events through the bus, enrolled in the
// caller transaction, projecting each payload's OccurredTime onto the
// envelope so downstream consumers see "when the thing happened" rather
// than "when we got around to publishing". Returns nil when bus is nil or
// events is empty.
//
// This is the entry point used by sites outside the CQRS pipeline (timeout
// scanner, engine fallback when no EventCollector is bound, node service
// auto-CC). The CQRS pipeline itself uses EventCollector +
// EventPublishBehavior to batch the publish at the end of the handler. Both
// paths share behavior.PublishEventsTx so the publish loop lives in one place.
func PublishEventsTx(ctx context.Context, bus event.Bus, db orm.DB, events ...approval.DomainEvent) error {
	return behavior.PublishEventsTx(ctx, bus, db, events...)
}
