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

	assignees, decided, err := p.applySameApplicantPolicy(ctx, pc, assignees)
	if err != nil {
		return nil, err
	}

	if decided != nil {
		return decided, nil
	}

	events, err := p.createApprovalTasks(ctx, pc, assignees)
	if err != nil {
		return nil, err
	}

	rule, err := p.entryAutoPassRule(ctx, pc, assignees)
	if err != nil {
		return nil, err
	}

	if rule == nil {
		return &ProcessResult{Action: NodeActionWait, Events: events}, nil
	}

	// The creation events go in so the auto-pass pass can retract the
	// activation of a task it clears — one created Pending and immediately
	// auto-passed never needed its assignee to act either.
	return p.autoPassEligibleTasks(ctx, pc, events, rule)
}

// createApprovalTasks creates tasks with sequential ordering support and
// returns their lifecycle events in insertion order. Sequential tasks after
// the first are created as TaskWaiting with a nil deadline, so only the first
// is announced as activated here; the rest are activated as the queue reaches
// them.
func (*ApprovalProcessor) createApprovalTasks(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) ([]approval.DomainEvent, error) {
	events := make([]approval.DomainEvent, 0, len(assignees)*2)

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

		events = append(events, taskInsertedEvents(pc, task)...)
	}

	return events, nil
}

// applySameApplicantPolicy applies the node's same-applicant policy to the
// applicant's seat in the resolved assignee set. The policy keys on the
// resolved actor — a seat counts as the applicant's when the person who would
// act on it, after delegation, is the applicant. It either transforms the seat
// list (exclude removes the seat, transfer_superior replaces it) or decides
// the node outright (non-nil result: sole-assignee auto-pass, or an exclusion
// that emptied the set and fell back to EmptyAssigneeAction). Auto-pass over a
// mixed set is settled per task after creation — see entryAutoPassRule.
func (p *ApprovalProcessor) applySameApplicantPolicy(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) ([]approval.ResolvedAssignee, *ProcessResult, error) {
	if !containsApplicant(assignees, pc.Instance.ApplicantID) {
		return assignees, nil, nil
	}

	switch pc.Node.SameApplicantAction {
	case approval.SameApplicantAutoPass:
		// Assignees are deduplicated, so a single seat containing the
		// applicant means they are the only approver: nothing is left to
		// decide and the node passes without tasks.
		if len(assignees) == 1 {
			return nil, nodeAutoPassResult(ctx, pc, AutoPassReasonSameApplicant), nil
		}

		return assignees, nil, nil

	case approval.SameApplicantExclude:
		recordSystemActionLog(ctx, pc, nil, excludeReasonSameApplicant)

		remaining := slices.DeleteFunc(slices.Clone(assignees), func(a approval.ResolvedAssignee) bool {
			return a.User.ID == pc.Instance.ApplicantID
		})

		if len(remaining) == 0 {
			result, err := handleEmptyAssignee(ctx, pc, p.assigneeService)

			return nil, result, err
		}

		return remaining, nil, nil

	case approval.SameApplicantTransferSuperior:
		superior, err := p.resolveSuperiorSeat(ctx, pc)
		if err != nil {
			return nil, nil, err
		}

		replaced := slices.Clone(assignees)
		for i := range replaced {
			if replaced[i].User.ID == pc.Instance.ApplicantID {
				replaced[i] = superior
			}
		}

		return deduplicateAssignees(replaced), nil, nil

	default: // SameApplicantSelfApprove and unrecognized values: the applicant approves like any other assignee.
		return assignees, nil, nil
	}
}

// resolveSuperiorSeat resolves the applicant's superior into a seat, with the
// person snapshot taken through the canonical UserInfoResolver.
func (p *ApprovalProcessor) resolveSuperiorSeat(ctx context.Context, pc *ProcessContext) (approval.ResolvedAssignee, error) {
	superiorInfo, err := getSuperior(ctx, p.assigneeService, pc.Instance.ApplicantID)
	if err != nil {
		return approval.ResolvedAssignee{}, err
	}

	if superiorInfo == nil || superiorInfo.ID == "" {
		return approval.ResolvedAssignee{}, approval.ErrNoAssignee
	}

	infos, err := shared.ResolveUserInfoMap(ctx, pc.UserResolver, []string{superiorInfo.ID})
	if err != nil {
		return approval.ResolvedAssignee{}, fmt.Errorf("resolve superior info: %w", err)
	}

	info := infos[superiorInfo.ID]
	info.ID = superiorInfo.ID

	return approval.ResolvedAssignee{User: info}, nil
}

// autoPassRule decides whether a just-created task may be cleared without its
// assignee acting, naming the audit reason for the decision.
type autoPassRule func(assigneeID string) (reason string, ok bool)

// entryAutoPassRule combines the node's entry-time auto-pass sources — the
// same-applicant policy and the consecutive-approver policy — into one rule.
// For a task both sources would clear, the same-applicant reason wins as the
// more specific fact. Returns nil when no source applies, so the caller can
// skip the task sweep entirely.
func (*ApprovalProcessor) entryAutoPassRule(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) (autoPassRule, error) {
	var rules []autoPassRule

	if pc.Node.SameApplicantAction == approval.SameApplicantAutoPass && containsApplicant(assignees, pc.Instance.ApplicantID) {
		rules = append(rules, func(assigneeID string) (string, bool) {
			return AutoPassReasonSameApplicant, assigneeID == pc.Instance.ApplicantID
		})
	}

	if pc.Node.ConsecutiveApproverAction == approval.ConsecutiveApproverAutoPass {
		prevApprovers, err := findPreviousApprovalApprovers(ctx, pc.DB, pc.Instance, pc.Node.ID)
		if err != nil {
			return nil, err
		}

		if prevApprovers.Size() > 0 {
			rules = append(rules, func(assigneeID string) (string, bool) {
				return autoPassReasonConsecutiveApprover, prevApprovers.Contains(assigneeID)
			})
		}
	}

	if len(rules) == 0 {
		return nil, nil
	}

	return func(assigneeID string) (string, bool) {
		for _, rule := range rules {
			if reason, ok := rule(assigneeID); ok {
				return reason, true
			}
		}

		return "", false
	}, nil
}

