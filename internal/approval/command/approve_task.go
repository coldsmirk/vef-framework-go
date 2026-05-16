package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ApproveTaskCmd approves (or handles) a pending task.
type ApproveTaskCmd struct {
	cqrs.BaseCommand

	TaskID   string
	Operator approval.OperatorInfo
	Opinion  string
	FormData map[string]any
}

// ApproveTaskHandler handles the ApproveTaskCmd command.
type ApproveTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	nodeSvc       *service.NodeService
	validationSvc *service.ValidationService
}

// NewApproveTaskHandler creates a new ApproveTaskHandler.
func NewApproveTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	validSvc *service.ValidationService,
) *ApproveTaskHandler {
	return &ApproveTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		nodeSvc:       nodeSvc,
		validationSvc: validSvc,
	}
}

func (h *ApproveTaskHandler) Handle(ctx context.Context, cmd ApproveTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator.ID, cmd.FormData)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.validationSvc.ValidateOpinion(tc.Node, cmd.Opinion); err != nil {
		return cqrs.Unit{}, err
	}

	instance, task, node := tc.Instance, tc.Task, tc.Node

	isHandle := node.Kind == approval.NodeHandle

	targetStatus := approval.TaskApproved
	if isHandle {
		targetStatus = approval.TaskHandled
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, targetStatus); err != nil {
		return cqrs.Unit{}, err
	}

	var taskEvent approval.DomainEvent
	if isHandle {
		taskEvent = approval.NewTaskHandledEvent(task.ID, task.TenantID, instance.ID, node.ID, cmd.Operator.ID, cmd.Opinion)
	} else {
		taskEvent = approval.NewTaskApprovedEvent(task.ID, task.TenantID, instance.ID, node.ID, cmd.Operator.ID, cmd.Opinion)
	}

	events := []approval.DomainEvent{taskEvent}

	if node.ApprovalMethod == approval.ApprovalSequential {
		if err := h.taskSvc.ActivateNextSequentialTask(ctx, db, instance, node); err != nil {
			return cqrs.Unit{}, err
		}
	}

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}

	events = append(events, completionEvents...)

	actionType := approval.ActionApprove
	if isHandle {
		actionType = approval.ActionHandle
	}

	actionLog := h.taskSvc.BuildActionLog(instance.ID, task, cmd.Operator, actionType, service.ActionLogParams{Opinion: cmd.Opinion})
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	if _, err := db.NewUpdate().
		Model(instance).
		Select("form_data", "current_node_id", "status", "finished_at").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	behavior.CollectorFromContext(ctx).Append(events...)

	return cqrs.Unit{}, nil
}
