package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// LifecycleHookRunner aggregates host-registered InstanceLifecycleHook
// implementations and invokes them in registration order. A non-nil error
// short-circuits the chain so the caller can roll back the surrounding
// transaction.
//
// The runner is intentionally minimal: hosts that want fan-out or async
// dispatch should subscribe to the corresponding domain events instead;
// hooks are reserved for cases that must run inside the business
// transaction.
type LifecycleHookRunner struct {
	hooks []approval.InstanceLifecycleHook
}

// NewLifecycleHookRunner constructs a runner from the FX group of hooks.
func NewLifecycleHookRunner(hooks []approval.InstanceLifecycleHook) *LifecycleHookRunner {
	return &LifecycleHookRunner{hooks: hooks}
}

// OnInstanceCreated invokes every registered hook's OnInstanceCreated.
func (r *LifecycleHookRunner) OnInstanceCreated(ctx context.Context, db orm.DB, instance *approval.Instance) error {
	for i, h := range r.hooks {
		if err := h.OnInstanceCreated(ctx, db, instance); err != nil {
			return fmt.Errorf("lifecycle hook[%d].OnInstanceCreated: %w", i, err)
		}
	}

	return nil
}

// OnInstanceCompleted invokes every registered hook's OnInstanceCompleted.
func (r *LifecycleHookRunner) OnInstanceCompleted(ctx context.Context, db orm.DB, instance *approval.Instance, finalStatus approval.InstanceStatus) error {
	for i, h := range r.hooks {
		if err := h.OnInstanceCompleted(ctx, db, instance, finalStatus); err != nil {
			return fmt.Errorf("lifecycle hook[%d].OnInstanceCompleted: %w", i, err)
		}
	}

	return nil
}
