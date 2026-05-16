package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ApplyInstanceTransitionWithHooks is the single write-side primitive for
// instance status transitions. It validates the transition through
// InstanceStateMachine, applies it atomically via an optimistic-lock UPDATE
// (WHERE pk AND status=from), and — when the new status is final and hooks
// is non-nil — invokes the registered LifecycleHookRunner inside the same
// transaction. All instance-completion paths (engine NodeActionComplete,
// pass-rule rejection, admin terminate, resubmit/withdraw, etc.) funnel
// through this helper so hooks fire consistently.
//
// extraCols lists columns the caller pre-populated on instance and wants
// persisted in the same UPDATE (e.g. "finished_at", "current_node_id",
// "form_data"). The status column is always included.
//
// Returns ErrInvalidTransition when the transition is not declared on the
// state machine, or when zero rows match (concurrent writer already moved
// the row off `from`). The in-memory instance.Status is restored on failure.
// Pass hooks=nil to skip hook invocation (e.g. test fixtures).
func ApplyInstanceTransitionWithHooks(
	ctx context.Context,
	db orm.DB,
	instance *approval.Instance,
	to approval.InstanceStatus,
	hooks *LifecycleHookRunner,
	extraCols ...string,
) error {
	from := instance.Status
	if !InstanceStateMachine.CanTransition(from, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
	}

	instance.Status = to

	cols := []string{"status"}

	for _, c := range extraCols {
		if c == "status" {
			continue
		}

		if !slices.Contains(cols, c) {
			cols = append(cols, c)
		}
	}

	res, err := db.NewUpdate().
		Model(instance).
		Select(cols...).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(instance.ID).
				Equals("status", from)
		}).
		Exec(ctx)
	if err != nil {
		instance.Status = from

		return fmt.Errorf("update instance status: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		instance.Status = from

		return fmt.Errorf("instance status update rows affected: %w", err)
	}

	if affected == 0 {
		instance.Status = from

		return fmt.Errorf("%w: pk=%s from=%s", ErrInvalidTransition, instance.ID, from)
	}

	if hooks == nil || !to.IsFinal() {
		return nil
	}

	if err := hooks.OnInstanceCompleted(ctx, db, instance, to); err != nil {
		return fmt.Errorf("lifecycle hooks on instance completed: %w", err)
	}

	return nil
}
