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
	projector InstanceProjector
	hooks     []approval.InstanceLifecycleHook
}

// InstanceProjector advances the durable business projection after every
// instance status transition. It runs inside the caller's transaction.
type InstanceProjector interface {
	// Project records or applies the instance's latest complete business state.
	Project(ctx context.Context, db orm.DB, instance *approval.Instance) error
}

// NewLifecycleHookRunner constructs a runner from the engine projector and FX
// group of host hooks.
func NewLifecycleHookRunner(projector InstanceProjector, hooks []approval.InstanceLifecycleHook) *LifecycleHookRunner {
	return &LifecycleHookRunner{projector: projector, hooks: hooks}
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

// OnInstanceTransition advances the engine-owned business projection, then
// invokes every registered hook's OnInstanceTransition — hooks always observe
// the business state as the projector left it.
func (r *LifecycleHookRunner) OnInstanceTransition(ctx context.Context, db orm.DB, instance *approval.Instance, from, to approval.InstanceStatus) error {
	if r == nil {
		return nil
	}

	if r.projector != nil {
		if err := r.projector.Project(ctx, db, instance); err != nil {
			return fmt.Errorf("business projection: %w", err)
		}
	}

	for i, h := range r.hooks {
		if err := h.OnInstanceTransition(ctx, db, instance, from, to); err != nil {
			return fmt.Errorf("lifecycle hook[%d].OnInstanceTransition: %w", i, err)
		}
	}

	return nil
}
