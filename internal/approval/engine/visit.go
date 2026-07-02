package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// beginNodeVisit records the engine entering a node: it inserts an active
// apv_node_visit row whose Sequence is the next step number within the
// instance. Every ProcessNode call — initial start, advance, rollback
// re-entry — begins exactly one visit; the row is concluded by whichever path
// decides the node's outcome (handleProcessResult for auto-advancing nodes,
// HandleNodeCompletion for human task nodes, the rollback / withdraw /
// terminate / CC-read paths for the rest). Callers hold the instance row lock,
// which serializes the count-then-insert against concurrent visits.
func beginNodeVisit(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) (*approval.NodeVisit, error) {
	count, err := db.NewSelect().
		Model((*approval.NodeVisit)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID)
		}).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count node visits: %w", err)
	}

	visit := &approval.NodeVisit{
		TenantID:   instance.TenantID,
		InstanceID: instance.ID,
		NodeID:     node.ID,
		Sequence:   int(count) + 1,
		Status:     approval.NodeVisitActive,
	}

	if _, err := db.NewInsert().Model(visit).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert node visit: %w", err)
	}

	return visit, nil
}

// concludeNodeVisit stamps the outcome and finish time on the visit the engine
// holds in memory — the one ProcessNode just began.
func concludeNodeVisit(ctx context.Context, db orm.DB, visit *approval.NodeVisit, status approval.NodeVisitStatus) error {
	visit.Status = status
	visit.FinishedAt = new(timex.Now())

	if _, err := db.NewUpdate().
		Model(visit).
		Select("status", "finished_at").
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("conclude node visit: %w", err)
	}

	return nil
}

// findActiveNodeVisit returns the node's open visit. The node must be
// executing when this is called, so a missing visit is a broken invariant and
// surfaces as an error rather than a silent fallback.
func findActiveNodeVisit(ctx context.Context, db orm.DB, instanceID, nodeID string) (*approval.NodeVisit, error) {
	var visit approval.NodeVisit

	err := db.NewSelect().
		Model(&visit).
		Select("id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				Equals("status", approval.NodeVisitActive)
		}).
		Scan(ctx)
	if err != nil {
		if result.IsRecordNotFound(err) {
			return nil, fmt.Errorf("%w: instance=%s node=%s", ErrActiveVisitNotFound, instanceID, nodeID)
		}

		return nil, fmt.Errorf("find active node visit: %w", err)
	}

	return &visit, nil
}

// ConcludeActiveNodeVisit stamps the outcome on the active visit of the given
// node. It is the conclusion primitive for paths that do not hold the visit in
// memory: pass-rule completion (HandleNodeCompletion), CC read-confirm
// advancement, and rollback. All of them act on an executing node, so exactly
// one row must match — anything else is a broken invariant.
func ConcludeActiveNodeVisit(ctx context.Context, db orm.DB, instanceID, nodeID string, status approval.NodeVisitStatus) error {
	res, err := db.NewUpdate().
		Model((*approval.NodeVisit)(nil)).
		Set("status", status).
		Set("finished_at", timex.Now()).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				Equals("status", approval.NodeVisitActive)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("conclude active node visit: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("conclude active node visit rows affected: %w", err)
	}

	if affected != 1 {
		return fmt.Errorf("%w: instance=%s node=%s", ErrActiveVisitNotFound, instanceID, nodeID)
	}

	return nil
}

// CancelActiveNodeVisits concludes every active visit of the instance as
// canceled — withdraw and terminate cut the traversal short wherever it
// stands. A no-op when nothing is active (e.g. terminating an already-paused
// instance).
func CancelActiveNodeVisits(ctx context.Context, db orm.DB, instanceID string) error {
	if _, err := db.NewUpdate().
		Model((*approval.NodeVisit)(nil)).
		Set("status", approval.NodeVisitCanceled).
		Set("finished_at", timex.Now()).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("status", approval.NodeVisitActive)
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("cancel active node visits: %w", err)
	}

	return nil
}
