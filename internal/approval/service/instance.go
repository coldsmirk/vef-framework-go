package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// InstanceService is the single write-side entry point for instance status
// transitions. Every status change (approve / reject / withdraw / rollback /
// resubmit / terminate / engine-driven completion) must go through Transition
// so that state-machine validation and the optimistic-lock UPDATE happen
// together. Direct UPDATE statements against apv_instance.status are a bug.
type InstanceService struct{}

// NewInstanceService creates a new InstanceService.
func NewInstanceService() *InstanceService { return new(InstanceService) }

// Transition validates the instance status transition through the state
// machine and applies it atomically with an optimistic-lock UPDATE.
//
// extraCols lists additional columns the caller pre-populated on instance
// and wants persisted in the same UPDATE (e.g. "finished_at",
// "current_node_id", "form_data"). The status column is always included.
//
// If a concurrent writer already advanced the status, the UPDATE matches
// zero rows and Transition returns ErrInvalidInstanceTransition with the
// in-memory status restored to the pre-call value.
func (*InstanceService) Transition(
	ctx context.Context,
	db orm.DB,
	instance *approval.Instance,
	to approval.InstanceStatus,
	extraCols ...string,
) error {
	err := engine.ApplyInstanceTransition(ctx, db, instance, to, extraCols...)
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
