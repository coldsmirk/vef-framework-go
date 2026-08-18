package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// TaskContext holds the validated context for task processing operations.
type TaskContext struct {
	Instance *approval.Instance
	Task     *approval.Task
	Node     *approval.FlowNode
	// FormFields is the flow version's parsed form fields. PrepareOperation
	// loads them (only when the node grants field permissions) so submitted-value
	// validation and the approve/handle required-permission check share one
	// query; nil for permission-less nodes and for contexts loaded directly via
	// LoadTaskContextForNodeOperation.
	FormFields []approval.FormFieldDefinition
}

// TaskContextLoadOptions controls validations when loading task operation context.
type TaskContextLoadOptions struct {
	OperatorID              string
	RequireOperatorAssignee bool
	RequireTaskPending      bool
	RequireCurrentNode      bool
	// Caller asserts the caller's tenant authority. The loader rejects
	// cross-tenant access (mapped to ErrTaskNotFound so callers cannot
	// probe existence across tenants); only super-admin / system-internal
	// callers bypass, and a zero value is denied fail-closed — see
	// approval.CallerContext for the trust model.
	Caller approval.CallerContext
}

// cancelableTaskStatuses lists the still-actionable statuses (Pending /
// Waiting): the set eligible for cancellation, reused wherever a query means
// "tasks still awaiting action" (dependency activation, peer authorization).
var cancelableTaskStatuses = []string{string(approval.TaskPending), string(approval.TaskWaiting)}

// TaskService provides task-level domain operations.
type TaskService struct {
	formDataMaxBytes int
}

// NewTaskService creates a new TaskService.
func NewTaskService(opts ...Option) *TaskService {
	return &TaskService{formDataMaxBytes: resolveOptions(opts).formDataMaxBytes}
}

// FinishTask transitions a task to the given status and sets its FinishedAt timestamp.
func (*TaskService) FinishTask(ctx context.Context, db orm.DB, task *approval.Task, status approval.TaskStatus) error {
	originalStatus := task.Status
	if !engine.TaskStateMachine.CanTransition(originalStatus, status) {
		return shared.ErrInvalidTaskTransition
	}

	finishedAt := timex.Now()

	result, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", status).
		Set("finished_at", finishedAt).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(task.ID).
				Equals("status", originalStatus)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for task update: %w", err)
	}

	if affected == 0 {
		if originalStatus == approval.TaskPending {
			return shared.ErrTaskNotPending
		}

		return shared.ErrInvalidTaskTransition
	}

	task.Status = status
	task.FinishedAt = &finishedAt

	return nil
}

// PersistInstanceFormData writes back only the instance's form_data column.
// Task-action handlers (approve / reject / transfer) mutate form_data locally
// via MergeFormData while the state machine already persists status /
// current_node_id / finished_at, so this is the single column those handlers
// still need to flush — shared here so the verbatim UPDATE is not copy-pasted.
func (*TaskService) PersistInstanceFormData(ctx context.Context, db orm.DB, instance *approval.Instance) error {
	if _, err := db.NewUpdate().
		Model(instance).
		Select("form_data").
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("update instance form_data: %w", err)
	}

	return nil
}

