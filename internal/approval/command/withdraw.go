package command

import (
	"context"
	"fmt"

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

// WithdrawCmd withdraws an approval instance.
type WithdrawCmd struct {
	cqrs.BaseCommand

	InstanceID string
	Operator   approval.OperatorInfo
	Reason     string
}

// WithdrawHandler handles the WithdrawCmd command.
type WithdrawHandler struct {
	db        orm.DB
	taskSvc   *service.TaskService
	publisher *dispatcher.EventPublisher
}

// NewWithdrawHandler creates a new WithdrawHandler.
func NewWithdrawHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	publisher *dispatcher.EventPublisher,
) *WithdrawHandler {
	return &WithdrawHandler{db: db, taskSvc: taskSvc, publisher: publisher}
}

func (h *WithdrawHandler) Handle(ctx context.Context, cmd WithdrawCmd) (cqrs.Unit, error) {
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

	if instance.ApplicantID != cmd.Operator.ID {
		return cqrs.Unit{}, shared.ErrNotApplicant
	}

	if !engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceWithdrawn) {
		return cqrs.Unit{}, shared.ErrWithdrawNotAllowed
	}

	originalStatus := instance.Status
	now := timex.Now()
	instance.Status = approval.InstanceWithdrawn
	instance.FinishedAt = &now

	updateResult, err := db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("status", instance.Status).
		Set("finished_at", now).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(instance.ID).
				Equals("status", originalStatus)
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
		return cqrs.Unit{}, shared.ErrWithdrawNotAllowed
	}

	if err := h.taskSvc.CancelInstanceTasks(ctx, db, cmd.InstanceID); err != nil {
		return cqrs.Unit{}, fmt.Errorf("cancel tasks on withdraw: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionWithdraw)
	if cmd.Reason != "" {
		actionLog.Opinion = &cmd.Reason
	}

	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewInstanceWithdrawnEvent(cmd.InstanceID, cmd.Operator.ID),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
