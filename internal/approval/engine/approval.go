package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
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
	if result, handled := resolveAutoExecution(ctx, pc); handled {
		return result, nil
	}

	if err := saveFormSnapshot(ctx, pc); err != nil {
		return nil, err
	}

	assignees, err := resolveNodeAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return handleEmptyAssignee(ctx, pc, p.assigneeService)
	}

	if p.isSameApplicant(assignees, pc.ApplicantID) {
		return p.handleSameApplicant(ctx, pc, assignees)
	}

	events, err := p.createApprovalTasks(ctx, pc, assignees)
	if err != nil {
		return nil, err
	}

	if pc.Node.ConsecutiveApproverAction == approval.ConsecutiveApproverAutoPass {
		result, err := p.autoPassConsecutiveApprovers(ctx, pc)
		if err != nil {
			return nil, err
		}

		// Creation events precede auto-pass events so downstream
		// subscribers observe the natural lifecycle order.
		result.Events = append(events, result.Events...)

		return result, nil
	}

	return &ProcessResult{Action: NodeActionWait, Events: events}, nil
}

// createApprovalTasks creates tasks with sequential ordering support and
// returns one TaskCreatedEvent per inserted task in insertion order.
// Sequential tasks after the first are created as TaskWaiting with a nil
// deadline; subscribers can use those fields to distinguish queued tasks
// from immediately actionable ones.
func (*ApprovalProcessor) createApprovalTasks(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) ([]approval.DomainEvent, error) {
	events := make([]approval.DomainEvent, 0, len(assignees))

	for i, assignee := range assignees {
		deadline := computeDeadline(pc.Node)
		task := buildTask(pc, assignee, deadline)

		if pc.Node.ApprovalMethod == approval.ApprovalSequential {
			task.SortOrder = i + 1

			if i > 0 {
				task.Status = approval.TaskWaiting
				task.Deadline = nil
			}
		}

		if _, err := pc.DB.NewInsert().Model(task).Exec(ctx); err != nil {
			return nil, fmt.Errorf("create approval task: %w", err)
		}

		events = append(events, newTaskCreatedEvent(pc, task))
	}

	return events, nil
}

func (p *ApprovalProcessor) handleSameApplicant(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) (*ProcessResult, error) {
	switch pc.Node.SameApplicantAction {
	case approval.SameApplicantAutoPass:
		return nodeAutoPassResult(ctx, pc, autoPassReasonSameApplicant), nil

	case approval.SameApplicantTransferSuperior:
		superiorInfo, err := getSuperior(ctx, p.assigneeService, pc.ApplicantID)
		if err != nil {
			return nil, err
		}

		if superiorInfo == nil || superiorInfo.ID == "" {
			return nil, shared.ErrNoAssignee
		}

		return createTasksForUsers(ctx, pc, []string{superiorInfo.ID})

	default: // includes SameApplicantSelfApprove and other unrecognized actions
		events, err := createTasksWithDelegation(ctx, pc, assignees)
		if err != nil {
			return nil, err
		}

		return &ProcessResult{Action: NodeActionWait, Events: events}, nil
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

	// FOR UPDATE serializes concurrent writers on the same node so the
	// in-memory task[] view used by the cascading-activation loop below
	// stays consistent with the database; without it, a parallel update
	// could change task[j] between our activate attempt and our retry,
	// causing us to skip an assignee that should auto-pass.
	if err := pc.DB.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", pc.Instance.ID).
				Equals("node_id", pc.Node.ID)
		}).
		OrderBy("sort_order").
		ForUpdate().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks for consecutive approver check: %w", err)
	}

	now := timex.Now()
	autoPassedAny := false

	var events []approval.DomainEvent

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

		// The SELECT ... FOR UPDATE above holds the row lock for this tx, and
		// the in-memory guard already established status == pending, so the
		// status="pending" CAS predicate is guaranteed to match: no concurrent
		// writer can flip the row between the locked read and this update. The
		// predicate is retained as a defense-in-depth invariant assertion.
		if _, err := pc.DB.NewUpdate().
			Model(task).
			Select("status", "finished_at").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(task.ID).
					Equals("status", string(approval.TaskPending))
			}).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("auto-pass consecutive approver task: %w", err)
		}

		autoPassedAny = true

		// The pass happened without the approver acting, so it must leave the
		// same audit trail a manual approval would: a task-approved event
		// (system-operated) and an action log entry.
		events = append(events, approval.NewTaskApprovedEvent(
			task.ID, task.TenantID, pc.Instance.ID, pc.Node.ID,
			shared.SystemOperator.ID, autoPassReasonConsecutiveApprover,
		))
		recordSystemActionLog(ctx, pc, task, autoPassReasonConsecutiveApprover)

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
						// A concurrent writer already advanced this row; fully revert
						// the optimistic in-memory mutation, deadline included, so the
						// reverted task keeps the waiting invariant (nil deadline).
						tasks[j].Status = approval.TaskWaiting
						tasks[j].Deadline = nil
					}

					break
				}
			}
		}
	}

	if !autoPassedAny {
		return &ProcessResult{Action: NodeActionWait}, nil
	}

	// If all tasks are now complete, advance to the next node.
	//
	// Entry-time auto-pass paths (consecutive-approver here, plus
	// same-applicant / empty-assignee / execution) intentionally do NOT fire
	// timing-based node CC (CCTimingOnApprove): they return NodeActionContinue
	// so the engine advances directly, bypassing service.HandleNodeCompletion
	// where TriggerNodeCC(PassRulePassed) lives. The engine cannot call into
	// the service layer (service imports engine, not vice versa), and a node
	// cleared without any human approval has no approver action to notify CC
	// about. This suppression is uniform across all entry-time auto-pass paths
	// and is pinned by TestConsecutiveApproverAutoPass.
	allComplete := !slices.ContainsFunc(tasks, func(t approval.Task) bool {
		return t.Status == approval.TaskPending || t.Status == approval.TaskWaiting
	})

	if allComplete {
		return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
	}

	return &ProcessResult{Action: NodeActionWait, Events: events}, nil
}

func (*ApprovalProcessor) isSameApplicant(assignees []approval.ResolvedAssignee, applicantID string) bool {
	if len(assignees) == 0 {
		return false
	}

	return !slices.ContainsFunc(assignees, func(a approval.ResolvedAssignee) bool {
		return a.User.ID != applicantID
	})
}
