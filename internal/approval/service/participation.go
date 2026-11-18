package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

func decisionOptions(userID string, caller approval.CallerContext) TaskContextLoadOptions {
	return TaskContextLoadOptions{OperatorID: userID, RequireOperatorAssignee: true, RequireTaskPending: true, RequireCurrentNode: true, Caller: caller}
}

func validateTaskState(instance *approval.Instance, task *approval.Task, opts TaskContextLoadOptions) error {
	if instance.Status != approval.InstanceRunning {
		return approval.ErrInstanceCompleted
	}

	if opts.RequireOperatorAssignee && task.AssigneeID != opts.OperatorID {
		return approval.ErrNotAssignee
	}

	if opts.RequireTaskPending && task.Status != approval.TaskPending {
		return approval.ErrTaskNotPending
	}

	if opts.RequireCurrentNode && (instance.CurrentNodeID == nil || *instance.CurrentNodeID != task.NodeID) {
		return approval.ErrTaskNotPending
	}

	return nil
}

// ResolveParticipation derives roles and actionable IDs from instance-scoped rows.
// Detail queries can reuse their already loaded rows.
func ResolveParticipation(inst *approval.Instance, tasks []approval.Task, hasCC bool, userID string) *approval.Participation {
	p := &approval.Participation{Roles: []approval.ParticipantRole{}, ActionableTaskIDs: []string{}}
	if userID == "" {
		return p
	}

	if inst.ApplicantID == userID {
		p.Roles = append(p.Roles, approval.ParticipantApplicant)
	}

	var assignee, delegator bool

	opts := decisionOptions(userID, approval.SystemCaller)
	for i := range tasks {
		task := &tasks[i]
		assignee = assignee || task.AssigneeID == userID

		delegator = delegator || task.DelegatorID != nil && *task.DelegatorID == userID
		if validateTaskState(inst, task, opts) == nil {
			p.ActionableTaskIDs = append(p.ActionableTaskIDs, task.ID)
		}
	}

	if assignee {
		p.Roles = append(p.Roles, approval.ParticipantAssignee)
	}

	if delegator {
		p.Roles = append(p.Roles, approval.ParticipantDelegator)
	}

	if hasCC {
		p.Roles = append(p.Roles, approval.ParticipantCCRecipient)
	}

	slices.Sort(p.ActionableTaskIDs)

	return p
}

type participationReader struct{}

// NewParticipationReader supplies a query-only reader, including to handler constructors.
func NewParticipationReader() approval.ParticipationReader { return participationReader{} }

func (participationReader) Participation(ctx context.Context, db orm.DB, in approval.ParticipationInput) (*approval.Participation, error) {
	if db == nil {
		return nil, approval.ErrDBRequired
	}

	if c, ok := ctx.(fiber.Ctx); ok {
		ctx = c.Context()
	}

	inst := approval.Instance{ID: in.InstanceID}
	if err := db.NewSelect().Model(&inst).Select("id", "tenant_id", "applicant_id", "current_node_id", "status").WherePK().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, approval.ErrInstanceNotFound
		}

		return nil, fmt.Errorf("load participation instance: %w", err)
	}

	if !in.Caller.Allows(inst.TenantID) {
		return nil, approval.ErrInstanceNotFound
	}

	var tasks []approval.Task
	if err := db.NewSelect().Model(&tasks).Select("id", "assignee_id", "delegator_id", "status", "node_id").Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", inst.ID).Group(func(cb orm.ConditionBuilder) { cb.Equals("assignee_id", in.UserID).OrEquals("delegator_id", in.UserID) })
	}).Scan(ctx); err != nil {
		return nil, err
	}

	hasCC, err := db.NewSelect().Model((*approval.CCRecord)(nil)).Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID).Equals("cc_user_id", in.UserID) }).Exists(ctx)
	if err != nil {
		return nil, err
	}

	return ResolveParticipation(&inst, tasks, hasCC, in.UserID), nil
}