// ActivateNextSequentialTask activates the next waiting task in a node's
// sort-ordered queue. It is a no-op while any task on the node is still
// Pending, which makes it idempotent and safe to call after any task finishes
// — or after a queued (Waiting) task is removed — without ever leaving two
// tasks active at once.
//
// When the queue reaches a seat the node's same-applicant auto-pass policy
// clears — the applicant's own, resolved from the node definition — the task
// is approved on the spot without announcing an activation (its assignee never
// has to act), and the queue advances past it. The returned slice then carries
// the system decision events ahead of the final activation; callers that
// evaluate node completion afterwards split them off first (see
// SplitQueueAdvanceDecisions) so decisions reach subscribers before the
// completion they may cause.
func (s *TaskService) ActivateNextSequentialTask(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) ([]approval.DomainEvent, error) {
	pendingExists, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				Equals("node_id", node.ID).
				Equals("status", approval.TaskPending)
		}).
		Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check pending task before sequential activation: %w", err)
	}

	if pendingExists {
		return nil, nil
	}

	var events []approval.DomainEvent

	for {
		var nextTask approval.Task

		err = db.NewSelect().
			Model(&nextTask).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instance.ID).
					Equals("node_id", node.ID).
					Equals("status", approval.TaskWaiting)
			}).
			OrderBy("sort_order").
			Limit(1).
			Scan(ctx)
		if err != nil {
			if result.IsRecordNotFound(err) {
				return events, nil
			}

			return nil, fmt.Errorf("find next sequential task: %w", err)
		}

		if !engine.TaskStateMachine.CanTransition(nextTask.Status, approval.TaskPending) {
			return events, nil
		}

		nextTask.Deadline = computeTaskDeadline(node)

		res, err := db.NewUpdate().
			Model((*approval.Task)(nil)).
			Set("status", approval.TaskPending).
			Set("deadline", nextTask.Deadline).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(nextTask.ID).
					Equals("status", approval.TaskWaiting)
			}).
			Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("activate next sequential task: %w", err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("get affected rows for sequential activation: %w", err)
		}

		// A concurrent writer may have advanced the row first; only the winner
		// announces the activation, so the assignee is notified exactly once —
		// and only the winner may continue the cascade.
		if affected == 0 {
			return events, nil
		}

		nextTask.Status = approval.TaskPending

		if shouldAutoPassSameApplicant(instance, node, &nextTask) {
			if err := s.FinishTask(ctx, db, &nextTask, approval.TaskApproved); err != nil {
				return nil, fmt.Errorf("auto-pass same-applicant task: %w", err)
			}

			// The pass happened without the approver acting: same audit trail
			// as a manual approval — a system-operated task-approved event,
			// plus an action log entry when the request-scoped collector is
			// present (outside the CQRS pipeline the event remains the record).
			events = append(events, approval.NewTaskApprovedEvent(
				instance, &nextTask, node,
				shared.SystemOperator, engine.AutoPassReasonSameApplicant,
			))
			appendSystemTaskActionLog(ctx, &nextTask, engine.AutoPassReasonSameApplicant)

			continue
		}

		events = append(events, approval.NewTaskActivatedEvent(instance, &nextTask, node, approval.TaskActivationQueueAdvanced))

		return events, nil
	}
}

// shouldAutoPassSameApplicant reports whether a just-promoted sequential task
// is the applicant's own seat under an auto-pass same-applicant policy. Tasks
// a human explicitly created (add-assignee splices carry AddAssigneeType;
// transfer and reassign replace tasks outside this path) stay manual — an
// explicit decision to involve the applicant overrides the node policy.
func shouldAutoPassSameApplicant(instance *approval.Instance, node *approval.FlowNode, task *approval.Task) bool {
	return node.Kind == approval.NodeApproval &&
		node.SameApplicantAction == approval.SameApplicantAutoPass &&
		task.AssigneeID == instance.ApplicantID &&
		task.AddAssigneeType == nil
}

// appendSystemTaskActionLog records a system-operated action log for a
// task-scoped engine decision when the request-scoped collector is available,
// mirroring the engine's recordSystemActionLog.
func appendSystemTaskActionLog(ctx context.Context, task *approval.Task, reason string) {
	collector, ok := behavior.TryActionLogCollectorFromContext(ctx)
	if !ok {
		return
	}

	entry := shared.SystemOperator.NewActionLog(task.InstanceID, approval.ActionExecute)
	entry.NodeID = new(task.NodeID)
	entry.TaskID = new(task.ID)
	entry.Opinion = new(reason)

	collector.Add(entry)
}

// SplitQueueAdvanceDecisions separates the system decision events a queue
// advance produced (same-applicant auto-passes) from the provisional
// activation events. Decisions occurred before any node evaluation that
// follows, so callers add them to their event flow ahead of
// HandleNodeCompletion; activations stay provisional until reconciled against
// the completion's cancellations (SuppressSupersededActivations).
func SplitQueueAdvanceDecisions(events []approval.DomainEvent) (decisions, activations []approval.DomainEvent) {
	for _, evt := range events {
		if _, ok := evt.(*approval.TaskActivatedEvent); ok {
			activations = append(activations, evt)
		} else {
			decisions = append(decisions, evt)
		}
	}

	return decisions, activations
}

