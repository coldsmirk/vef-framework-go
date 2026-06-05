package command

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// RemoveAssigneeCmd removes an assignee by canceling their task.
type RemoveAssigneeCmd struct {
	cqrs.BaseCommand

	TaskID   string
	Operator approval.OperatorInfo
	Caller   approval.CallerContext
}

// RemoveAssigneeHandler handles the RemoveAssigneeCmd command.
type RemoveAssigneeHandler struct {
	db      orm.DB
	taskSvc *service.TaskService
	nodeSvc *service.NodeService
	engine  *engine.FlowEngine
}

// NewRemoveAssigneeHandler creates a new RemoveAssigneeHandler.
func NewRemoveAssigneeHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	eng *engine.FlowEngine,
) *RemoveAssigneeHandler {
	return &RemoveAssigneeHandler{
		db: db, taskSvc: taskSvc, nodeSvc: nodeSvc, engine: eng,
	}
}

func (h *RemoveAssigneeHandler) Handle(ctx context.Context, cmd RemoveAssigneeCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.LoadTaskContextForNodeOperation(ctx, db, cmd.TaskID, service.TaskContextLoadOptions{
		RequireCurrentNode: true,
		Caller:             cmd.Caller,
	})
	if err != nil {
		return cqrs.Unit{}, err
	}

	instance := tc.Instance
	task := tc.Task
	node := tc.Node

	if !node.IsRemoveAssigneeAllowed {
		return cqrs.Unit{}, shared.ErrRemoveAssigneeNotAllowed
	}

	authorized, err := h.taskSvc.IsAuthorizedForNodeOperation(ctx, db, *task, cmd.Operator.ID)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if !authorized {
		return cqrs.Unit{}, shared.ErrNotAssignee
	}

	canRemove, err := h.taskSvc.CanRemoveAssigneeTask(ctx, db, h.engine, node, *task)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if !canRemove {
		return cqrs.Unit{}, shared.ErrLastAssigneeRemoval
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskRemoved); err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.taskSvc.ActivateDependentTasks(ctx, db, instance, node, task); err != nil {
		return cqrs.Unit{}, err
	}

	actionLog := cmd.Operator.NewActionLog(task.InstanceID, approval.ActionRemoveAssignee)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)

	actionLog.RemovedAssigneeIDs = []string{task.AssigneeID}
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	events := []approval.DomainEvent{
		approval.NewAssigneesRemovedEvent(task.InstanceID, task.TenantID, task.NodeID, task.ID, []string{task.AssigneeID}, map[string]string{task.AssigneeID: task.AssigneeName}),
	}

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}

	events = append(events, completionEvents...)

	// remove_assignee does not mutate form_data and HandleNodeCompletion has
	// already persisted any status / current_node_id / finished_at change
	// through the state machine. No extra UPDATE is required.

	behavior.EventCollectorFromContext(ctx).Add(events...)

	return cqrs.Unit{}, nil
}
