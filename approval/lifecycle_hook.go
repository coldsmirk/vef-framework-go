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
// `group:"vef:approval:lifecycle_hooks"`. The invocation order is
// unspecified — FX value groups carry no ordering — so hooks must be
// mutually independent; any non-nil error stops the remaining hooks and
// bubbles back to the caller.
type InstanceLifecycleHook interface {
	// OnInstanceCreated runs after the instance row is persisted, but before
	// the engine advances to the first node. The initial submit action log is
	// only buffered at this point — audit rows are batch-flushed after the
	// command handler returns — so the hook must not expect to read it.
	// Returning an error rolls back start_instance.
	OnInstanceCreated(ctx context.Context, db orm.DB, instance *Instance) error
	// OnInstanceTransition runs inside the same transaction as every
	// instance status transition — completion (to.IsFinal()), return,
	// withdrawal, resubmission, termination — after the engine-owned
	// business projection has recorded (and, in synchronous mode, applied)
	// the new state, so the hook observes the business table as the
	// transition leaves it. instance.Status already carries to. Returning
	// an error rolls back the whole transition. For side effects that need
	// no transactional coupling, subscribe to the corresponding instance
	// events (or bridge them into commands via BindCommand) instead.
	OnInstanceTransition(ctx context.Context, db orm.DB, instance *Instance, from, to InstanceStatus) error
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

// OnInstanceTransition forwards to the wrapped hook when the instance matches
// every filter.
func (h *filteredLifecycleHook) OnInstanceTransition(ctx context.Context, db orm.DB, instance *Instance, from, to InstanceStatus) error {
	if !matchesAll(h.filters, instance.FlowCode, instance.TenantID) {
		return nil
	}

	return h.inner.OnInstanceTransition(ctx, db, instance, from, to)
}