// SuppressSupersededActivations drops activation events whose task was canceled
// by node completion in the same transaction, returning the activations that
// still stand.
//
// Dependent tasks must be unblocked BEFORE the node is evaluated — otherwise a
// suspended parent or queued child leaves the node short of a decision — so an
// activation is provisional until the evaluation lands. When the same action
// also satisfies the pass rule (a sequential queue under an any/ratio rule is
// the common case), completion cancels the task that was just promoted, and
// announcing it would tell someone to act on work that no longer exists.
//
// Both event sets come from the same transaction, so the cancellations already
// name every superseded task; no reload is needed.
func SuppressSupersededActivations(activations, completions []approval.DomainEvent) []approval.DomainEvent {
	if len(activations) == 0 || len(completions) == 0 {
		return activations
	}

	canceled := collections.NewHashSet[string]()

	for _, evt := range completions {
		if c, ok := evt.(*approval.TaskCanceledEvent); ok {
			canceled.Add(c.TaskID)
		}
	}

	if canceled.IsEmpty() {
		return activations
	}

	kept := make([]approval.DomainEvent, 0, len(activations))

	for _, evt := range activations {
		if a, ok := evt.(*approval.TaskActivatedEvent); ok && canceled.Contains(a.TaskID) {
			continue
		}

		kept = append(kept, evt)
	}

	return kept
}

// ActivateDependentTasks activates whatever the completion of finishedTask
// unblocks on its node, so the node keeps making progress.
//
// Sequential nodes advance their single sort-ordered queue; add-assignee
// splices its tasks into that queue at the anchor's position, so they are
// picked up like any other — no special handling is needed.
//
// Parallel nodes have no implicit queue, so a task suspended or queued by
// add-assignee would otherwise never become actionable — that was the deadlock
// where a "before" add-assignee permanently stranded the original assignee on
// an all/ratio node. Their dependencies are resolved explicitly via the
// parent/child link instead:
//   - a "before" parent, suspended to Waiting while its pre-approvers act, is
//     reactivated once all of its before-children finish;
//   - "after" children, queued as Waiting, are activated once the parent they
//     were attached to finishes.
//
// Transfer and rollback intentionally do not call this: a transfer replaces a
// task in place (the work is not done), and a rollback abandons the node.
func (s *TaskService) ActivateDependentTasks(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, finishedTask *approval.Task) ([]approval.DomainEvent, error) {
	if node.ApprovalMethod == approval.ApprovalSequential {
		return s.ActivateNextSequentialTask(ctx, db, instance, node)
	}

	return s.activateParallelDependents(ctx, db, instance, node, finishedTask)
}

// activateParallelDependents resolves add-assignee task dependencies on a
// parallel node via the parent/child link, since there is no sort-ordered
// queue to do it implicitly.
func (s *TaskService) activateParallelDependents(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, finishedTask *approval.Task) ([]approval.DomainEvent, error) {
	var events []approval.DomainEvent

	// A finished "before" child may unblock its suspended parent.
	if finishedTask.ParentTaskID != nil &&
		finishedTask.AddAssigneeType != nil &&
		*finishedTask.AddAssigneeType == approval.AddAssigneeBefore {
		parentEvents, err := s.reactivateBeforeParent(ctx, db, instance, node, *finishedTask.ParentTaskID)
		if err != nil {
			return nil, err
		}

		events = append(events, parentEvents...)
	}

	// The finished task may itself be the parent that "after" children wait on.
	childEvents, err := s.activateAfterChildren(ctx, db, instance, node, finishedTask.ID)
	if err != nil {
		return nil, err
	}

	return append(events, childEvents...), nil
}

