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
	"github.com/coldsmirk/vef-framework-go/timex"
)

// WithdrawInstanceCmd withdraws an approval instance.
type WithdrawInstanceCmd struct {
	cqrs.BaseCommand

	InstanceID string
	Operator   approval.UserInfo
	Reason     string
	Caller     approval.CallerContext
}

// WithdrawInstanceHandler handles the WithdrawInstanceCmd command.
type WithdrawInstanceHandler struct {
	db          orm.DB
	taskSvc     *service.TaskService
	instanceSvc *service.InstanceService
}

// NewWithdrawInstanceHandler creates a new WithdrawInstanceHandler.
func NewWithdrawInstanceHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	instanceSvc *service.InstanceService,
) *WithdrawInstanceHandler {
	return &WithdrawInstanceHandler{db: db, taskSvc: taskSvc, instanceSvc: instanceSvc}
}

func (h *WithdrawInstanceHandler) Handle(ctx context.Context, cmd WithdrawInstanceCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	instance, err := h.instanceSvc.LoadForUpdate(ctx, db, cmd.InstanceID, cmd.Caller)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if instance.ApplicantID != cmd.Operator.ID {
		return cqrs.Unit{}, shared.ErrNotApplicant
	}

	if !engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceWithdrawn) {
		return cqrs.Unit{}, shared.ErrWithdrawNotAllowed
	}

	now := timex.Now()
	instance.FinishedAt = &now

	if err := h.instanceSvc.Transition(ctx, db, instance, approval.InstanceWithdrawn, "finished_at"); err != nil {
		if errors.Is(err, shared.ErrInvalidInstanceTransition) {
			return cqrs.Unit{}, shared.ErrWithdrawNotAllowed
		}

		return cqrs.Unit{}, err
	}

	canceledEvents, err := h.taskSvc.CancelInstanceTasks(ctx, db, cmd.InstanceID, "申请已撤回，任务取消")
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("cancel tasks on withdraw: %w", err)
	}

	if err := engine.CancelActiveNodeVisits(ctx, db, cmd.InstanceID); err != nil {
		return cqrs.Unit{}, err
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionWithdraw)
	if cmd.Reason != "" {
		actionLog.Opinion = &cmd.Reason
	}

	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.EventCollectorFromContext(ctx).Add(append(canceledEvents,
		approval.NewInstanceWithdrawnEvent(cmd.InstanceID, instance.TenantID, cmd.Operator.ID),
	)...)

	return cqrs.Unit{}, nil
}
