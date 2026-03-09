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
}

// cancelableTaskStatuses lists statuses eligible for bulk cancellation.
var cancelableTaskStatuses = []string{string(approval.TaskPending), string(approval.TaskWaiting)}

// TaskService provides task-level domain operations.
type TaskService struct{}

// NewTaskService creates a new TaskService.
func NewTaskService() *TaskService {
	return &TaskService{}
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

// ActivateNextSequentialTask activates the next waiting task in sequential approval.
func (*TaskService) ActivateNextSequentialTask(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) error {
	var nextTask approval.Task

	err := db.NewSelect().
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

// IsAuthorizedForNodeOperation checks if the operator is authorized to perform
// node-level operations (e.g., remove assignee). Returns true if the operator
// is a peer assignee on the same node or a flow admin.
func (*TaskService) IsAuthorizedForNodeOperation(ctx context.Context, db orm.DB, task approval.Task, operatorID string) bool {
	peerCount, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", task.InstanceID).
				Equals("node_id", task.NodeID).
				Equals("assignee_id", operatorID).
				In("status", cancelableTaskStatuses)
		}).
		Count(ctx)
	if err == nil && peerCount > 0 {
		return true
	}

	var instance approval.Instance

	instance.ID = task.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		Select("flow_id").
		WherePK().
		Scan(ctx); err != nil {
		return false
	}

	var flow approval.Flow

	flow.ID = instance.FlowID

	if err := db.NewSelect().
		Model(&flow).
		Select("admin_user_ids").
		WherePK().
		Scan(ctx); err != nil {
		return false
	}

	return slices.Contains(flow.AdminUserIDs, operatorID)
}

// IsInstanceParticipant checks whether the user is related to the instance as
// applicant, task assignee, or CC recipient.
func (*TaskService) IsInstanceParticipant(ctx context.Context, db orm.DB, instanceID, userID string) (bool, error) {
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

		return false, fmt.Errorf("load instance for participant check: %w", err)
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

	if hasTask {
		return true, nil
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
func (s *TaskService) PrepareOperation(ctx context.Context, db orm.DB, taskID, operatorID string, formData map[string]any) (*TaskContext, error) {
	tc, err := s.LoadTaskContextForNodeOperation(ctx, db, taskID, TaskContextLoadOptions{
		OperatorID:              operatorID,
		RequireOperatorAssignee: true,
		RequireTaskPending:      true,
		RequireCurrentNode:      true,
	})
	if err != nil {
		return nil, err
	}

	MergeFormData(tc.Instance, formData, tc.Node.FieldPermissions)

	return tc, nil
}

// ActionLogParams holds optional fields for InsertActionLog.
type ActionLogParams struct {
	Opinion          string
	TransferToID     string
	TransferToName   string
	RollbackToNodeID string
}

// InsertActionLog creates and inserts an action log entry.
func (*TaskService) InsertActionLog(
	ctx context.Context,
	db orm.DB,
	instanceID string,
	task *approval.Task,
	operator approval.OperatorInfo,
	action approval.ActionType,
	params ActionLogParams,
) error {
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

	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return fmt.Errorf("insert action log: %w", err)
	}

	return nil
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
