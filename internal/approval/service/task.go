package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
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
}

// TaskContextLoadOptions controls validations when loading task operation context.
type TaskContextLoadOptions struct {
	OperatorID              string
	RequireOperatorAssignee bool
	RequireTaskPending      bool
	RequireCurrentNode      bool
	// Caller asserts the caller's tenant authority. Non-zero values cause
	// the loader to reject cross-tenant access (mapped to ErrTaskNotFound
	// so callers cannot probe existence across tenants). Zero / system
	// callers bypass — see approval.CallerContext for the trust model.
	Caller approval.CallerContext
}

// cancelableTaskStatuses lists statuses eligible for bulk cancellation.
var cancelableTaskStatuses = []string{string(approval.TaskPending), string(approval.TaskWaiting)}

// TaskService provides task-level domain operations.
type TaskService struct{}

// NewTaskService creates a new TaskService.
func NewTaskService() *TaskService {
	return new(TaskService)
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

// ActivateNextSequentialTask activates the next waiting task in a node's
// sort-ordered queue. It is a no-op while any task on the node is still
// Pending, which makes it idempotent and safe to call after any task finishes
// — or after a queued (Waiting) task is removed — without ever leaving two
// tasks active at once.
func (*TaskService) ActivateNextSequentialTask(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) error {
	pendingExists, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				Equals("node_id", node.ID).
				Equals("status", approval.TaskPending)
		}).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("check pending task before sequential activation: %w", err)
	}

	if pendingExists {
		return nil
	}

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
			return nil
		}

		return fmt.Errorf("find next sequential task: %w", err)
	}

	if !engine.TaskStateMachine.CanTransition(nextTask.Status, approval.TaskPending) {
		return nil
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
		return fmt.Errorf("activate next sequential task: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows for sequential activation: %w", err)
	}

	if affected > 0 {
		nextTask.Status = approval.TaskPending
	}

	return nil
}

// ActivateDependentTasks activates whatever the completion of finishedTask
// unblocks on its node, so the node keeps making progress.
//
// Sequential nodes advance their single sort-ordered queue; an add-assignee
// task carries a sort order and is picked up by that queue like any other, so
// no special handling is needed.
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
func (s *TaskService) ActivateDependentTasks(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, finishedTask *approval.Task) error {
	if node.ApprovalMethod == approval.ApprovalSequential {
		return s.ActivateNextSequentialTask(ctx, db, instance, node)
	}

	return s.activateParallelDependents(ctx, db, node, finishedTask)
}

// activateParallelDependents resolves add-assignee task dependencies on a
// parallel node via the parent/child link, since there is no sort-ordered
// queue to do it implicitly.
func (s *TaskService) activateParallelDependents(ctx context.Context, db orm.DB, node *approval.FlowNode, finishedTask *approval.Task) error {
	// A finished "before" child may unblock its suspended parent.
	if finishedTask.ParentTaskID != nil &&
		finishedTask.AddAssigneeType != nil &&
		*finishedTask.AddAssigneeType == approval.AddAssigneeBefore {
		if err := s.reactivateBeforeParent(ctx, db, node, *finishedTask.ParentTaskID); err != nil {
			return err
		}
	}

	// The finished task may itself be the parent that "after" children wait on.
	return s.activateAfterChildren(ctx, db, node, finishedTask.ID)
}

// reactivateBeforeParent returns a "before"-suspended parent task to Pending
// once all of its before-children have finished. The optimistic
// WHERE status = waiting makes it a no-op if the parent was already
// reactivated, was canceled, or is not actually suspended.
func (*TaskService) reactivateBeforeParent(ctx context.Context, db orm.DB, node *approval.FlowNode, parentTaskID string) error {
	activeBeforeChildren, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("parent_task_id", parentTaskID).
				Equals("add_assignee_type", string(approval.AddAssigneeBefore)).
				In("status", cancelableTaskStatuses)
		}).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("count active before-children: %w", err)
	}

	if activeBeforeChildren > 0 {
		return nil
	}

	if _, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskPending).
		Set("deadline", computeTaskDeadline(node)).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(parentTaskID).
				Equals("status", string(approval.TaskWaiting))
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("reactivate before-parent task: %w", err)
	}

	return nil
}

// activateAfterChildren promotes the Waiting "after" children of a just-
// finished parent task to Pending so they take their turn.
func (*TaskService) activateAfterChildren(ctx context.Context, db orm.DB, node *approval.FlowNode, parentTaskID string) error {
	if _, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskPending).
		Set("deadline", computeTaskDeadline(node)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("parent_task_id", parentTaskID).
				Equals("add_assignee_type", string(approval.AddAssigneeAfter)).
				Equals("status", string(approval.TaskWaiting))
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("activate after-children: %w", err)
	}

	return nil
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

