package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
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
	db          orm.DB
	taskSvc     *service.TaskService
	instanceSvc *service.InstanceService
}

// NewWithdrawHandler creates a new WithdrawHandler.
func NewWithdrawHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	instanceSvc *service.InstanceService,
) *WithdrawHandler {
	return &WithdrawHandler{db: db, taskSvc: taskSvc, instanceSvc: instanceSvc}
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

	now := timex.Now()
	instance.FinishedAt = &now

	if err := h.instanceSvc.Transition(ctx, db, &instance, approval.InstanceWithdrawn, "finished_at"); err != nil {
		if errors.Is(err, shared.ErrInvalidInstanceTransition) {
			return cqrs.Unit{}, shared.ErrWithdrawNotAllowed
		}

		return cqrs.Unit{}, err
	}

	if err := h.taskSvc.CancelInstanceTasks(ctx, db, cmd.InstanceID); err != nil {
		return cqrs.Unit{}, fmt.Errorf("cancel tasks on withdraw: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionWithdraw)
	if cmd.Reason != "" {
		actionLog.Opinion = &cmd.Reason
	}

	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.CollectorFromContext(ctx).Append(
		approval.NewInstanceWithdrawnEvent(cmd.InstanceID, instance.TenantID, cmd.Operator.ID),
	)

	return cqrs.Unit{}, nil
}
