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

// RejectTaskCmd rejects a pending task.
type RejectTaskCmd struct {
	cqrs.BaseCommand

	TaskID      string
	Operator    approval.UserInfo
	Opinion     string
	FormData    map[string]any
	Attachments []string
	Caller      approval.CallerContext
}

// RejectTaskHandler handles the RejectTaskCmd command.
type RejectTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	nodeSvc       *service.NodeService
	validationSvc *service.ValidationService
	formStorage   *storage.Dispatcher
}

// NewRejectTaskHandler creates a new RejectTaskHandler. formStorage refreshes
// the table-mode physical projection after a rejecter edits form data; it may
// be nil in test fixtures that do not exercise table-mode storage.
func NewRejectTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	validationSvc *service.ValidationService,
	formStorage *storage.Dispatcher,
) *RejectTaskHandler {
	return &RejectTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		nodeSvc:       nodeSvc,
		validationSvc: validationSvc,
		formStorage:   formStorage,
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
		approval.NewTaskRejectedEvent(instance, task, node, cmd.Operator, cmd.Opinion),
	}

	// A rejected task may still leave the node running (e.g. "any" pass rule),
	// so unblock whatever its completion enables before evaluating the node —
	// otherwise a suspended "before" parent or queued "after" child could
	// strand the node short of a decision.
	activationEvents, err := h.taskSvc.ActivateDependentTasks(ctx, db, instance, node, task)
	if err != nil {
		return cqrs.Unit{}, err
	}

	events = append(events, activationEvents...)

	completionEvents, err := h.nodeSvc.HandleNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return cqrs.Unit{}, err
	}

	events = append(events, completionEvents...)

	actionLog := h.taskSvc.BuildActionLog(instance.ID, task, cmd.Operator, approval.ActionReject, service.ActionLogParams{Opinion: cmd.Opinion, Attachments: cmd.Attachments})
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	// Status / current_node_id / finished_at are already persisted by the
	// state machine through HandleNodeCompletion → ApplyInstanceTransition.
	// Only form_data — mutated locally via MergeFormData — still needs writing.
	if err := h.taskSvc.PersistInstanceFormData(ctx, db, instance); err != nil {
		return cqrs.Unit{}, err
	}

	// Keep the table-mode physical projection in lockstep with the form_data
	// just written — a rejecter can edit the fields a node grants them, and
	// those edits must reach the projection too. JSON mode is a no-op.
	if h.formStorage != nil {
		if err := h.formStorage.SyncInstanceProjection(ctx, db, instance); err != nil {
			return cqrs.Unit{}, fmt.Errorf("sync form projection: %w", err)
		}
	}

	behavior.EventCollectorFromContext(ctx).Add(events...)

	return cqrs.Unit{}, nil
}
