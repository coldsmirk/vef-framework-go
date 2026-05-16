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

// RejectTaskCmd rejects a pending task.
type RejectTaskCmd struct {
	cqrs.BaseCommand

	TaskID   string
	Operator approval.OperatorInfo
	Opinion  string
	FormData map[string]any
	Caller   approval.CallerContext
}

// RejectTaskHandler handles the RejectTaskCmd command.
type RejectTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	nodeSvc       *service.NodeService
	validationSvc *service.ValidationService
}

// NewRejectTaskHandler creates a new RejectTaskHandler.
func NewRejectTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	validationSvc *service.ValidationService,
) *RejectTaskHandler {
	return &RejectTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		nodeSvc:       nodeSvc,
		validationSvc: validationSvc,
	}
}

func (h *RejectTaskHandler) Handle(ctx context.Context, cmd RejectTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator, cmd.Caller, cmd.FormData)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.validationSvc.ValidateOpinion(tc.Node, cmd.Opinion); err != nil {
		return cqrs.Unit{}, err
	}

	instance, task, node := tc.Instance, tc.Task, tc.Node

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskRejected); err != nil {
		return cqrs.Unit{}, err
	}

	events := []approval.DomainEvent{
		approval.NewTaskRejectedEvent(task.ID, task.TenantID, instance.ID, node.ID, cmd.Operator.ID, cmd.Opinion),
	}

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}

	events = append(events, completionEvents...)

	actionLog := h.taskSvc.BuildActionLog(instance.ID, task, cmd.Operator, approval.ActionReject, service.ActionLogParams{Opinion: cmd.Opinion})
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	// Status / current_node_id / finished_at are already persisted by the
	// state machine through HandleNodeCompletion → ApplyInstanceTransition.
	// Only form_data — mutated locally via MergeFormData — still needs writing.
	if _, err := db.NewUpdate().
		Model(instance).
		Select("form_data").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance form_data: %w", err)
	}

	behavior.EventCollectorFromContext(ctx).Add(events...)

	return cqrs.Unit{}, nil
}
