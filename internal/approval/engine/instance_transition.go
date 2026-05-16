package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ApplyInstanceTransition is the single write-side primitive for instance
// status transitions. It validates the transition through InstanceStateMachine
// and applies it atomically via an optimistic-lock UPDATE (WHERE pk AND
// status=from), so concurrent writers cannot silently overwrite state.
//
// extraCols lists columns the caller pre-populated on instance and wants
// persisted in the same UPDATE (e.g. "finished_at", "current_node_id",
// "form_data"). The status column is always included.
//
// Returns ErrInvalidTransition when the transition is not declared on the
// state machine, or when zero rows match (concurrent writer already moved
// the row off `from`). The in-memory instance.Status is restored on failure.
//
// Engine-side callers use this directly; service/InstanceService is a thin
// wrapper that surfaces a domain-specific error type for the API layer.
func ApplyInstanceTransition(
	ctx context.Context,
	db orm.DB,
	instance *approval.Instance,
	to approval.InstanceStatus,
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

	return nil
}
