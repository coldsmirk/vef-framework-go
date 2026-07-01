package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// RollbackTaskCmd rolls back a task to a previous node.
type RollbackTaskCmd struct {
	cqrs.BaseCommand

	TaskID       string
	Operator     approval.OperatorInfo
	Opinion      string
	FormData     map[string]any
	TargetNodeID string
	Attachments  []string
	Caller       approval.CallerContext
}

// RollbackTaskHandler handles the RollbackTaskCmd command.
type RollbackTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	instanceSvc   *service.InstanceService
	validationSvc *service.ValidationService
	engine        *engine.FlowEngine
	formStorage   *storage.Dispatcher
}

// NewRollbackTaskHandler creates a new RollbackTaskHandler. formStorage refreshes
// the table-mode physical projection after a rollback rewrites form data; it may
// be nil in test fixtures that do not exercise table-mode storage.
func NewRollbackTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	instanceSvc *service.InstanceService,
	validationSvc *service.ValidationService,
	eng *engine.FlowEngine,
	formStorage *storage.Dispatcher,
) *RollbackTaskHandler {
	return &RollbackTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		instanceSvc:   instanceSvc,
		validationSvc: validationSvc,
		engine:        eng,
		formStorage:   formStorage,
	}
}

func (h *RollbackTaskHandler) Handle(ctx context.Context, cmd RollbackTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator, cmd.Caller, cmd.FormData)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.validationSvc.ValidateOpinion(tc.Node, cmd.Opinion); err != nil {
		return cqrs.Unit{}, err
	}

	instance, task, node := tc.Instance, tc.Task, tc.Node

	if !node.IsRollbackAllowed {
		return cqrs.Unit{}, shared.ErrRollbackNotAllowed
	}

	targetNodeID := strings.TrimSpace(cmd.TargetNodeID)
	if targetNodeID == "" {
		return cqrs.Unit{}, shared.ErrInvalidRollbackTarget
	}

	if err := h.validationSvc.ValidateRollbackTarget(ctx, db, instance, node, targetNodeID); err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.taskSvc.FinishTask(ctx, db, task, approval.TaskRolledBack); err != nil {
		return cqrs.Unit{}, err
	}

	canceledEvents, err := h.taskSvc.CancelRemainingTasks(ctx, db, instance.ID, node.ID, "节点被回退，剩余任务取消")
	if err != nil {
		return cqrs.Unit{}, err
	}

	// Resolve form data per the node's rollback data strategy (keep / clear).
	if err := h.instanceSvc.ApplyRollbackFormData(ctx, db, instance, targetNodeID, node.RollbackDataStrategy); err != nil {
		return cqrs.Unit{}, err
	}

	instance.CurrentNodeID = new(targetNodeID)

	var targetNode approval.FlowNode

	targetNode.ID = targetNodeID

	if err := db.NewSelect().
		Model(&targetNode).
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("find target node: %w", err)
	}

	events := canceledEvents

	if targetNode.Kind == approval.NodeStart {
		// Return to initiator: pause instance as returned. State machine
		// transition persists form_data / current_node_id / finished_at
		// in the same UPDATE.
		now := timex.Now()
		instance.FinishedAt = &now

		if err := h.instanceSvc.Transition(
			ctx, db, instance, approval.InstanceReturned,
			"form_data", "current_node_id", "finished_at",
		); err != nil {
			return cqrs.Unit{}, err
		}

		events = append(events,
			approval.NewInstanceReturnedEvent(instance.ID, instance.TenantID, node.ID, targetNodeID, cmd.Operator.ID),
		)
	} else {
		// Rollback to intermediate node: status stays running but
		// form_data / current_node_id need to be persisted before the
		// engine processes the target. ProcessNode handles any further
		// state transitions through ApplyInstanceTransition.
		if _, err := db.NewUpdate().
			Model(instance).
			Select("form_data", "current_node_id").
			WherePK().
			Exec(ctx); err != nil {
			return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
		}

		if err := h.engine.ProcessNode(ctx, db, instance, &targetNode); err != nil {
			return cqrs.Unit{}, fmt.Errorf("process rollback target node: %w", err)
		}

		events = append(events,
			approval.NewInstanceRolledBackEvent(instance.ID, instance.TenantID, node.ID, targetNodeID, cmd.Operator.ID),
		)
	}

	// Keep the table-mode physical projection in lockstep with the rolled-back
	// form data — the node's rollback strategy may have cleared or retained
	// fields, and either way the projection must match. JSON mode is a no-op.
	if h.formStorage != nil {
		if err := h.formStorage.SyncInstanceProjection(ctx, db, instance); err != nil {
			return cqrs.Unit{}, fmt.Errorf("sync form projection: %w", err)
		}
	}

	actionLog := h.taskSvc.BuildActionLog(
		instance.ID,
		task,
		cmd.Operator,
		approval.ActionRollback,
		service.ActionLogParams{Opinion: cmd.Opinion, RollbackToNodeID: targetNodeID, Attachments: cmd.Attachments},
	)
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.EventCollectorFromContext(ctx).Add(events...)

	return cqrs.Unit{}, nil
}
