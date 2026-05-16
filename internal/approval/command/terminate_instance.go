package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// TerminateInstanceCmd terminates a running approval instance (admin operation).
type TerminateInstanceCmd struct {
	cqrs.BaseCommand

	InstanceID string
	Operator   approval.OperatorInfo
	Reason     string
}

// TerminateInstanceHandler handles the TerminateInstanceCmd command.
type TerminateInstanceHandler struct {
	db          orm.DB
	taskSvc     *service.TaskService
	instanceSvc *service.InstanceService
	bus         event.Bus
}

// NewTerminateInstanceHandler creates a new TerminateInstanceHandler.
func NewTerminateInstanceHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	instanceSvc *service.InstanceService,
	bus event.Bus,
) *TerminateInstanceHandler {
	return &TerminateInstanceHandler{db: db, taskSvc: taskSvc, instanceSvc: instanceSvc, bus: bus}
}

func (h *TerminateInstanceHandler) Handle(ctx context.Context, cmd TerminateInstanceCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance

	instance.ID = cmd.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		ForUpdate().
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, shared.ErrInstanceNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load instance: %w", err)
	}

	if instance.Status != approval.InstanceRunning {
		return cqrs.Unit{}, shared.ErrInstanceNotRunning
	}

	now := timex.Now()
	instance.FinishedAt = &now

	if err := h.instanceSvc.Transition(ctx, db, &instance, approval.InstanceTerminated, "finished_at"); err != nil {
		if errors.Is(err, shared.ErrInvalidInstanceTransition) {
			return cqrs.Unit{}, shared.ErrInstanceNotRunning
		}

		return cqrs.Unit{}, err
	}

	if err := h.taskSvc.CancelInstanceTasks(ctx, db, cmd.InstanceID); err != nil {
		return cqrs.Unit{}, fmt.Errorf("cancel tasks on terminate: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionTerminate)
	if cmd.Reason != "" {
		actionLog.Opinion = &cmd.Reason
	}

	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.bus.PublishBatch(ctx, event.AsEvents([]approval.DomainEvent{
		approval.NewInstanceCompletedEvent(cmd.InstanceID, instance.TenantID, approval.InstanceTerminated),
	}), event.WithTx(db)); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
