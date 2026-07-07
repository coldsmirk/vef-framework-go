package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// InstanceLifecycleHook is a synchronous extension point for host
// applications that need to react to approval lifecycle moments inside the
// same transaction as the business change. Unlike event subscriptions —
// which are asynchronous, retryable through outbox, and fire after commit —
// hooks run *during* the transaction, so a hook that returns an error
// aborts the surrounding business operation.
//
// Use hooks for invariants that must hold within the transaction (e.g.
// allocating a business row, writing a tightly-coupled record). Use event
// subscriptions for everything else (webhooks, notifications, analytics,
// async integrations).
//
// Multiple implementations are aggregated via FX group
// `group:"vef:approval:lifecycle_hooks"` and invoked in registration order;
// any non-nil error stops further hooks and bubbles back to the caller.
type InstanceLifecycleHook interface {
	// OnInstanceCreated runs after the instance row is persisted and the
	// initial action log is written, but before the engine advances to
	// the first node. Returning an error rolls back start_instance.
	OnInstanceCreated(ctx context.Context, db orm.DB, instance *Instance) error
	// OnInstanceCompleted runs after the engine applies the final state
	// transition (within the same transaction). Returning an error rolls
	// back the completion. For at-most-once / fire-and-forget side effects
	// (webhooks, notifications), subscribe to InstanceCompletedEvent
	// instead so the outbox can guarantee delivery.
	OnInstanceCompleted(ctx context.Context, db orm.DB, instance *Instance, finalStatus InstanceStatus) error
}

// NewFilteredLifecycleHook wraps a hook with the same declarative routing
// filters used by SubscribeInstance, so the hook only sees instances it
// declared interest in (non-matching instances pass through as a no-op).
// Hooks are global by default; hosts whose hook serves a single flow wrap it
// at registration:
//
//	vef.ProvideApprovalLifecycleHook(func() approval.InstanceLifecycleHook {
//	    return approval.NewFilteredLifecycleHook(newRightApplicationHook(), approval.ForFlows("right_application"))
//	})
//
// No filters returns the hook unchanged.
func NewFilteredLifecycleHook(hook InstanceLifecycleHook, filters ...InstanceFilter) InstanceLifecycleHook {
	if len(filters) == 0 {
		return hook
	}

	return &filteredLifecycleHook{inner: hook, filters: filters}
}

type filteredLifecycleHook struct {
	inner   InstanceLifecycleHook
	filters []InstanceFilter
}

// OnInstanceCreated forwards to the wrapped hook when the instance matches
// every filter.
func (h *filteredLifecycleHook) OnInstanceCreated(ctx context.Context, db orm.DB, instance *Instance) error {
	if !matchesAll(h.filters, instance.FlowCode, instance.TenantID) {
		return nil
	}

	return h.inner.OnInstanceCreated(ctx, db, instance)
}

// OnInstanceCompleted forwards to the wrapped hook when the instance matches
// every filter.
func (h *filteredLifecycleHook) OnInstanceCompleted(ctx context.Context, db orm.DB, instance *Instance, finalStatus InstanceStatus) error {
	if !matchesAll(h.filters, instance.FlowCode, instance.TenantID) {
		return nil
	}

	return h.inner.OnInstanceCompleted(ctx, db, instance, finalStatus)
}