// CancelRemainingTasks cancels all pending/waiting tasks on the given node.
func (*TaskService) CancelRemainingTasks(ctx context.Context, db orm.DB, instanceID, nodeID string) error {
	_, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Set("finished_at", timex.Now()).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				In("status", cancelableTaskStatuses)
		}).
		Exec(ctx)

	return err
}

// CancelInstanceTasks cancels all pending/waiting tasks for an entire instance.
func (*TaskService) CancelInstanceTasks(ctx context.Context, db orm.DB, instanceID string) error {
	_, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Set("finished_at", timex.Now()).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				In("status", cancelableTaskStatuses)
		}).
		Exec(ctx)

	return err
}

// IsAuthorizedForNodeOperation reports whether the operator may perform
// node-level operations (e.g. remove assignee): true if the operator is a
// peer assignee on the same node or a flow admin. Database errors are
// returned to the caller rather than swallowed, so an infrastructure
// failure surfaces as a server error instead of a silent authorization
// denial.
func (*TaskService) IsAuthorizedForNodeOperation(ctx context.Context, db orm.DB, task approval.Task, operatorID string) (bool, error) {
	peerCount, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", task.InstanceID).
				Equals("node_id", task.NodeID).
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

	instance.ID = task.InstanceID

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

// isApplicantOrAssignee loads the instance's applicant_id, returns true if
// userID matches the applicant, or if the user has any assignee task on the
// instance. DB errors are propagated; a not-found instance maps to
// shared.ErrInstanceNotFound.
func isApplicantOrAssignee(ctx context.Context, db orm.DB, instanceID, userID string) (bool, error) {
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
				Equals("assignee_id", userID)
		}).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check task participation: %w", err)
	}

	return hasTask, nil
}

// IsUrgeAuthorized reports whether userID may dispatch an urge for tasks
// belonging to instanceID. Narrower than IsInstanceParticipant: only the
// applicant and users who have (or had) an assignee task on the instance
// count; CC recipients are excluded because they are not on the hook for
// the decision and the right to urge has been abused by random observers
// in prior incidents.
func (*TaskService) IsUrgeAuthorized(ctx context.Context, db orm.DB, instanceID, userID string) (bool, error) {
	return isApplicantOrAssignee(ctx, db, instanceID, userID)
}

// IsInstanceParticipant checks whether the user is related to the instance as
// applicant, task assignee, or CC recipient.
func (*TaskService) IsInstanceParticipant(ctx context.Context, db orm.DB, instanceID, userID string) (bool, error) {
	ok, err := isApplicantOrAssignee(ctx, db, instanceID, userID)
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

	if err := db.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", task.InstanceID).
				Equals("node_id", task.NodeID)
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
func (s *TaskService) PrepareOperation(ctx context.Context, db orm.DB, taskID string, operator approval.OperatorInfo, caller approval.CallerContext, formData map[string]any) (*TaskContext, error) {
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

	// Capture the pre-merge size so the cap is enforced on the growth this
	// action introduces, not on the standing payload. An instance whose stored
	// form data already exceeds the cap (created before the cap existed, or
	// after lowering FormDataMaxBytes at build time) must stay actionable —
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
	if afterSize > FormDataMaxBytes && afterSize > beforeSize {
		return nil, shared.ErrFormDataTooLarge
	}

	return tc, nil
}

// ActionLogParams holds optional fields for BuildActionLog.
type ActionLogParams struct {
	Opinion          string
	TransferToID     string
	TransferToName   string
	RollbackToNodeID string
}

// BuildActionLog constructs a task-scoped ActionLog entry. Persistence is
// deferred to ActionLogBehavior; callers hand the result to the request-
// scoped ActionLogCollector instead of inserting directly so transactional
// log writes happen at one place.
func (*TaskService) BuildActionLog(
	instanceID string,
	task *approval.Task,
	operator approval.OperatorInfo,
	action approval.ActionType,
	params ActionLogParams,
) *approval.ActionLog {
	actionLog := operator.NewActionLog(instanceID, action)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)

	if params.Opinion != "" {
		actionLog.Opinion = new(params.Opinion)
	}

	if params.TransferToID != "" {
		actionLog.TransferToID = new(params.TransferToID)
	}

	if params.TransferToName != "" {
		actionLog.TransferToName = new(params.TransferToName)
	}

	if params.RollbackToNodeID != "" {
		actionLog.RollbackToNodeID = new(params.RollbackToNodeID)
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
