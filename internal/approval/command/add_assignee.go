package command

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
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
	Operator approval.UserInfo
	Caller   approval.CallerContext
}

// AddAssigneeHandler handles the AddAssigneeCmd command.
type AddAssigneeHandler struct {
	db           orm.DB
	taskSvc      *service.TaskService
	userResolver approval.UserInfoResolver
}

// NewAddAssigneeHandler creates a new AddAssigneeHandler.
func NewAddAssigneeHandler(db orm.DB, taskSvc *service.TaskService, userResolver approval.UserInfoResolver) *AddAssigneeHandler {
	return &AddAssigneeHandler{db: db, taskSvc: taskSvc, userResolver: userResolver}
}

func (h *AddAssigneeHandler) Handle(ctx context.Context, cmd AddAssigneeCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	tc, err := h.taskSvc.LoadTaskContextForNodeOperation(ctx, db, cmd.TaskID, service.TaskContextLoadOptions{
		OperatorID:              cmd.Operator.ID,
		RequireOperatorAssignee: true,
		RequireTaskPending:      true,
		RequireCurrentNode:      true,
		Caller:                  cmd.Caller,
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

	sequential := node.ApprovalMethod == approval.ApprovalSequential

	// A sequential node advances one task at a time through its sort-ordered
	// queue, so a "parallel" addition has nothing to join. Deploy validation
	// refuses to configure the type; this guards the empty-config default
	// that allows every type.
	if sequential && cmd.AddType == approval.AddAssigneeParallel {
		return cqrs.Unit{}, shared.ErrInvalidAddAssigneeType
	}

	userIDs := shared.NormalizeUniqueIDs(cmd.UserIDs)
	if len(userIDs) == 0 {
		return cqrs.Unit{}, shared.ErrNoUsersSpecified
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
	userInfos := shared.ResolveUserInfoMapSilent(ctx, h.userResolver, insertUsers)

	// insertBase is the sort position of the first inserted task. Parallel
	// nodes append after the node's max sort order — position is cosmetic
	// there. Sequential nodes splice into the live queue relative to the
	// anchor: "before" additions take over the anchor's position (the anchor
	// and the tail behind it shift back), "after" additions slot in right
	// behind it, so the queue reaches them in add order.
	insertBase := baseSortOrder + 1

	if sequential {
		shiftFrom := task.SortOrder
		if cmd.AddType == approval.AddAssigneeAfter {
			shiftFrom++
		}

		insertBase = shiftFrom

		if _, err := db.NewUpdate().
			Model((*approval.Task)(nil)).
			SetExpr("sort_order", func(eb orm.ExprBuilder) any {
				return eb.Add(eb.Column("sort_order"), len(insertUsers))
			}).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("visit_id", task.VisitID).
					GreaterThanOrEqual("sort_order", shiftFrom)
			}).
			Exec(ctx); err != nil {
			return cqrs.Unit{}, fmt.Errorf("shift sequential queue: %w", err)
		}
	}

	// For AddAssigneeBefore, suspend the original task before inserting new
	// ones. The task was already validated as Pending by LoadTaskContextForNodeOperation
	// (RequireTaskPending), and Pending→Waiting is always a legal transition, so
	// the update is unconditional — no CanTransition guard needed.
	if cmd.AddType == approval.AddAssigneeBefore {
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

	eventCollector := behavior.EventCollectorFromContext(ctx)

	for i, userID := range insertUsers {
		info := userInfos[userID]

		newTask := &approval.Task{
			TenantID:               instance.TenantID,
			InstanceID:             instance.ID,
			NodeID:                 task.NodeID,
			VisitID:                task.VisitID,
			AssigneeID:             userID,
			AssigneeName:           info.Name,
			AssigneeDepartmentID:   info.DepartmentID,
			AssigneeDepartmentName: info.DepartmentName,
			SortOrder:              insertBase + i,
			ParentTaskID:           new(task.ID),
			AddAssigneeType:        &cmd.AddType,
		}
		switch {
		case cmd.AddType == approval.AddAssigneeAfter, sequential && i > 0:
			// Queued: promoted by the sequential queue, or activated by the
			// parallel parent/child dependency once the anchor finishes.
			newTask.Status = approval.TaskWaiting
			newTask.Deadline = nil
		default:
			newTask.Status = approval.TaskPending
			newTask.Deadline = pendingDeadline
		}

		if _, err := db.NewInsert().
			Model(newTask).
			Exec(ctx); err != nil {
			return cqrs.Unit{}, fmt.Errorf("insert assignee task: %w", err)
		}

		eventCollector.Add(approval.NewTaskCreatedEvent(instance, newTask, node))
	}

	actionLog := cmd.Operator.NewActionLog(instance.ID, approval.ActionAddAssignee)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)
	actionLog.AddAssigneeType = &cmd.AddType
	actionLog.AddedAssignees = shared.UserInfos(insertUsers, userInfos)
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	eventCollector.Add(
		approval.NewAssigneesAddedEvent(instance, task, node, cmd.AddType, shared.UserInfos(insertUsers, userInfos)),
	)

	return cqrs.Unit{}, nil
}
