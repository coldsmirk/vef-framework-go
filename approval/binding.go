package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// BusinessBindingHook bridges the approval engine with the host application's
// business tables when Flow.BindingMode is BindingBusiness. It is narrowly
// scoped to the "approval row ↔ business row" plumbing; broader lifecycle
// extension goes through InstanceLifecycleHook instead.
//
// Two lifecycle moments matter:
//
//   - OnInstanceCreated runs inside the start_instance transaction so the
//     host can resolve / create the business row and return the primary
//     key that the engine stores in Instance.BusinessRecordID. Returning
//     an error rolls back the entire instance creation.
//
//   - WriteBackStatus runs asynchronously via the binding Listener (which
//     subscribes to InstanceCompletedEvent) so the host can stamp the
//     final approval decision onto its own business table. A non-nil
//     error does NOT roll back the approval — the workflow has already
//     decided. Instead the listener publishes InstanceBindingFailedEvent
//     so the host can retry (saga / outbox compensation).
//
// Naming note: this method used to be called OnInstanceCompleted, which
// collided semantically with InstanceLifecycleHook.OnInstanceCompleted —
// the two hooks have different execution models (async via listener vs
// sync inside the engine tx). The current name makes the single
// responsibility explicit.
//
// Hosts override the default implementation by binding their own
// BusinessBindingHook into the FX container, typically through
// vef.SupplyBusinessBindingHook.
type BusinessBindingHook interface {
	// OnInstanceCreated returns the business primary key (BusinessRecordID)
	// to persist on the instance. Returning empty string indicates the host
	// has nothing to bind (engine stores nil).
	OnInstanceCreated(ctx context.Context, db orm.DB, flow *Flow, instance *Instance) (businessRecordID string, err error)
	// WriteBackStatus writes the final approval status back to the
	// business table. Called asynchronously from the binding Listener
	// after InstanceCompletedEvent fires. Implementations should be
	// idempotent — the listener may retry through the outbox.
	WriteBackStatus(ctx context.Context, db orm.DB, flow *Flow, instance *Instance, finalStatus InstanceStatus) error
}
