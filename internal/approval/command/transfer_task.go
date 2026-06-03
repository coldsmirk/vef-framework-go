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

// TransferTaskCmd transfers a pending task to another user.
type TransferTaskCmd struct {
	cqrs.BaseCommand

	TaskID       string
	Operator     approval.OperatorInfo
	Opinion      string
	FormData     map[string]any
	TransferToID string
	Caller       approval.CallerContext
}

// TransferTaskHandler handles the TransferTaskCmd command.
type TransferTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	validationSvc *service.ValidationService
	userResolver  approval.UserInfoResolver
}

// NewTransferTaskHandler creates a new TransferTaskHandler.
func NewTransferTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	validationSvc *service.ValidationService,
	userResolver approval.UserInfoResolver,
) *TransferTaskHandler {
	return &TransferTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		validationSvc: validationSvc,
		userResolver:  userResolver,
	}
}

func (h *TransferTaskHandler) Handle(ctx context.Context, cmd TransferTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator, cmd.Caller, cmd.FormData)
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

	duplicate, err := hasActiveTaskForAssignee(ctx, db, instance.ID, task.NodeID, transferToID)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("query transfer target active task: %w", err)
	}

	if duplicate {
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
		approval.NewTaskTransferredEvent(task.ID, task.TenantID, instance.ID, node.ID, cmd.Operator.ID, cmd.Operator.Name, transferToID, transferToName, cmd.Opinion),
		approval.NewTaskCreatedEvent(newTask.ID, newTask.TenantID, instance.ID, node.ID, transferToID, transferToName, task.Deadline),
	}

	actionLog := h.taskSvc.BuildActionLog(
		instance.ID,
		task,
		cmd.Operator,
		approval.ActionTransfer,
		service.ActionLogParams{Opinion: cmd.Opinion, TransferToID: transferToID, TransferToName: transferToName},
	)
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	if _, err := db.NewUpdate().
		Model(instance).
		Select("form_data").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	behavior.EventCollectorFromContext(ctx).Add(events...)

	return cqrs.Unit{}, nil
}
