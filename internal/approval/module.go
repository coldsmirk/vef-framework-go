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
// bus is not configured to deliver approval business events via a
// transactional transport. EventPublishBehavior and engine.PublishEventsTx
// publish with event.WithTx so consumers see the event iff the originating
// business transaction commits; without a transactional route the first
// publish would fail at runtime with event.ErrTxRequired.
//
// InstanceBindingFailedEvent is deliberately excluded: it is emitted by
// the asynchronous binding listener outside any business transaction, so
// requiring a transactional route for it would force misconfiguration on
// hosts that legitimately route only binding_failed through non-tx paths.
//
// The check itself is deferred to OnStart so the bus has built its
// router by the time we query it (bus.Start runs first in the lifecycle
// order — see bootstrap module ordering).
func verifyEventRouting(lc fx.Lifecycle, inspector event.RouteInspector) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			required := []string{
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

			for _, et := range required {
				if !inspector.HasTransactionalRoute(et) {
					return fmt.Errorf(
						"%w: %q (enable vef.event.transports.outbox.enabled=true and add a "+
							"routing rule for pattern \"approval.*\" -> [\"outbox\", \"memory\"] "+
							"or [\"outbox\", \"redis_stream\"] with the matching outbox sink)",
						ErrEventRouteNotTransactional, et)
				}
			}

			return nil
		},
	})
}