// autoPassEligibleTasks marks the just-created tasks the rule clears as
// approved without their assignee acting. creationEvents are the events of the
// tasks this node just inserted; they lead the returned slice so subscribers
// observe the natural lifecycle order, and they take part in the activation
// retraction below.
func (*ApprovalProcessor) autoPassEligibleTasks(ctx context.Context, pc *ProcessContext, creationEvents []approval.DomainEvent, rule autoPassRule) (*ProcessResult, error) {
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
		return nil, fmt.Errorf("query tasks for entry auto-pass sweep: %w", err)
	}

	now := timex.Now()
	autoPassedAny := false

	var events []approval.DomainEvent

	for i := range tasks {
		task := &tasks[i]

		reason, ok := rule(task.AssigneeID)
		if !ok {
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
			return nil, fmt.Errorf("auto-pass task: %w", err)
		}

		autoPassedAny = true

		// The pass happened without the approver acting, so it must leave the
		// same audit trail a manual approval would: a task-approved event
		// (system-operated) and an action log entry.
		events = append(events, approval.NewTaskApprovedEvent(
			pc.Instance, task, pc.Node,
			shared.SystemOperator, reason,
		))
		recordSystemActionLog(ctx, pc, task, reason)

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
					} else {
						// The cascade may auto-pass this task on the next loop pass,
						// which then emits its own approved event — announcing the
						// activation first keeps the observable order truthful.
						events = append(events, approval.NewTaskActivatedEvent(
							pc.Instance, &tasks[j], pc.Node, approval.TaskActivationQueueAdvanced,
						))
					}

					break
				}
			}
		}
	}

	events = append(slices.Clone(creationEvents), events...)

	if !autoPassedAny {
		return &ProcessResult{Action: NodeActionWait, Events: events}, nil
	}

	// The clears may already satisfy the node's pass rule (an any rule needs
	// one approval; a full clear satisfies every rule), so evaluate it the
	// same way a manual action would and conclude the node at entry when it
	// does — leaving the node open would demand decisions the rule no longer
	// needs.
	//
	// Entry-time auto-pass paths (this sweep, plus same-applicant sole-seat /
	// empty-assignee / execution) intentionally do NOT fire timing-based node
	// CC (CCTimingOnApprove): they return NodeActionContinue so the engine
	// advances directly, bypassing service.HandleNodeCompletion where
	// TriggerNodeCC(PassRulePassed) lives. The engine cannot call into the
	// service layer (service imports engine, not vice versa), and a node
	// cleared without any human approval has no approver action to notify CC
	// about. This suppression is uniform across all entry-time auto-pass paths
	// and is pinned by TestConsecutiveApproverAutoPass.
	completion, err := evaluatePassRule(pc.Registry, pc.Node, tasks)
	if err != nil {
		return nil, err
	}

	if completion == approval.PassRulePassed {
		cancelEvents, err := cancelRemainingEntryTasks(ctx, pc, tasks, now)
		if err != nil {
			return nil, err
		}

		events = append(events, cancelEvents...)
	}

	// A task can be created Pending, or promoted by the cascade, and then be
	// cleared by the same pass — or canceled because the pass already decided
	// the node; its assignee never had to act, so the activation must not
	// reach them as a notification.
	events = suppressActivationsForClearedTasks(events, tasks)

	if completion == approval.PassRulePassed {
		return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
	}

	return &ProcessResult{Action: NodeActionWait, Events: events}, nil
}

// cancelRemainingEntryTasks cancels the still-actionable tasks left after an
// entry-time auto-pass already satisfied the node's pass rule, mirroring the
// completion path in service.HandleNodeCompletion. The rows are already locked
// by the caller's FOR UPDATE read, so a plain update suffices; the in-memory
// tasks are mutated alongside so the caller's activation retraction sees the
// final states.
func cancelRemainingEntryTasks(ctx context.Context, pc *ProcessContext, tasks []approval.Task, now timex.DateTime) ([]approval.DomainEvent, error) {
	remaining := make([]*approval.Task, 0, len(tasks))

	for i := range tasks {
		if tasks[i].Status == approval.TaskPending || tasks[i].Status == approval.TaskWaiting {
			remaining = append(remaining, &tasks[i])
		}
	}

	if len(remaining) == 0 {
		return nil, nil
	}

	ids := make([]string, len(remaining))
	for i, task := range remaining {
		ids[i] = task.ID
	}

	if _, err := pc.DB.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Set("finished_at", now).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("id", ids)
		}).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("cancel remaining entry tasks: %w", err)
	}

	events := make([]approval.DomainEvent, len(remaining))

	for i, task := range remaining {
		task.Status = approval.TaskCanceled
		task.FinishedAt = new(now)
		events[i] = approval.NewTaskCanceledEvent(pc.Instance, task, pc.Node, cancelReasonEntryNodePassed)
	}

	return events, nil
}

// containsApplicant reports whether the applicant holds a seat in the resolved
// assignee set, judged by the resolved actor.
func containsApplicant(assignees []approval.ResolvedAssignee, applicantID string) bool {
	return slices.ContainsFunc(assignees, func(a approval.ResolvedAssignee) bool {
		return a.User.ID == applicantID
	})
}
