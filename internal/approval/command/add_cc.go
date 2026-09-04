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
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// AddCCCmd adds CC records for an instance.
type AddCCCmd struct {
	cqrs.BaseCommand
	approval.AddCCInput
}

// AddCCHandler handles the AddCCCmd command.
type AddCCHandler struct {
	db           orm.DB
	taskSvc      *service.TaskService
	instanceSvc  *service.InstanceService
	userResolver approval.UserInfoResolver
}

// NewAddCCHandler creates a new AddCCHandler.
func NewAddCCHandler(db orm.DB, taskSvc *service.TaskService, instanceSvc *service.InstanceService, userResolver approval.UserInfoResolver) *AddCCHandler {
	return &AddCCHandler{db: db, taskSvc: taskSvc, instanceSvc: instanceSvc, userResolver: userResolver}
}

func (h *AddCCHandler) Handle(ctx context.Context, cmd AddCCCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	instance, err := h.instanceSvc.LoadForUpdate(ctx, db, cmd.InstanceID, cmd.Caller)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if instance.Status != approval.InstanceRunning || instance.CurrentNodeID == nil {
		return cqrs.Unit{}, approval.ErrInstanceCompleted
	}

	var node approval.FlowNode

	node.ID = *instance.CurrentNodeID

	if err := db.NewSelect().
		Model(&node).
		Select("name", "is_manual_cc_allowed").
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load current node: %w", err)
	}

	if !node.IsManualCCAllowed {
		return cqrs.Unit{}, approval.ErrManualCcNotAllowed
	}

	operatorID := strings.TrimSpace(cmd.Operator.ID)
	if operatorID == "" {
		return cqrs.Unit{}, approval.ErrNotAssignee
	}

	authorized, err := h.taskSvc.IsAuthorizedForNodeOperation(ctx, db, instance.ID, *instance.CurrentNodeID, operatorID)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if !authorized {
		return cqrs.Unit{}, approval.ErrNotAssignee
	}

	userIDs := shared.NormalizeUniqueIDs(cmd.CCUserIDs)
	if len(userIDs) == 0 {
		return cqrs.Unit{}, nil
	}

	ccUserInfos := shared.ResolveUserInfoMapSilent(ctx, h.userResolver, userIDs)

	visit, err := engine.FindActiveNodeVisit(ctx, db, cmd.InstanceID, *instance.CurrentNodeID)
	if err != nil {
		return cqrs.Unit{}, err
	}

	insertedUserIDs, err := shared.InsertManualCCRecords(ctx, db, cmd.InstanceID, *instance.CurrentNodeID, visit.ID, userIDs, ccUserInfos)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if len(insertedUserIDs) == 0 {
		return cqrs.Unit{}, nil
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionAddCC)
	actionLog.NodeID = new(*instance.CurrentNodeID)
	actionLog.CCUsers = shared.UserInfos(insertedUserIDs, ccUserInfos)
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewCCNotifiedEvent(instance, &node, shared.UserInfos(insertedUserIDs, ccUserInfos), true),
	)

	return cqrs.Unit{}, nil
}
