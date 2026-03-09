package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// ReassignTaskCmd reassigns a pending task to a different user (admin operation).
type ReassignTaskCmd struct {
	cqrs.BaseCommand

	TaskID        string
	NewAssigneeID string
	Operator      approval.OperatorInfo
	Reason        string
}

// ReassignTaskHandler handles the ReassignTaskCmd command.
type ReassignTaskHandler struct {
	db           orm.DB
	publisher    *dispatcher.EventPublisher
	userResolver approval.UserInfoResolver
}

// NewReassignTaskHandler creates a new ReassignTaskHandler.
func NewReassignTaskHandler(
	db orm.DB,
	publisher *dispatcher.EventPublisher,
	userResolver approval.UserInfoResolver,
) *ReassignTaskHandler {
	return &ReassignTaskHandler{db: db, publisher: publisher, userResolver: userResolver}
}

func (h *ReassignTaskHandler) Handle(ctx context.Context, cmd ReassignTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var task approval.Task

	task.ID = cmd.TaskID

	if err := db.NewSelect().
		Model(&task).
		ForUpdate().
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, shared.ErrTaskNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load task: %w", err)
	}

	if task.Status != approval.TaskPending {
		return cqrs.Unit{}, shared.ErrTaskNotPending
	}

	newAssigneeID := strings.TrimSpace(cmd.NewAssigneeID)
	if newAssigneeID == "" || newAssigneeID == task.AssigneeID {
		return cqrs.Unit{}, shared.ErrInvalidTransferTarget
	}

	activeTaskCount, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", task.InstanceID).
				Equals("node_id", task.NodeID).
				Equals("assignee_id", newAssigneeID).
				In("status", []approval.TaskStatus{approval.TaskPending, approval.TaskWaiting})
		}).
		Count(ctx)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("query reassignment target active task: %w", err)
	}

	if activeTaskCount > 0 {
		return cqrs.Unit{}, shared.ErrInvalidTransferTarget
	}

	oldAssigneeID := task.AssigneeID
	oldAssigneeName := task.AssigneeName
	newAssigneeName := shared.ResolveUserName(ctx, h.userResolver, newAssigneeID)

	if _, err := db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("assignee_id", newAssigneeID).
		Set("assignee_name", newAssigneeName).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(task.ID)
		}).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update task assignee: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(task.InstanceID, approval.ActionReassign)
	actionLog.TaskID = &task.ID
	actionLog.NodeID = &task.NodeID
	actionLog.TransferToID = &newAssigneeID

	actionLog.TransferToName = &newAssigneeName
	if cmd.Reason != "" {
		actionLog.Opinion = &cmd.Reason
	}

	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewTaskReassignedEvent(task.ID, task.InstanceID, task.NodeID, oldAssigneeID, oldAssigneeName, newAssigneeID, newAssigneeName, cmd.Reason),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
