package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ApprovalProcessor handles approval nodes.
type ApprovalProcessor struct {
	assigneeService approval.AssigneeService
}

// NewApprovalProcessor creates a new approval processor.
func NewApprovalProcessor(assigneeService approval.AssigneeService) *ApprovalProcessor {
	return &ApprovalProcessor{assigneeService: assigneeService}
}

func (*ApprovalProcessor) NodeKind() approval.NodeKind { return approval.NodeApproval }

func (p *ApprovalProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	if err := saveFormSnapshot(ctx, pc); err != nil {
		return nil, err
	}

	assignees, err := p.resolveAndProcessAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return handleEmptyAssignee(ctx, pc, p.assigneeService)
	}

	if p.isSameApplicant(assignees, pc.ApplicantID) {
		return p.handleSameApplicant(ctx, pc, assignees)
	}

	if err := p.createApprovalTasks(ctx, pc, assignees); err != nil {
		return nil, err
	}

	if pc.Node.ConsecutiveApproverAction == approval.ConsecutiveApproverAutoPass {
		return p.autoPassConsecutiveApprovers(ctx, pc)
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

func (*ApprovalProcessor) resolveAndProcessAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	assignees, err := resolveAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	assignees = deduplicateAssignees(assignees)

	assignees, err = applyDelegation(ctx, pc.DB, pc.Instance.FlowID, assignees, pc.UserResolver)
	if err != nil {
		return nil, err
	}

	return assignees, nil
}

// createApprovalTasks creates tasks with sequential ordering support.
func (*ApprovalProcessor) createApprovalTasks(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) error {
	for i, assignee := range assignees {
		sortOrder := 0
		status := approval.TaskPending
		deadline := computeDeadline(pc.Node)

		if pc.Node.ApprovalMethod == approval.ApprovalSequential {
			sortOrder = i + 1

			if i > 0 {
				status = approval.TaskWaiting
				deadline = nil
			}
		}

		task := &approval.Task{
			TenantID:      pc.Instance.TenantID,
			InstanceID:    pc.Instance.ID,
			NodeID:        pc.Node.ID,
			AssigneeID:    assignee.UserID,
			AssigneeName:  assignee.UserName,
			DelegatorID:   assignee.DelegatorID,
			DelegatorName: assignee.DelegatorName,
			SortOrder:     sortOrder,
			Status:        status,
			Deadline:      deadline,
		}

		if _, err := pc.DB.NewInsert().Model(task).Exec(ctx); err != nil {
			return fmt.Errorf("create approval task: %w", err)
		}
	}

	return nil
}

func (p *ApprovalProcessor) handleSameApplicant(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) (*ProcessResult, error) {
	switch pc.Node.SameApplicantAction {
	case approval.SameApplicantAutoPass:
		return &ProcessResult{Action: NodeActionContinue}, nil

	case approval.SameApplicantTransferSuperior:
		superiorInfo, err := getSuperior(ctx, p.assigneeService, pc.ApplicantID)
		if err != nil {
			return nil, err
		}

		if superiorInfo == nil || superiorInfo.ID == "" {
			return nil, ErrNoAssignee
		}

		return createTasksForUsers(ctx, pc, []string{superiorInfo.ID})

	default: // includes SameApplicantSelfApprove and other unrecognized actions
		if err := createTasksWithDelegation(ctx, pc, assignees); err != nil {
			return nil, err
		}

		return &ProcessResult{Action: NodeActionWait}, nil
	}
}

// autoPassConsecutiveApprovers marks tasks as approved for assignees who already
// approved in the immediately preceding approval node.
func (*ApprovalProcessor) autoPassConsecutiveApprovers(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	prevApprovers, err := findPreviousApprovalApprovers(ctx, pc.DB, pc.Instance, pc.Node.ID)
	if err != nil {
		return nil, err
	}

	if prevApprovers.Size() == 0 {
		return &ProcessResult{Action: NodeActionWait}, nil
	}

	var tasks []approval.Task

	if err := pc.DB.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", pc.Instance.ID).
				Equals("node_id", pc.Node.ID)
		}).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks for consecutive approver check: %w", err)
	}

	now := timex.Now()
	autoPassedAny := false

	for i := range tasks {
		task := &tasks[i]

		if !prevApprovers.Contains(task.AssigneeID) {
			continue
		}

		// For parallel approval, only auto-pass pending tasks.
		// For sequential approval, also handle waiting tasks that become pending via cascading activation.
		if task.Status != approval.TaskPending {
			continue
		}

		task.Status = approval.TaskApproved
		task.FinishedAt = new(now)

		res, err := pc.DB.NewUpdate().
			Model(task).
			Select("status", "finished_at").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(task.ID).
					Equals("status", string(approval.TaskPending))
			}).
			Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("auto-pass consecutive approver task: %w", err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("auto-pass rows affected: %w", err)
		}

		if affected == 0 {
			continue
		}

		autoPassedAny = true

		// For sequential approval, activate the next waiting task.
		// The outer loop will then check if this newly activated task
		// also qualifies for auto-pass (cascading).
		if pc.Node.ApprovalMethod == approval.ApprovalSequential {
			for j := i + 1; j < len(tasks); j++ {
				if tasks[j].Status == approval.TaskWaiting {
					tasks[j].Status = approval.TaskPending
					tasks[j].Deadline = computeDeadline(pc.Node)

					activateRes, activateErr := pc.DB.NewUpdate().
						Model(&tasks[j]).
						Select("status", "deadline").
						Where(func(cb orm.ConditionBuilder) {
							cb.PKEquals(tasks[j].ID).
								Equals("status", string(approval.TaskWaiting))
						}).
						Exec(ctx)
					if activateErr != nil {
						return nil, fmt.Errorf("activate next sequential task: %w", activateErr)
					}

					activateAffected, activateErr := activateRes.RowsAffected()
					if activateErr != nil {
						return nil, fmt.Errorf("activate rows affected: %w", activateErr)
					}

					if activateAffected == 0 {
						tasks[j].Status = approval.TaskWaiting
					}

					break
				}
			}
		}
	}

	if !autoPassedAny {
		return &ProcessResult{Action: NodeActionWait}, nil
	}

	// If all tasks are now complete, advance to the next node
	allComplete := !slices.ContainsFunc(tasks, func(t approval.Task) bool {
		return t.Status == approval.TaskPending || t.Status == approval.TaskWaiting
	})

	if allComplete {
		return &ProcessResult{Action: NodeActionContinue}, nil
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

func (*ApprovalProcessor) isSameApplicant(assignees []approval.ResolvedAssignee, applicantID string) bool {
	if len(assignees) == 0 {
		return false
	}

	return !slices.ContainsFunc(assignees, func(a approval.ResolvedAssignee) bool {
		return a.UserID != applicantID
	})
}
