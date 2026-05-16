package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// InstanceService is the single write-side entry point for instance status
// transitions. Every status change (approve / reject / withdraw / rollback /
// resubmit / terminate / engine-driven completion) must go through Transition
// so that state-machine validation, the optimistic-lock UPDATE, and the
// host-registered lifecycle hooks fire together. Direct UPDATE statements
// against apv_instance.status are a bug.
type InstanceService struct {
	hooks *engine.LifecycleHookRunner
}

// NewInstanceService creates a new InstanceService. hooks may be nil in
// test fixtures; production wiring always supplies the engine's
// LifecycleHookRunner so completion transitions invoke registered
// extensions inside the same tx as the status change.
func NewInstanceService(hooks *engine.LifecycleHookRunner) *InstanceService {
	return &InstanceService{hooks: hooks}
}

// LoadForUpdate loads an instance by ID with a row-level lock and asserts
// that the caller is authorized to act on it. Cross-tenant access surfaces
// as ErrInstanceNotFound — the same response shape as "no such instance"
// — so the API never reveals existence across tenants.
//
// caller may be a zero CallerContext (system-internal call, test fixtures);
// in that case Authorize is permissive and tenant enforcement is skipped.
// Production resource paths always populate it; making the parameter
// mandatory means "any tenant-scoped load goes through this guard" is a
// compile-time invariant, not a code-review hope.
func (*InstanceService) LoadForUpdate(
	ctx context.Context,
	db orm.DB,
	instanceID string,
	caller approval.CallerContext,
) (*approval.Instance, error) {
	instance := &approval.Instance{}
	instance.ID = instanceID

	if err := db.NewSelect().
		Model(instance).
		ForUpdate().
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrInstanceNotFound
		}

		return nil, fmt.Errorf("load instance: %w", err)
	}

	if !caller.Allows(instance.TenantID) {
		return nil, shared.ErrInstanceNotFound
	}

	return instance, nil
}

// Transition validates the instance status transition through the state
// machine, applies it atomically with an optimistic-lock UPDATE, and
// invokes lifecycle hooks when the new status is final.
//
// extraCols lists additional columns the caller pre-populated on instance
// and wants persisted in the same UPDATE (e.g. "finished_at",
// "current_node_id", "form_data"). The status column is always included.
//
// If a concurrent writer already advanced the status, the UPDATE matches
// zero rows and Transition returns ErrInvalidInstanceTransition with the
// in-memory status restored to the pre-call value.
func (s *InstanceService) Transition(
	ctx context.Context,
	db orm.DB,
	instance *approval.Instance,
	to approval.InstanceStatus,
	extraCols ...string,
) error {
	err := engine.ApplyInstanceTransitionWithHooks(ctx, db, instance, to, s.hooks, extraCols...)
	if err == nil {
		return nil
	}

	// Engine returns a wrapped ErrInvalidTransition; surface the domain
	// sentinel so callers can branch on a stable error and the API layer
	// gets the right error code.
	if errors.Is(err, engine.ErrInvalidTransition) {
		return shared.ErrInvalidInstanceTransition
	}

	return fmt.Errorf("apply instance transition: %w", err)
}
