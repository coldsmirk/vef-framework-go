package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ApproveTaskCmd approves (or handles) a pending task.
type ApproveTaskCmd struct {
	cqrs.BaseCommand

	TaskID      string
	Operator    approval.UserInfo
	Opinion     string
	FormData    map[string]any
	Attachments []string
	Caller      approval.CallerContext
}

// ApproveTaskHandler handles the ApproveTaskCmd command.
type ApproveTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	nodeSvc       *service.NodeService
	validationSvc *service.ValidationService
	formStorage   *storage.Dispatcher
}

// NewApproveTaskHandler creates a new ApproveTaskHandler. formStorage refreshes
// the table-mode physical projection after an approver edits form data; it may
// be nil in test fixtures that do not exercise table-mode storage.
func NewApproveTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	validSvc *service.ValidationService,
	formStorage *storage.Dispatcher,
) *ApproveTaskHandler {
	return &ApproveTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		nodeSvc:       nodeSvc,
		validationSvc: validSvc,
		formStorage:   formStorage,
	}
}

func (h *ApproveTaskHandler) Handle(ctx context.Context, cmd ApproveTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator, cmd.Caller, cmd.FormData)
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

	if err := h.taskSvc.ActivateDependentTasks(ctx, db, instance, node, task); err != nil {
		return cqrs.Unit{}, err
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

	actionLog := h.taskSvc.BuildActionLog(instance.ID, task, cmd.Operator, actionType, service.ActionLogParams{Opinion: cmd.Opinion, Attachments: cmd.Attachments})
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	// Status / current_node_id / finished_at are already persisted by the
	// state machine through HandleNodeCompletion → ApplyInstanceTransition
	// (or by the engine's NodeActionComplete / NodeActionWait paths). Only
	// form_data — mutated locally via MergeFormData — still needs writing.
	if err := h.taskSvc.PersistInstanceFormData(ctx, db, instance); err != nil {
		return cqrs.Unit{}, err
	}

	// Keep the table-mode physical projection in lockstep with the form_data
	// just written — an approver can edit the fields a node grants them, and
	// those edits must reach the projection too. JSON mode is a no-op.
	if h.formStorage != nil {
		if err := h.formStorage.SyncInstanceProjection(ctx, db, instance); err != nil {
			return cqrs.Unit{}, fmt.Errorf("sync form projection: %w", err)
		}
	}

	behavior.EventCollectorFromContext(ctx).Add(events...)

	return cqrs.Unit{}, nil
}
