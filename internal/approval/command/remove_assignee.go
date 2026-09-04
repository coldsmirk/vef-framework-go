package command

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// RemoveAssigneeCmd removes an assignee by canceling their task.
type RemoveAssigneeCmd struct {
	cqrs.BaseCommand
	approval.RemoveAssigneeInput
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
		return cqrs.Unit{}, approval.ErrRemoveAssigneeNotAllowed
	}

	authorized, err := h.taskSvc.IsAuthorizedForNodeOperation(ctx, db, task.InstanceID, task.NodeID, cmd.Operator.ID)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if !authorized {
		return cqrs.Unit{}, approval.ErrNotAssignee
	}

	canRemove, err := h.taskSvc.CanRemoveAssigneeTask(ctx, db, h.engine, node, *task)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if !canRemove {
		return cqrs.Unit{}, approval.ErrLastAssigneeRemoval
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskRemoved); err != nil {
		return cqrs.Unit{}, err
	}

	activationEvents, err := h.taskSvc.ActivateDependentTasks(ctx, db, instance, node, task)
	if err != nil {
		return cqrs.Unit{}, err
	}

	actionLog := cmd.Operator.NewActionLog(task.InstanceID, approval.ActionRemoveAssignee)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)
	actionLog.RemovedAssignees = []approval.UserInfo{{
		ID:             task.AssigneeID,
		Name:           task.AssigneeName,
		DepartmentID:   task.AssigneeDepartmentID,
		DepartmentName: task.AssigneeDepartmentName,
	}}
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	events := behavior.EventCollectorFromContext(ctx)

	// The removal is announced before the node evaluation it triggers: that
	// evaluation emits its own events as it runs — removing the last blocking
	// assignee can complete the instance — so deferring this one would let the
	// consequence reach subscribers ahead of its cause.
	events.Add(approval.NewAssigneesRemovedEvent(instance, task, node, []approval.UserInfo{task.Assignee()}))

	// Queue-advance decisions (same-applicant auto-passes) happened before the
	// node evaluation below and may be its cause, so they must reach
	// subscribers first; the activations stay provisional until reconciled.
	decisionEvents, activationEvents := service.SplitQueueAdvanceDecisions(activationEvents)
	events.Add(decisionEvents...)

	// HandleNodeCompletion has already emitted what it produced; the return
	// value is only the reconciliation input for the activations above.
	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}

	// Activations precede completion in the lifecycle, but only those the
	// completion did not cancel actually happened.
	events.Add(service.SuppressSupersededActivations(activationEvents, completionEvents)...)

	// remove_assignee does not mutate form_data and HandleNodeCompletion has
	// already persisted any status / current_node_id / finished_at change
	// through the state machine. No extra UPDATE is required.

	return cqrs.Unit{}, nil
}
