package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
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
}

// RollbackTaskHandler handles the RollbackTaskCmd command.
type RollbackTaskHandler struct {
	db            orm.DB
	taskSvc       *service.TaskService
	validationSvc *service.ValidationService
	engine        *engine.FlowEngine
	publisher     *dispatcher.EventPublisher
}

// NewRollbackTaskHandler creates a new RollbackTaskHandler.
func NewRollbackTaskHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	validationSvc *service.ValidationService,
	eng *engine.FlowEngine,
	publisher *dispatcher.EventPublisher,
) *RollbackTaskHandler {
	return &RollbackTaskHandler{
		db:            db,
		taskSvc:       taskSvc,
		validationSvc: validationSvc,
		engine:        eng,
		publisher:     publisher,
	}
}

func (h *RollbackTaskHandler) Handle(ctx context.Context, cmd RollbackTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.PrepareOperation(ctx, db, cmd.TaskID, cmd.Operator.ID, cmd.FormData)
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

	if err := h.taskSvc.CancelRemainingTasks(ctx, db, instance.ID, node.ID); err != nil {
		return cqrs.Unit{}, err
	}

	// Restore form snapshot if rollback data strategy is "keep"
	if node.RollbackDataStrategy == approval.RollbackDataKeep {
		var snapshot approval.FormSnapshot

		err := db.NewSelect().
			Model(&snapshot).
			Select("form_data").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instance.ID).
					Equals("node_id", targetNodeID)
			}).
			Scan(ctx)

		switch {
		case err == nil && snapshot.FormData != nil:
			instance.FormData = snapshot.FormData
		case err != nil && !result.IsRecordNotFound(err):
			return cqrs.Unit{}, fmt.Errorf("load form snapshot: %w", err)
		}
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

	var events []approval.DomainEvent

	if targetNode.Kind == approval.NodeStart {
		// Return to initiator: pause instance as returned
		now := timex.Now()
		instance.Status = approval.InstanceReturned
		instance.FinishedAt = &now
		events = []approval.DomainEvent{
			approval.NewInstanceReturnedEvent(instance.ID, node.ID, targetNodeID, cmd.Operator.ID),
		}
	} else {
		// Rollback to intermediate node: continue processing
		if err := h.engine.ProcessNode(ctx, db, instance, &targetNode); err != nil {
			return cqrs.Unit{}, fmt.Errorf("process rollback target node: %w", err)
		}

		events = []approval.DomainEvent{
			approval.NewInstanceRolledBackEvent(instance.ID, node.ID, targetNodeID, cmd.Operator.ID),
		}
	}

	if err := h.taskSvc.InsertActionLog(
		ctx,
		db,
		instance.ID,
		task,
		cmd.Operator,
		approval.ActionRollback,
		service.ActionLogParams{Opinion: cmd.Opinion, RollbackToNodeID: targetNodeID},
	); err != nil {
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