// reactivateBeforeParent returns a "before"-suspended parent task to Pending
// once all of its before-children have finished. The optimistic
// WHERE status = waiting makes it a no-op if the parent was already
// reactivated, was canceled, or is not actually suspended.
func (*TaskService) reactivateBeforeParent(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, parentTaskID string) ([]approval.DomainEvent, error) {
	activeBeforeChildren, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("parent_task_id", parentTaskID).
				Equals("add_assignee_type", string(approval.AddAssigneeBefore)).
				In("status", cancelableTaskStatuses)
		}).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count active before-children: %w", err)
	}

	if activeBeforeChildren > 0 {
		return nil, nil
	}

	res, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskPending).
		Set("deadline", computeTaskDeadline(node)).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(parentTaskID).
				Equals("status", string(approval.TaskWaiting))
		}).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("reactivate before-parent task: %w", err)
	}

	// The optimistic WHERE makes this a no-op when the parent was already
	// reactivated, canceled, or never suspended — no activation to announce.
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get affected rows for before-parent reactivation: %w", err)
	}

	if affected == 0 {
		return nil, nil
	}

	// Reload rather than reuse the caller's copy: the event has to carry the
	// assignee this task is now waiting on.
	var parent approval.Task

	parent.ID = parentTaskID

	if err := db.NewSelect().Model(&parent).WherePK().Scan(ctx); err != nil {
		return nil, fmt.Errorf("load reactivated before-parent task: %w", err)
	}

	return []approval.DomainEvent{
		approval.NewTaskActivatedEvent(instance, &parent, node, approval.TaskActivationQueueAdvanced),
	}, nil
}

// activateAfterChildren promotes the Waiting "after" children of a just-
// finished parent task to Pending so they take their turn.
// Each child is promoted individually rather than in one sweeping UPDATE: the
// activation event has to name the assignee it woke, and only a per-row
// compare-and-set can tell which rows this call actually promoted versus which
// a concurrent writer had already taken.
func (*TaskService) activateAfterChildren(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, parentTaskID string) ([]approval.DomainEvent, error) {
	var children []approval.Task

	if err := db.NewSelect().
		Model(&children).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("parent_task_id", parentTaskID).
				Equals("add_assignee_type", string(approval.AddAssigneeAfter)).
				Equals("status", string(approval.TaskWaiting))
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("find after-children to activate: %w", err)
	}

	if len(children) == 0 {
		return nil, nil
	}

	deadline := computeTaskDeadline(node)
	events := make([]approval.DomainEvent, 0, len(children))

	for i := range children {
		child := &children[i]

		res, err := db.NewUpdate().
			Model((*approval.Task)(nil)).
			Set("status", approval.TaskPending).
			Set("deadline", deadline).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(child.ID).
					Equals("status", string(approval.TaskWaiting))
			}).
			Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("activate after-child: %w", err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("get affected rows for after-child activation: %w", err)
		}

		if affected == 0 {
			continue
		}

		child.Status = approval.TaskPending
		child.Deadline = deadline

		events = append(events, approval.NewTaskActivatedEvent(
			instance, child, node, approval.TaskActivationQueueAdvanced,
		))
	}

	return events, nil
}

// RepointAddAssigneeChildren re-parents the still-active add-assignee children
// of a replaced task onto its stand-in. A transfer finishes the original task
// and inserts a replacement with a new ID; the parallel-node dependency
// resolution keys off parent_task_id, so without re-pointing, the "after"
// children queued against the original would never be activated by the
// replacement's completion — orphaning them and re-creating the very deadlock
// the parent/child activation was built to avoid. Scoped to the instance for
// defense-in-depth on top of the globally-unique parent id.
func (*TaskService) RepointAddAssigneeChildren(ctx context.Context, db orm.DB, fromParentID, toParentID, instanceID string) error {
	if _, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("parent_task_id", toParentID).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("parent_task_id", fromParentID).
				In("status", cancelableTaskStatuses)
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("repoint add-assignee children: %w", err)
	}

	return nil
}

// computeTaskDeadline calculates task deadline from node timeout configuration.
// Returns nil when timeout is disabled.
func computeTaskDeadline(node *approval.FlowNode) *timex.DateTime {
	if node == nil {
		return nil
	}

	return shared.ComputeTaskDeadline(node.TimeoutHours)
}

