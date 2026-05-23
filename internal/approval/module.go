package approval

import (
	"context"
	"errors"
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
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/approval/timeout"
)

// ErrEventRouteNotTransactional indicates the framework's event bus is
// not configured to deliver an approval domain event through a
// transactional transport. Approval publishes every business-side event
// with event.WithTx (via EventPublishBehavior and engine.PublishEventsTx)
// so subscribers see the event iff the originating business transaction
// commits; without a transactional route the first publish would fail at
// runtime with event.ErrTxRequired and roll the business transaction
// back. The wrapped formatted error names the offending event type and
// points operators at the configuration that must be set.
var ErrEventRouteNotTransactional = errors.New("approval: event must route to a transactional transport")

// ErrEventRouteNotSubscribable indicates the framework's event bus has
// no subscribable transport on the route for an event that approval
// itself subscribes to. The binding listener subscribes to
// InstanceCompletedEvent; a route resolving only to publish-only
// transports (e.g. just the outbox) would let the application start,
// then silently drop every event because Subscribe is filtered at
// routing time. The wrapped formatted error names the offending event
// type and points operators at the configuration that must be set.
var ErrEventRouteNotSubscribable = errors.New("approval: event must route to a subscribable transport")

// Module is the approval workflow engine module.
var Module = fx.Module(
	"vef:approval",

	auth.Module,
	strategy.Module,
	behavior.Module,
	binding.Module,
	engine.Module,
	service.Module,
	command.Module,
	query.Module,
	resource.Module,
	timeout.Module,
	migration.Module,

	fx.Invoke(verifyEventRouting),
)

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
// InstanceBindingFailedEvent is deliberately excluded from the
// transactional set: it is emitted by the asynchronous binding listener
// outside any business transaction, so requiring a transactional route
// for it would force misconfiguration on hosts that legitimately route
// only binding_failed through non-tx paths.
//
// The check itself is deferred to OnStart so the bus has built its
// router by the time we query it (bus.Start runs first in the lifecycle
// order — see bootstrap module ordering).
func verifyEventRouting(lc fx.Lifecycle, inspector event.RouteInspector) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			txRequired := []string{
				approval.EventTypeInstanceCreated,
				approval.EventTypeInstanceCompleted,
				approval.EventTypeInstanceWithdrawn,
				approval.EventTypeInstanceRolledBack,
				approval.EventTypeInstanceReturned,
				approval.EventTypeInstanceResubmitted,
				approval.EventTypeNodeEntered,
				approval.EventTypeNodeAutoPassed,
				approval.EventTypeTaskCreated,
				approval.EventTypeTaskApproved,
				approval.EventTypeTaskHandled,
				approval.EventTypeTaskRejected,
				approval.EventTypeTaskTransferred,
				approval.EventTypeTaskReassigned,
				approval.EventTypeTaskTimedOut,
				approval.EventTypeAssigneesAdded,
				approval.EventTypeAssigneesRemoved,
				approval.EventTypeTaskDeadlineWarning,
				approval.EventTypeTaskUrged,
				approval.EventTypeCCNotified,
				approval.EventTypeFlowCreated,
				approval.EventTypeFlowUpdated,
				approval.EventTypeFlowDeployed,
				approval.EventTypeFlowToggled,
				approval.EventTypeFlowPublished,
			}

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
