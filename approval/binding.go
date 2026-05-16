package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// BusinessBindingHook bridges the approval engine with the host application's
// business tables when Flow.BindingMode is BindingBusiness.
//
// Two lifecycle moments matter:
//
//   - OnInstanceCreated is called inside the start_instance transaction so
//     the host can resolve / create the business row and return the primary
//     key that the engine stores in Instance.BusinessRecordID. Returning an
//     error rolls back the entire instance creation.
//
//   - OnInstanceCompleted is called inside the engine completion transaction
//     after the state-machine transition succeeds. It writes the final
//     decision back to Flow.BusinessTable / BusinessStatusField. A non-nil
//     error does NOT roll back the approval — the workflow has already
//     decided. Instead the engine emits InstanceBindingFailedEvent so the
//     host can retry asynchronously (via outbox / saga compensation).
//
// Hosts override the default implementation by binding their own
// BusinessBindingHook into the FX container, typically through
// vef.SupplyBusinessBindingHook.
type BusinessBindingHook interface {
	// OnInstanceCreated returns the business primary key (BusinessRecordID)
	// to persist on the instance. Returning empty string indicates the host
	// has nothing to bind (engine stores nil).
	OnInstanceCreated(ctx context.Context, db orm.DB, flow *Flow, instance *Instance) (businessRecordID string, err error)
	// OnInstanceCompleted writes the final approval status back to the
	// business table. Implementations should be idempotent — the engine may
	// retry through the outbox.
	OnInstanceCompleted(ctx context.Context, db orm.DB, flow *Flow, instance *Instance, finalStatus InstanceStatus) error
}
