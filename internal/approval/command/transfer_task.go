package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// TransferTaskCmd transfers a pending task to another user.
type TransferTaskCmd struct {
	cqrs.BaseCommand

	TaskID       string
	Operator     approval.OperatorInfo
	Opinion      string
	FormData     map[string]any
	TransferToID string
}

// TransferTaskHandler handles the TransferTaskCmd command.
type TransferTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	validationSvc *service.ValidationService
	publisher     *dispatcher.EventPublisher
	userResolver  approval.UserInfoResolver
}

// NewTransferTaskHandler creates a new TransferTaskHandler.
func NewTransferTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	validationSvc *service.ValidationService,
	publisher *dispatcher.EventPublisher,
	userResolver approval.UserInfoResolver,
) *TransferTaskHandler {
	return &TransferTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		validationSvc: validationSvc,
		publisher:     publisher,
		userResolver:  userResolver,
	}
}

func (h *TransferTaskHandler) Handle(ctx context.Context, cmd TransferTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator.ID, cmd.FormData)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.validationSvc.ValidateOpinion(tc.Node, cmd.Opinion); err != nil {
		return cqrs.Unit{}, err
	}

	instance, task, node := tc.Instance, tc.Task, tc.Node

	if !node.IsTransferAllowed {
		return cqrs.Unit{}, shared.ErrTransferNotAllowed
	}

	transferToID := strings.TrimSpace(cmd.TransferToID)
	if transferToID == "" || transferToID == task.AssigneeID {
		return cqrs.Unit{}, shared.ErrInvalidTransferTarget
	}

	activeTaskCount, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				Equals("node_id", task.NodeID).
				Equals("assignee_id", transferToID).
				In("status", []approval.TaskStatus{approval.TaskPending, approval.TaskWaiting})
		}).
		Count(ctx)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("query transfer target active task: %w", err)
	}

	if activeTaskCount > 0 {
		return cqrs.Unit{}, shared.ErrInvalidTransferTarget
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskTransferred); err != nil {
		return cqrs.Unit{}, err
	}

	transferToName := shared.ResolveUserName(ctx, h.userResolver, transferToID)

	newTask := &approval.Task{
		TenantID:     instance.TenantID,
		InstanceID:   instance.ID,
		NodeID:       task.NodeID,
		AssigneeID:   transferToID,
		AssigneeName: transferToName,
		SortOrder:    task.SortOrder,
		Status:       approval.TaskPending,
		Deadline:     task.Deadline,
	}
	if _, err := db.NewInsert().Model(newTask).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert transfer task: %w", err)
	}

	events := []approval.DomainEvent{
		approval.NewTaskTransferredEvent(task.ID, instance.ID, node.ID, cmd.Operator.ID, cmd.Operator.Name, transferToID, transferToName, cmd.Opinion),
		approval.NewTaskCreatedEvent(newTask.ID, instance.ID, node.ID, transferToID, transferToName, task.Deadline),
	}

	if err := h.taskSvc.InsertActionLog(
		ctx,
		db,
		instance.ID,
		task,
		cmd.Operator,
		approval.ActionTransfer,
		service.ActionLogParams{Opinion: cmd.Opinion, TransferToID: transferToID, TransferToName: transferToName},
	); err != nil {
		return cqrs.Unit{}, err
	}

	if _, err := db.NewUpdate().
		Model(instance).
		Select("form_data").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, events); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
