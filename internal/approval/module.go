package approval

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/auth"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/migration"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/resource"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/approval/timeout"
)

// Module is the approval workflow engine module.
var Module = fx.Module(
	"vef:approval",

	auth.Module,
	strategy.Module,
	behavior.Module,
	binding.Module,
	engine.Module,
	service.Module,
	storage.Module,
	command.Module,
	query.Module,
	resource.Module,
	timeout.Module,
	migration.Module,

	fx.Invoke(verifyEventRouting),
)

// nonTransactionalEventTypes is the explicit, reviewed set of approval event
// types that are NOT required to route through a transactional transport.
// InstanceBindingFailedEvent is the sole member: it is emitted by the
// asynchronous binding listener outside any business transaction (the
// listener falls back to a plain publish on event.ErrTxRequired), so
// requiring a transactional route for it would force misconfiguration on
// hosts that legitimately route only binding_failed through non-tx paths.
//
// Any future intentional exclusion must be added here as a conscious edit;
// TestTransactionalEventTypesCoverAllEvents asserts that
// transactionalEventTypes == approval.AllEventTypes() minus this set, so an
// accidentally-omitted new event constant fails the test instead of silently
// bypassing the fail-fast routing check.
var nonTransactionalEventTypes = map[string]struct{}{
	approval.EventTypeInstanceBindingFailed: {},
}

// transactionalEventTypes lists the approval event types that must route to a
// transactional transport, derived from the canonical approval.AllEventTypes()
// minus the documented nonTransactionalEventTypes. Deriving it (rather than
// hand-maintaining a parallel list) means a new event constant is transactional
// by default and a drift test guards the one allowed exclusion.
var transactionalEventTypes = buildTransactionalEventTypes()

func buildTransactionalEventTypes() []string {
	all := approval.AllEventTypes()
	out := make([]string, 0, len(all))

	for _, et := range all {
		if _, excluded := nonTransactionalEventTypes[et]; excluded {
			continue
		}

		out = append(out, et)
	}

	return out
}

// verifyEventRouting fails fast at start-up when the framework's event
// bus is mis-configured for approval's two delivery requirements:
//
//  1. Every business-side event must route to a transactional
//     transport. EventPublishBehavior and engine.PublishEventsTx
//     publish with event.WithTx so consumers see the event iff the
//     originating business transaction commits; without a
//     transactional route the first publish would fail at runtime with
//     event.ErrTxRequired and roll the business transaction back.
//
//  2. Events that approval itself subscribes to must additionally
//     route to a subscribable (non publish-only) transport. The
//     binding listener attaches to InstanceCompletedEvent; a route
//     resolving only to a publish-only outbox would silently filter
//     the subscription at registration time, so no binding write-back
//     would ever happen even though the application started cleanly.
//
// The check itself is deferred to OnStart so the bus has built its
// router by the time we query it (bus.Start runs first in the lifecycle
// order — see bootstrap module ordering).
func verifyEventRouting(lc fx.Lifecycle, inspector event.RouteInspector) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			txRequired := transactionalEventTypes

			for _, et := range txRequired {
				if !inspector.HasTransactionalRoute(et) {
					return fmt.Errorf(
						"%w: %q (enable vef.event.transports.outbox.enabled=true and add a "+
							"routing rule for pattern \"approval.*\" -> [\"outbox\", \"memory\"] "+
							"or [\"outbox\", \"redis_stream\"] with the matching outbox sink)",
						ErrEventRouteNotTransactional, et)
				}
			}

			// Framework-internal subscribers attach to these event types.
			// The route must include a sink transport (memory or
			// redis_stream) alongside the outbox, otherwise the bus
			// filters the subscription out at registration time and the
			// listener is silently dead.
			subscribed := []string{
				approval.EventTypeInstanceCompleted,
			}
			for _, et := range subscribed {
				if !inspector.HasSubscribableTransport(et) {
					return fmt.Errorf(
						"%w: %q (the binding listener subscribes to this event; the "+
							"routing rule for pattern \"approval.*\" must include a sink "+
							"transport such as \"memory\" or \"redis_stream\" alongside \"outbox\")",
						ErrEventRouteNotSubscribable, et)
				}
			}

			return nil
		},
	})
}
