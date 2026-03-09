package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// AddCCCmd adds CC records for an instance.
type AddCCCmd struct {
	cqrs.BaseCommand

	InstanceID string
	CCUserIDs  []string
	OperatorID string
}

// AddCCHandler handles the AddCCCmd command.
type AddCCHandler struct {
	db           orm.DB
	taskSvc      *service.TaskService
	publisher    *dispatcher.EventPublisher
	userResolver approval.UserInfoResolver
}

// NewAddCCHandler creates a new AddCCHandler.
func NewAddCCHandler(db orm.DB, taskSvc *service.TaskService, publisher *dispatcher.EventPublisher, userResolver approval.UserInfoResolver) *AddCCHandler {
	return &AddCCHandler{db: db, taskSvc: taskSvc, publisher: publisher, userResolver: userResolver}
}

func (h *AddCCHandler) Handle(ctx context.Context, cmd AddCCCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance

	instance.ID = cmd.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		Select("status", "current_node_id").
		ForUpdate().
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, shared.ErrInstanceNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load instance: %w", err)
	}

	if instance.Status != approval.InstanceRunning || instance.CurrentNodeID == nil {
		return cqrs.Unit{}, shared.ErrInstanceCompleted
	}

	// Validate manual CC is allowed on current node
	var node approval.FlowNode

	node.ID = *instance.CurrentNodeID

	if err := db.NewSelect().
		Model(&node).
		Select("is_manual_cc_allowed").
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load current node: %w", err)
	}

	if !node.IsManualCCAllowed {
		return cqrs.Unit{}, shared.ErrManualCcNotAllowed
	}

	operatorID := strings.TrimSpace(cmd.OperatorID)
	if operatorID == "" {
		return cqrs.Unit{}, shared.ErrNotAssignee
	}

	if !h.taskSvc.IsAuthorizedForNodeOperation(ctx, db, approval.Task{
		InstanceID: instance.ID,
		NodeID:     *instance.CurrentNodeID,
	}, operatorID) {
		return cqrs.Unit{}, shared.ErrNotAssignee
	}

	userIDs := shared.NormalizeUniqueIDs(cmd.CCUserIDs)
	if len(userIDs) == 0 {
		return cqrs.Unit{}, nil
	}

	ccUserNames := shared.ResolveUserNameMapSilent(ctx, h.userResolver, userIDs)

	insertedUserIDs, err := shared.InsertManualCCRecords(ctx, db, cmd.InstanceID, *instance.CurrentNodeID, userIDs, ccUserNames)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if len(insertedUserIDs) == 0 {
		return cqrs.Unit{}, nil
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewCCNotifiedEvent(cmd.InstanceID, *instance.CurrentNodeID, insertedUserIDs, ccUserNames, true),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
