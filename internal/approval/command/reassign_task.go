package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ReassignTaskCmd reassigns a pending task to a different user (admin operation).
type ReassignTaskCmd struct {
	cqrs.BaseCommand

	TaskID        string
	NewAssigneeID string
	Operator      approval.OperatorInfo
	Reason        string
	Caller        approval.CallerContext
}

// ReassignTaskHandler handles the ReassignTaskCmd command.
type ReassignTaskHandler struct {
	db           orm.DB
	taskSvc      *service.TaskService
	userResolver approval.UserInfoResolver
}

// NewReassignTaskHandler creates a new ReassignTaskHandler.
func NewReassignTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	userResolver approval.UserInfoResolver,
) *ReassignTaskHandler {
	return &ReassignTaskHandler{db: db, taskSvc: taskSvc, userResolver: userResolver}
}

func (h *ReassignTaskHandler) Handle(ctx context.Context, cmd ReassignTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	// Load instance (ForUpdate) then task (ForUpdate), matching the lock order
	// used by all sibling task-mutating handlers (approve_task, transfer_task,
	// remove_assignee, etc.) via LoadTaskContextForNodeOperation. Holding the
	// instance lock serializes concurrent reassigns that target the same node,
	// so the duplicate-assignee check below runs atomically.
	tc, err := h.taskSvc.LoadTaskContextForNodeOperation(ctx, db, cmd.TaskID, service.TaskContextLoadOptions{
		RequireTaskPending: true,
		Caller:             cmd.Caller,
	})
	if err != nil {
		return cqrs.Unit{}, err
	}

	task := tc.Task

	newAssigneeID := strings.TrimSpace(cmd.NewAssigneeID)
	if newAssigneeID == "" || newAssigneeID == task.AssigneeID {
		return cqrs.Unit{}, shared.ErrInvalidTransferTarget
	}

	duplicate, err := hasActiveTaskForAssignee(ctx, db, task.InstanceID, task.NodeID, newAssigneeID)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("query reassignment target active task: %w", err)
	}

	if duplicate {
		return cqrs.Unit{}, shared.ErrInvalidTransferTarget
	}

	oldAssigneeID := task.AssigneeID
	oldAssigneeName := task.AssigneeName
	newAssigneeName := shared.ResolveUserName(ctx, h.userResolver, newAssigneeID)

	if _, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("assignee_id", newAssigneeID).
		Set("assignee_name", newAssigneeName).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(task.ID)
		}).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update task assignee: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(task.InstanceID, approval.ActionReassign)
	actionLog.TaskID = &task.ID
	actionLog.NodeID = &task.NodeID
	actionLog.TransferToID = &newAssigneeID

	actionLog.TransferToName = &newAssigneeName
	if cmd.Reason != "" {
		actionLog.Opinion = &cmd.Reason
	}

	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewTaskReassignedEvent(task.ID, task.TenantID, task.InstanceID, task.NodeID, oldAssigneeID, oldAssigneeName, newAssigneeID, newAssigneeName, cmd.Reason),
	)

	return cqrs.Unit{}, nil
}

// hasActiveTaskForAssignee reports whether the given assignee already has a
// pending or waiting task on the specified node of an instance.  Used by both
// transfer_task and reassign_task to enforce the "one active task per assignee
// per node" invariant under the instance-level lock.
func hasActiveTaskForAssignee(ctx context.Context, db orm.DB, instanceID, nodeID, assigneeID string) (bool, error) {
	count, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				Equals("assignee_id", assigneeID).
				In("status", []approval.TaskStatus{approval.TaskPending, approval.TaskWaiting})
		}).
		Count(ctx)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