// CancelRemainingTasks cancels all pending/waiting tasks on the given node
// and returns one TaskCanceledEvent per canceled task, so assignees whose
// decision is no longer needed can have their pending to-dos retracted.
// Callers attach the events to their own event flow (the caller holds the
// instance row lock, which serializes this two-step read-then-update against
// every other task mutation path).
func (s *TaskService) CancelRemainingTasks(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, reason string) ([]approval.DomainEvent, error) {
	return s.cancelActiveTasks(ctx, db, instance, reason, func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instance.ID).
			Equals("node_id", node.ID).
			In("status", cancelableTaskStatuses)
	})
}

// CancelInstanceTasks cancels all pending/waiting tasks for an entire
// instance, returning the corresponding TaskCanceledEvents.
func (s *TaskService) CancelInstanceTasks(ctx context.Context, db orm.DB, instance *approval.Instance, reason string) ([]approval.DomainEvent, error) {
	return s.cancelActiveTasks(ctx, db, instance, reason, func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instance.ID).
			In("status", cancelableTaskStatuses)
	})
}

// cancelActiveTasks loads the tasks matching filter, marks them canceled, and
// returns their cancellation events in load order. Canceled tasks may span
// several nodes (instance-wide cancellation), so node names for the events
// are batch-loaded by ID.
func (*TaskService) cancelActiveTasks(ctx context.Context, db orm.DB, instance *approval.Instance, reason string, filter func(orm.ConditionBuilder)) ([]approval.DomainEvent, error) {
	var tasks []approval.Task

	if err := db.NewSelect().
		Model(&tasks).
		Select("id", "tenant_id", "instance_id", "node_id",
			"assignee_id", "assignee_name", "assignee_department_id", "assignee_department_name").
		Where(filter).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load tasks to cancel: %w", err)
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	taskIDs := make([]string, len(tasks))
	for i := range tasks {
		taskIDs[i] = tasks[i].ID
	}

	if _, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Set("finished_at", timex.Now()).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("id", taskIDs).
				In("status", cancelableTaskStatuses)
		}).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("cancel tasks: %w", err)
	}

	nodeByID, err := loadNodesByID(ctx, db, tasks)
	if err != nil {
		return nil, err
	}

	events := make([]approval.DomainEvent, len(tasks))

	for i := range tasks {
		task := &tasks[i]

		node := nodeByID[task.NodeID]
		if node == nil {
			// Defensive: the FK guarantees the node row exists; an absent
			// entry would mean the load above raced a deletion. Emit the
			// event with the ID-only coordinates rather than dropping it.
			node = &approval.FlowNode{}
			node.ID = task.NodeID
		}

		events[i] = approval.NewTaskCanceledEvent(instance, task, node, reason)
	}

	return events, nil
}

// loadNodesByID batch-loads the flow nodes referenced by the given tasks,
// keyed by node ID.
func loadNodesByID(ctx context.Context, db orm.DB, tasks []approval.Task) (map[string]*approval.FlowNode, error) {
	nodeIDs := collections.NewHashSet[string]()
	for i := range tasks {
		nodeIDs.Add(tasks[i].NodeID)
	}

	var nodes []approval.FlowNode

	if err := db.NewSelect().
		Model(&nodes).
		Select("id", "name").
		Where(func(cb orm.ConditionBuilder) {
			cb.In("id", nodeIDs.ToSlice())
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load nodes for canceled tasks: %w", err)
	}

	nodeByID := make(map[string]*approval.FlowNode, len(nodes))
	for i := range nodes {
		nodeByID[nodes[i].ID] = &nodes[i]
	}

	return nodeByID, nil
}

// IsAuthorizedForNodeOperation reports whether the operator may perform
// node-level operations (e.g. remove assignee): true if the operator is a
// peer assignee on the same node or a flow admin. Database errors are
// returned to the caller rather than swallowed, so an infrastructure
// failure surfaces as a server error instead of a silent authorization
// denial.
func (*TaskService) IsAuthorizedForNodeOperation(ctx context.Context, db orm.DB, instanceID, nodeID, operatorID string) (bool, error) {
	peerCount, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				Equals("assignee_id", operatorID).
				In("status", cancelableTaskStatuses)
		}).
		Count(ctx)
	if err != nil {
		return false, err
	}

	if peerCount > 0 {
		return true, nil
	}

	var instance approval.Instance

	instance.ID = instanceID

	// A missing instance/flow means no flow-admin grant exists — a
	// legitimate "not authorized" answer — so it returns (false, nil).
	// Any other error is an infrastructure failure and is surfaced.
	if err := db.NewSelect().
		Model(&instance).
		Select("flow_id").
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return false, nil
		}

		return false, err
	}

	var flow approval.Flow

	flow.ID = instance.FlowID

	if err := db.NewSelect().
		Model(&flow).
		Select("admin_user_ids").
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return slices.Contains(flow.AdminUserIDs, operatorID), nil
}

