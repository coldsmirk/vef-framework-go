package command

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// AddAssigneeCmd dynamically adds assignees to a task.
type AddAssigneeCmd struct {
	cqrs.BaseCommand

	TaskID   string
	UserIDs  []string
	AddType  approval.AddAssigneeType
	Operator approval.OperatorInfo
}

// AddAssigneeHandler handles the AddAssigneeCmd command.
type AddAssigneeHandler struct {
	db           orm.DB
	taskSvc      *service.TaskService
	publisher    *dispatcher.EventPublisher
	userResolver approval.UserInfoResolver
}

// NewAddAssigneeHandler creates a new AddAssigneeHandler.
func NewAddAssigneeHandler(db orm.DB, taskSvc *service.TaskService, publisher *dispatcher.EventPublisher, userResolver approval.UserInfoResolver) *AddAssigneeHandler {
	return &AddAssigneeHandler{db: db, taskSvc: taskSvc, publisher: publisher, userResolver: userResolver}
}

func (h *AddAssigneeHandler) Handle(ctx context.Context, cmd AddAssigneeCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.LoadTaskContextForNodeOperation(ctx, db, cmd.TaskID, service.TaskContextLoadOptions{
		OperatorID:              cmd.Operator.ID,
		RequireOperatorAssignee: true,
		RequireTaskPending:      true,
		RequireCurrentNode:      true,
	})
	if err != nil {
		return cqrs.Unit{}, err
	}

	instance := tc.Instance
	task := tc.Task
	node := tc.Node

	if !node.IsAddAssigneeAllowed {
		return cqrs.Unit{}, shared.ErrAddAssigneeNotAllowed
	}

	if !cmd.AddType.IsValid() {
		return cqrs.Unit{}, shared.ErrInvalidAddAssigneeType
	}

	if len(node.AddAssigneeTypes) > 0 && !slices.Contains(node.AddAssigneeTypes, cmd.AddType) {
		return cqrs.Unit{}, shared.ErrInvalidAddAssigneeType
	}

	userIDs := shared.NormalizeUniqueIDs(cmd.UserIDs)
	if len(userIDs) == 0 {
		return cqrs.Unit{}, nil
	}

	var nodeTasks []approval.Task
	if err := db.NewSelect().
		Model(&nodeTasks).
		Select("assignee_id", "status", "sort_order").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				Equals("node_id", task.NodeID)
		}).
		ForUpdate().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("query node tasks: %w", err)
	}

	existingAssignees := shared.NewOrderedUnique[string](len(nodeTasks) + len(userIDs))

	baseSortOrder := task.SortOrder
	for _, nodeTask := range nodeTasks {
		if nodeTask.SortOrder > baseSortOrder {
			baseSortOrder = nodeTask.SortOrder
		}

		if nodeTask.Status == approval.TaskPending || nodeTask.Status == approval.TaskWaiting {
			existingAssignees.Add(nodeTask.AssigneeID)
		}
	}

	insertUsers := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		if existingAssignees.Add(userID) {
			insertUsers = append(insertUsers, userID)
		}
	}

	if len(insertUsers) == 0 {
		return cqrs.Unit{}, nil
	}

	pendingDeadline := shared.ComputeTaskDeadline(node.TimeoutHours)
	userNames := shared.ResolveUserNameMapSilent(ctx, h.userResolver, insertUsers)

	// For AddAssigneeBefore, suspend the original task before inserting new ones.
	if cmd.AddType == approval.AddAssigneeBefore &&
		engine.TaskStateMachine.CanTransition(task.Status, approval.TaskWaiting) {
		task.Status = approval.TaskWaiting

		task.Deadline = nil
		if _, err := db.NewUpdate().
			Model(task).
			Select("status", "deadline").
			WherePK().
			Exec(ctx); err != nil {
			return cqrs.Unit{}, fmt.Errorf("update original task: %w", err)
		}
	}

	for i, userID := range insertUsers {
		newTask := &approval.Task{
			TenantID:        instance.TenantID,
			InstanceID:      instance.ID,
			NodeID:          task.NodeID,
			AssigneeID:      userID,
			AssigneeName:    userNames[userID],
			SortOrder:       baseSortOrder + i + 1,
			ParentTaskID:    new(task.ID),
			AddAssigneeType: &cmd.AddType,
		}
		switch cmd.AddType {
		case approval.AddAssigneeBefore, approval.AddAssigneeParallel:
			newTask.Status = approval.TaskPending
			newTask.Deadline = pendingDeadline
		case approval.AddAssigneeAfter:
			newTask.Status = approval.TaskWaiting
			newTask.Deadline = nil
		}

		if _, err := db.NewInsert().
			Model(newTask).
			Exec(ctx); err != nil {
			return cqrs.Unit{}, fmt.Errorf("insert assignee task: %w", err)
		}
	}

	actionLog := cmd.Operator.NewActionLog(instance.ID, approval.ActionAddAssignee)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)
	actionLog.AddAssigneeType = &cmd.AddType

	actionLog.AddedAssigneeIDs = insertUsers
	if _, err := db.NewInsert().
		Model(actionLog).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewAssigneesAddedEvent(instance.ID, task.NodeID, task.ID, cmd.AddType, insertUsers, userNames),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
