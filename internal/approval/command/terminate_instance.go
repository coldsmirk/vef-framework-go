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

// TerminateInstanceCmd force-closes an approval instance (admin operation).
// Running, returned, and withdrawn instances can all be terminated — the
// state machine is the single authority on which statuses may close.
type TerminateInstanceCmd struct {
	cqrs.BaseCommand

	InstanceID string
	Operator   approval.UserInfo
	Reason     string
	Caller     approval.CallerContext
}

// TerminateInstanceHandler handles the TerminateInstanceCmd command.
type TerminateInstanceHandler struct {
	db          orm.DB
	taskSvc     *service.TaskService
	instanceSvc *service.InstanceService
}

// NewTerminateInstanceHandler creates a new TerminateInstanceHandler.
func NewTerminateInstanceHandler(
	db orm.DB,
	taskSvc *service.TaskService,
	instanceSvc *service.InstanceService,
) *TerminateInstanceHandler {
	return &TerminateInstanceHandler{db: db, taskSvc: taskSvc, instanceSvc: instanceSvc}
}

func (h *TerminateInstanceHandler) Handle(ctx context.Context, cmd TerminateInstanceCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	instance, err := h.instanceSvc.LoadForUpdate(ctx, db, cmd.InstanceID, cmd.Caller)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if !engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceTerminated) {
		return cqrs.Unit{}, shared.ErrTerminateNotAllowed
	}

	now := timex.Now()
	instance.FinishedAt = &now

	if err := h.instanceSvc.Transition(ctx, db, instance, approval.InstanceTerminated, "finished_at"); err != nil {
		if errors.Is(err, shared.ErrInvalidInstanceTransition) {
			return cqrs.Unit{}, shared.ErrTerminateNotAllowed
		}

		return cqrs.Unit{}, err
	}

	canceledEvents, err := h.taskSvc.CancelInstanceTasks(ctx, db, instance, "申请已被终止，任务取消")
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("cancel tasks on terminate: %w", err)
	}

	if err := engine.CancelActiveNodeVisits(ctx, db, cmd.InstanceID); err != nil {
		return cqrs.Unit{}, err
	}

	var reason *string
	if cmd.Reason != "" {
		reason = &cmd.Reason
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionTerminate)
	actionLog.Opinion = reason

	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	completed := approval.NewInstanceCompletedEvent(instance, approval.InstanceTerminated)
	completed.Reason = reason

	behavior.EventCollectorFromContext(ctx).Add(append(canceledEvents, completed)...)

	return cqrs.Unit{}, nil
}