// isDecisionParticipant reports whether userID carries responsibility for the
// instance's decision: the applicant, or anyone a task on the instance was ever
// opened on — held directly (assignee_id) or handed to a delegate while the
// slot stayed theirs (delegator_id). Observers are excluded; CC recipients are
// deliberately not part of this set.
//
// Both handoff shapes must land here. A transfer opens a new task on the
// transferee and leaves the original row's assignee_id intact, so the person
// who handed the work on still matches; a delegation instead moves them to
// delegator_id, so matching assignee_id alone would deny the same
// organizational situation purely because of how it is stored.
//
// DB errors are propagated; a not-found instance maps to
// shared.ErrInstanceNotFound.
func isDecisionParticipant(ctx context.Context, db orm.DB, instanceID, userID string) (bool, error) {
	var instance approval.Instance

	instance.ID = instanceID

	if err := db.NewSelect().
		Model(&instance).
		Select("applicant_id").
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return false, shared.ErrInstanceNotFound
		}

		return false, fmt.Errorf("load instance: %w", err)
	}

	if instance.ApplicantID == userID {
		return true, nil
	}

	hasTask, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Group(func(cb orm.ConditionBuilder) {
					cb.Equals("assignee_id", userID).
						OrEquals("delegator_id", userID)
				})
		}).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check task participation: %w", err)
	}

	return hasTask, nil
}

// IsUrgeAuthorized reports whether userID may dispatch an urge for tasks
// belonging to instanceID. Narrower than IsInstanceParticipant by exactly one
// leg: CC recipients are excluded, because they are not on the hook for the
// decision and the right to urge has been abused by random observers in prior
// incidents.
//
// The right is instance-scoped, not task-scoped: a participant may urge any
// pending task on the instance, not only the one they hold. An approver two
// nodes back is as entitled to ask why the instance is stuck as the applicant
// is, and the per-(task, urger) cooldown is what bounds the volume.
func (*TaskService) IsUrgeAuthorized(ctx context.Context, db orm.DB, instanceID, userID string) (bool, error) {
	return isDecisionParticipant(ctx, db, instanceID, userID)
}

// IsInstanceParticipant checks whether the user is related to the instance as
// applicant, task assignee, delegator, or CC recipient.
//
// The CC leg is what separates this from IsUrgeAuthorized: an observer may read
// the instance but not nag the people deciding it.
//
// A delegator reaches the detail read-only. The delegate holds the pending
// task, so no action is offered (computeActions keys the actionable set off
// assignee_id) and the field-permission projection clamps this context to
// visible.
//
// Whatever is added here MUST also be recognized by
// query.resolveViewerFieldPermissions, whose contexts have to stay a superset
// of this participant set — otherwise the new viewer reaches the detail and
// finds every form field stripped.
func (*TaskService) IsInstanceParticipant(ctx context.Context, db orm.DB, instanceID, userID string) (bool, error) {
	ok, err := isDecisionParticipant(ctx, db, instanceID, userID)
	if err != nil || ok {
		return ok, err
	}

	hasCC, err := db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("cc_user_id", userID)
		}).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check cc participation: %w", err)
	}

	return hasCC, nil
}

