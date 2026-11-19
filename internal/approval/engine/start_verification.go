package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// EnterInstance traverses the start node, then lets the pinned business kind
// verify precisely the visits of this traversal, even across nested commands.
func (e *FlowEngine) EnterInstance(ctx context.Context, db orm.DB, reg *business.Registry, instance *approval.Instance, version *approval.FlowVersion, resubmission bool) error {
	if instance.BusinessKind == nil {
		return e.StartProcess(ctx, db, instance)
	}

	h, _ := reg.Lookup(*instance.BusinessKind)

	verifier, ok := h.(approval.StartVerifier)
	if !ok {
		return e.StartProcess(ctx, db, instance)
	}

	var visits []string
	if err := e.startProcess(ctx, db, instance, &visits); err != nil {
		return err
	}

	tasks := []approval.Task{}

	err := db.NewSelect().
		Model(&tasks).
		Join((*approval.NodeVisit)(nil), func(cb orm.ConditionBuilder) { cb.EqualsColumn("anv.id", "at.visit_id") }).
		Where(func(cb orm.ConditionBuilder) { cb.In("at.visit_id", visits) }).
		OrderBy("anv.sequence", "at.sort_order", "at.id").
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("load tasks for start verification: %w", err)
	}

	return verifier.VerifyStart(ctx, db, &approval.StartVerification{Instance: instance, Version: version, Tasks: tasks, IsResubmission: resubmission})
}
