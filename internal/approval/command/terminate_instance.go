package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
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
	db        orm.DB
	taskSvc   *service.TaskService
	publisher *dispatcher.EventPublisher
}

// NewTerminateInstanceHandler creates a new TerminateInstanceHandler.
func NewTerminateInstanceHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	publisher *dispatcher.EventPublisher,
) *TerminateInstanceHandler {
	return &TerminateInstanceHandler{db: db, taskSvc: taskSvc, publisher: publisher}
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

	updateResult, err := db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("status", approval.InstanceTerminated).
		Set("finished_at", now).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(instance.ID).
				Equals("status", approval.InstanceRunning)
		}).
		Exec(ctx)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	affected, err := updateResult.RowsAffected()
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("get affected rows for instance update: %w", err)
	}

	if affected == 0 {
		return cqrs.Unit{}, shared.ErrInstanceNotRunning
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

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewInstanceCompletedEvent(cmd.InstanceID, approval.InstanceTerminated),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