// CanRemoveAssigneeTask determines whether removing a task can still drive the
// node to progress (either through remaining actionable tasks or immediate
// completion under pass-rule evaluation).
func (*TaskService) CanRemoveAssigneeTask(ctx context.Context, db orm.DB, eng *engine.FlowEngine, node *approval.FlowNode, task approval.Task) (bool, error) {
	var tasks []approval.Task

	// Scoped to the candidate's visit so the simulation evaluates the same
	// task set the engine's completion evaluation will — tasks left behind by
	// an earlier traversal are excluded.
	if err := db.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("visit_id", task.VisitID)
		}).
		ForUpdate().
		Scan(ctx); err != nil {
		return false, fmt.Errorf("query node tasks: %w", err)
	}

	hasOtherActionable := false
	for i := range tasks {
		if tasks[i].ID == task.ID {
			tasks[i].Status = approval.TaskRemoved
		} else if tasks[i].Status == approval.TaskPending || tasks[i].Status == approval.TaskWaiting {
			hasOtherActionable = true
		}
	}

	if hasOtherActionable {
		return true, nil
	}

	evalResult, err := eng.EvaluatePassRuleWithTasks(node, tasks)
	if err != nil {
		return false, err
	}

	return evalResult != approval.PassRulePending, nil
}

// PrepareOperation loads task context and merges editable form data.
// Callers that require opinion validation should invoke ValidateOpinion separately.
func (s *TaskService) PrepareOperation(ctx context.Context, db orm.DB, taskID string, operator approval.UserInfo, caller approval.CallerContext, formData map[string]any) (*TaskContext, error) {
	tc, err := s.LoadTaskContextForNodeOperation(ctx, db, taskID, TaskContextLoadOptions{
		OperatorID:              operator.ID,
		RequireOperatorAssignee: true,
		RequireTaskPending:      true,
		RequireCurrentNode:      true,
		Caller:                  caller,
	})
	if err != nil {
		return nil, err
	}

	// A permission-less node exposes nothing for editing and demands no required
	// value, so both the editable-subset validation and the approve-side required
	// check are no-ops — skip the form_fields load entirely (tc.FormFields stays nil).
	if len(tc.Node.FieldPermissions) > 0 {
		// Load the version's parsed form fields so submitted edits validate against
		// them, and expose them on the context so the approve/handle
		// required-permission check reuses the same field list without a second query.
		var version approval.FlowVersion

		version.ID = tc.Instance.FlowVersionID
		if err := db.NewSelect().
			Model(&version).
			Select("form_fields").
			WherePK().
			Scan(ctx); err != nil {
			return nil, fmt.Errorf("load flow version form fields: %w", err)
		}

		tc.FormFields = version.FormFields
	}

	// Validate the submitted editable subset against the schema before merging,
	// so a malformed approver edit is rejected instead of persisted. Emptiness is
	// deferred to the approve/handle required-permission check, not treated as a
	// value error here.
	if err := validateEditableFormData(tc.FormFields, formData, tc.Node.FieldPermissions); err != nil {
		return nil, err
	}

	// Capture the pre-merge size so the cap is enforced on the growth this
	// action introduces, not on the standing payload. An instance whose stored
	// form data already exceeds the cap (created before the cap existed, or
	// after lowering vef.approval.form_data_max_bytes) must stay actionable —
	// approvers can still push it forward and a rollback-clear can shrink it.
	beforeSize, err := encodedFormDataSize(tc.Instance.FormData)
	if err != nil {
		return nil, err
	}

	MergeFormData(tc.Instance, formData, tc.Node.FieldPermissions)

	afterSize, err := encodedFormDataSize(tc.Instance.FormData)
	if err != nil {
		return nil, err
	}

	// Reject only when this action grows the encoded form past the cap — the
	// drip-feed-growth vector, since FilterEditableFormData bounds the keys but
	// not the encoded size. ValidateFormData still enforces the absolute cap at
	// start / resubmit, where the applicant owns the whole payload.
	if afterSize > s.formDataMaxBytes && afterSize > beforeSize {
		return nil, shared.ErrFormDataTooLarge
	}

	return tc, nil
}

