package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
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
}

// RejectTaskHandler handles the RejectTaskCmd command.
type RejectTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	nodeSvc       *service.NodeService
	validationSvc *service.ValidationService
	publisher     *dispatcher.EventPublisher
}

// NewRejectTaskHandler creates a new RejectTaskHandler.
func NewRejectTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	validationSvc *service.ValidationService,
	publisher *dispatcher.EventPublisher,
) *RejectTaskHandler {
	return &RejectTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		nodeSvc:       nodeSvc,
		validationSvc: validationSvc,
		publisher:     publisher,
	}
}

func (h *RejectTaskHandler) Handle(ctx context.Context, cmd RejectTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator.ID, cmd.FormData)
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
		approval.NewTaskRejectedEvent(task.ID, instance.ID, node.ID, cmd.Operator.ID, cmd.Opinion),
	}

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}

	events = append(events, completionEvents...)

	if err := h.taskSvc.InsertActionLog(ctx, db, instance.ID, task, cmd.Operator, approval.ActionReject, service.ActionLogParams{Opinion: cmd.Opinion}); err != nil {
		return cqrs.Unit{}, err
	}

	if _, err := db.NewUpdate().
		Model(instance).
		Select("form_data", "current_node_id", "status", "finished_at").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, events); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