// ActionLogParams holds optional fields for BuildActionLog.
type ActionLogParams struct {
	Opinion string
	// TransferTo names the recipient of a transfer-style action, snapshotting
	// identity and department at action time.
	TransferTo       *approval.UserInfo
	RollbackToNodeID string
	// Attachments holds storage file references the operator attached to this
	// action (e.g. a signed document uploaded with an approval opinion). The
	// approval module persists them verbatim; upload/resolution is the caller's
	// concern (the storage module).
	Attachments []string
}

// BuildActionLog constructs a task-scoped ActionLog entry. Persistence is
// deferred to ActionLogBehavior; callers hand the result to the request-
// scoped ActionLogCollector instead of inserting directly so transactional
// log writes happen at one place.
func (*TaskService) BuildActionLog(
	instanceID string,
	task *approval.Task,
	operator approval.UserInfo,
	action approval.ActionType,
	params ActionLogParams,
) *approval.ActionLog {
	actionLog := operator.NewActionLog(instanceID, action)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)

	if params.Opinion != "" {
		actionLog.Opinion = new(params.Opinion)
	}

	if params.TransferTo != nil {
		actionLog.TransferToID = new(params.TransferTo.ID)
		actionLog.TransferToName = new(params.TransferTo.Name)
		actionLog.TransferToDepartmentID = params.TransferTo.DepartmentID
		actionLog.TransferToDepartmentName = params.TransferTo.DepartmentName
	}

	if params.RollbackToNodeID != "" {
		actionLog.RollbackToNodeID = new(params.RollbackToNodeID)
	}

	if len(params.Attachments) > 0 {
		actionLog.Attachments = params.Attachments
	}

	return actionLog
}

// LoadTaskContextForNodeOperation loads and validates instance/task/node context for node operations.
// The lock order is always instance first, then task.
func (s *TaskService) LoadTaskContextForNodeOperation(ctx context.Context, db orm.DB, taskID string, options TaskContextLoadOptions) (*TaskContext, error) {
	return s.loadContext(ctx, db, taskID, options)
}

// loadContext loads and validates the instance, task, and node for task processing.
func (*TaskService) loadContext(ctx context.Context, db orm.DB, taskID string, options TaskContextLoadOptions) (*TaskContext, error) {
	var task approval.Task

	task.ID = taskID

	if err := db.NewSelect().
		Model(&task).
		Select("instance_id").
		WherePK().
		Scan(ctx); err != nil {
		return nil, shared.ErrTaskNotFound
	}

	var instance approval.Instance

	instance.ID = task.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		WherePK().
		ForUpdate().
		Scan(ctx); err != nil {
		return nil, shared.ErrInstanceNotFound
	}

	// Tenant guard: cross-tenant callers see a uniform "task not found" so
	// they can't probe entity existence across tenants. System / super-admin
	// callers fall through (see approval.CallerContext.Allows).
	if !options.Caller.Allows(instance.TenantID) {
		return nil, shared.ErrTaskNotFound
	}

	// Lock task after instance to keep a consistent lock order across command handlers.
	if err := db.NewSelect().
		Model(&task).
		WherePK().
		ForUpdate().
		Scan(ctx); err != nil {
		return nil, shared.ErrTaskNotFound
	}

	if instance.Status != approval.InstanceRunning {
		return nil, shared.ErrInstanceCompleted
	}

	if options.RequireOperatorAssignee && task.AssigneeID != options.OperatorID {
		return nil, shared.ErrNotAssignee
	}

	if options.RequireTaskPending && task.Status != approval.TaskPending {
		return nil, shared.ErrTaskNotPending
	}

	if options.RequireCurrentNode && (instance.CurrentNodeID == nil || *instance.CurrentNodeID != task.NodeID) {
		return nil, shared.ErrTaskNotPending
	}

	var node approval.FlowNode

	node.ID = task.NodeID

	if err := db.NewSelect().
		Model(&node).
		WherePK().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load node: %w", err)
	}

	return &TaskContext{Instance: &instance, Task: &task, Node: &node}, nil
}
