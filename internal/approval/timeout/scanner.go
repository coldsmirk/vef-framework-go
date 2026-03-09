package timeout

import (
	"context"
	"errors"
	"fmt"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var (
	errNilDeadline        = errors.New("task has nil deadline in timeout notify")
	errNoAdminUsers       = errors.New("node configured TimeoutActionTransferAdmin but has no admin users")
	errAllAdminsHaveTasks = errors.New("all admin users already have active tasks")
)

// Scanner scans for timed-out tasks and processes them.
type Scanner struct {
	db           orm.DB
	publisher    *dispatcher.EventPublisher
	taskSvc      *service.TaskService
	nodeSvc      *service.NodeService
	userResolver approval.UserInfoResolver
}

// systemOperator is the operator identity used for system-initiated actions.
var systemOperator = approval.OperatorInfo{ID: "system", Name: "系统"}

// NewScanner creates a new timeout scanner.
func NewScanner(
	db orm.DB,
	publisher *dispatcher.EventPublisher,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	userResolver approval.UserInfoResolver,
) *Scanner {
	return &Scanner{
		db:           db,
		publisher:    publisher,
		taskSvc:      taskSvc,
		nodeSvc:      nodeSvc,
		userResolver: userResolver,
	}
}

// ScanTimeouts finds tasks that have passed their deadline and processes them.
func (s *Scanner) ScanTimeouts(ctx context.Context) {
	var tasks []approval.Task

	if err := s.db.NewSelect().
		Model(&tasks).
		Select("id", "node_id", "instance_id", "assignee_id", "assignee_name", "deadline", "tenant_id", "status").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("status", string(approval.TaskPending)).
				IsNotNull("deadline").
				LessThan("deadline", timex.Now()).
				IsFalse("is_timeout")
		}).
		Scan(ctx); err != nil {
		logger.Errorf("Failed to scan timeout tasks: %v", err)

		return
	}

	if len(tasks) == 0 {
		return
	}

	logger.Infof("Found %d timed-out tasks", len(tasks))

	for i := range tasks {
		if err := s.processTimeout(ctx, &tasks[i]); err != nil {
			logger.Errorf("Failed to process timeout for task %s: %v", tasks[i].ID, err)
		}
	}
}

// processTimeout handles a single timed-out task.
func (s *Scanner) processTimeout(ctx context.Context, task *approval.Task) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var instance approval.Instance

		instance.ID = task.InstanceID
		if err := tx.NewSelect().
			Model(&instance).
			WherePK().
			ForUpdate().
			Scan(ctx); err != nil {
			return fmt.Errorf("load instance %s: %w", task.InstanceID, err)
		}

		freshTask := &approval.Task{}

		freshTask.ID = task.ID
		if err := tx.NewSelect().
			Model(freshTask).
			WherePK().
			ForUpdate().
			Scan(ctx); err != nil {
			return fmt.Errorf("load task %s: %w", task.ID, err)
		}

		if freshTask.IsTimeout {
			return nil
		}

		if freshTask.Status != approval.TaskPending {
			return nil
		}

		var node approval.FlowNode

		node.ID = freshTask.NodeID
		if err := tx.NewSelect().
			Model(&node).
			WherePK().
			Scan(ctx); err != nil {
			return fmt.Errorf("load node %s: %w", freshTask.NodeID, err)
		}

		freshTask.IsTimeout = true
		if _, err := tx.NewUpdate().
			Model(freshTask).
			WherePK().
			Select("is_timeout").
			Exec(ctx); err != nil {
			return fmt.Errorf("mark timeout: %w", err)
		}

		events, err := s.executeTimeoutAction(ctx, tx, freshTask, &instance, &node)
		if err != nil {
			return fmt.Errorf("execute timeout action: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, events)
	})
}

// executeTimeoutAction executes the configured timeout action for the node.
func (s *Scanner) executeTimeoutAction(
	ctx context.Context,
	tx orm.DB,
	task *approval.Task,
	instance *approval.Instance,
	node *approval.FlowNode,
) ([]approval.DomainEvent, error) {
	switch node.TimeoutAction {
	case approval.TimeoutActionNotify:
		return s.recordTimeoutNotify(ctx, tx, task)
	case approval.TimeoutActionAutoPass:
		return s.autoFinishTask(ctx, tx, task, instance, node, approval.TaskApproved)
	case approval.TimeoutActionAutoReject:
		return s.autoFinishTask(ctx, tx, task, instance, node, approval.TaskRejected)
	case approval.TimeoutActionTransferAdmin:
		return s.transferToAdmin(ctx, tx, task, node)
	default:
		return nil, nil
	}
}

// recordTimeoutNotify returns the timeout event for the timed-out task.
// Deduplication is handled by the is_timeout flag set in processTimeout.
func (*Scanner) recordTimeoutNotify(_ context.Context, _ orm.DB, task *approval.Task) ([]approval.DomainEvent, error) {
	if task.Deadline == nil {
		return nil, fmt.Errorf("%w: task %s", errNilDeadline, task.ID)
	}

	return []approval.DomainEvent{
		approval.NewTaskTimeoutEvent(
			task.ID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			task.AssigneeName,
			*task.Deadline,
		),
	}, nil
}

// autoFinishTask finishes a task with the given status and logs the action.
func (s *Scanner) autoFinishTask(
	ctx context.Context,
	tx orm.DB,
	task *approval.Task,
	instance *approval.Instance,
	node *approval.FlowNode,
	status approval.TaskStatus,
) ([]approval.DomainEvent, error) {
	if err := s.taskSvc.FinishTask(ctx, tx, task, status); err != nil {
		return nil, fmt.Errorf("finish task: %w", err)
	}

	actionType := approval.ActionApprove
	if status == approval.TaskRejected {
		actionType = approval.ActionReject
	}

	// For sequential approval auto-pass, activate the next waiting task before evaluating
	// node completion. If the node completes, HandleNodeCompletion will cancel all remaining tasks.
	// If not, the next task is correctly activated for the next assignee.
	if status == approval.TaskApproved && node.ApprovalMethod == approval.ApprovalSequential {
		if err := s.taskSvc.ActivateNextSequentialTask(ctx, tx, instance, node); err != nil {
			return nil, fmt.Errorf("activate next sequential task: %w", err)
		}
	}

	events := make([]approval.DomainEvent, 0, 2)
	if status == approval.TaskApproved {
		events = append(events,
			approval.NewTaskApprovedEvent(task.ID, task.InstanceID, node.ID, "system", "任务处理超时，系统自动通过"),
		)
	} else {
		events = append(events,
			approval.NewTaskRejectedEvent(task.ID, task.InstanceID, node.ID, "system", "任务处理超时，系统自动驳回"),
		)
	}

	completionEvents, err := s.nodeSvc.HandleNodeCompletion(ctx, tx, instance, node)
	if err != nil {
		return nil, fmt.Errorf("handle node completion: %w", err)
	}

	events = append(events, completionEvents...)

	if _, err := tx.NewUpdate().
		Model(instance).
		Select("current_node_id", "status", "finished_at").
		WherePK().
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("update instance: %w", err)
	}

	actionLog := systemOperator.NewActionLog(task.InstanceID, actionType)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)

	actionLog.Opinion = new("系统超时自动处理")
	if _, err := tx.NewInsert().
		Model(actionLog).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert action log: %w", err)
	}

	return events, nil
}

// transferToAdmin transfers a timed-out task to the node's admin users.
func (s *Scanner) transferToAdmin(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode) ([]approval.DomainEvent, error) {
	targetAdminIDs := shared.NormalizeUniqueIDs(node.AdminUserIDs)
	if len(targetAdminIDs) == 0 {
		return nil, fmt.Errorf("%w: node %q", errNoAdminUsers, node.Key)
	}

	// Finish the original task as transferred
	task.Status = approval.TaskTransferred
	task.FinishedAt = new(timex.Now())

	if _, err := tx.NewUpdate().
		Model(task).
		WherePK().
		Select("status", "finished_at").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("finish transferred task: %w", err)
	}

	events := make([]approval.DomainEvent, 0, len(targetAdminIDs)*2)
	pendingDeadline := shared.ComputeTaskDeadline(node.TimeoutHours)

	// Filter out admins who already have active tasks on this node.
	var existingAssigneeIDs []string
	if err := tx.NewSelect().
		Model((*approval.Task)(nil)).
		Select("assignee_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", task.InstanceID).
				Equals("node_id", task.NodeID).
				In("status", []approval.TaskStatus{approval.TaskPending, approval.TaskWaiting}).
				In("assignee_id", targetAdminIDs)
		}).
		Scan(ctx, &existingAssigneeIDs); err != nil {
		return nil, fmt.Errorf("query existing admin tasks: %w", err)
	}

	existingSet := collections.NewHashSetFrom(existingAssigneeIDs...)

	var eligibleAdminIDs []string
	for _, id := range targetAdminIDs {
		if !existingSet.Contains(id) {
			eligibleAdminIDs = append(eligibleAdminIDs, id)
		}
	}

	if len(eligibleAdminIDs) == 0 {
		return nil, fmt.Errorf("%w: node %q", errAllAdminsHaveTasks, node.Key)
	}

	adminNames := shared.ResolveUserNameMapSilent(ctx, s.userResolver, eligibleAdminIDs)

	// Create new tasks for eligible admin users
	for _, adminID := range eligibleAdminIDs {
		adminName := adminNames[adminID]

		newTask := &approval.Task{
			TenantID:     task.TenantID,
			InstanceID:   task.InstanceID,
			NodeID:       task.NodeID,
			AssigneeID:   adminID,
			AssigneeName: adminName,
			SortOrder:    0,
			Status:       approval.TaskPending,
			Deadline:     pendingDeadline,
		}
		if _, err := tx.NewInsert().
			Model(newTask).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("create admin task: %w", err)
		}

		events = append(events, approval.NewTaskTransferredEvent(
			task.ID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			task.AssigneeName,
			adminID,
			adminName,
			"任务处理超时，系统自动转交管理员",
		))

		events = append(events, approval.NewTaskCreatedEvent(
			newTask.ID,
			task.InstanceID,
			task.NodeID,
			adminID,
			adminName,
			pendingDeadline,
		))

		actionLog := systemOperator.NewActionLog(task.InstanceID, approval.ActionTransfer)
		actionLog.NodeID = new(task.NodeID)
		actionLog.TaskID = new(task.ID)
		actionLog.TransferToID = new(adminID)
		actionLog.TransferToName = &adminName

		actionLog.Opinion = new("任务处理超时，系统自动转交管理员")
		if _, err := tx.NewInsert().
			Model(actionLog).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert transfer action log: %w", err)
		}
	}

	return events, nil
}

// ScanPreWarnings finds tasks approaching their deadline and sends warning notifications.
func (s *Scanner) ScanPreWarnings(ctx context.Context) {
	var tasks []approval.Task

	if err := s.db.NewSelect().
		Model(&tasks).
		SelectModelColumns().
		Join((*approval.FlowNode)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("afn.id", "at.node_id")
		}).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("at.status", approval.TaskPending).
				IsNotNull("at.deadline").
				IsFalse("at.is_timeout").
				GreaterThan("afn.timeout_notify_before_hours", 0).
				// deadline - hours <= NOW(), equivalent to: deadline <= NOW() + hours
				LessThanOrEqualExpr("at.deadline", func(eb orm.ExprBuilder) any {
					return eb.DateAdd(eb.Now(), eb.Column("afn.timeout_notify_before_hours"), orm.UnitHour)
				}).
				IsFalse("at.is_pre_warning_sent")
		}).
		Scan(ctx); err != nil {
		logger.Errorf("Failed to scan pre-warning tasks: %v", err)

		return
	}

	for i := range tasks {
		task := &tasks[i]
		if task.Deadline == nil {
			continue
		}

		hoursLeft := max(int(task.Deadline.Until().Hours()), 0)

		if err := s.sendPreWarning(ctx, task, hoursLeft); err != nil {
			logger.Errorf("Failed to send pre-warning for task %s: %v", task.ID, err)
		}
	}
}

// sendPreWarning marks the task as pre-warning sent and publishes the warning event.
func (s *Scanner) sendPreWarning(ctx context.Context, task *approval.Task, hoursLeft int) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		result, err := tx.NewUpdate().
			Model((*approval.Task)(nil)).
			Set("is_pre_warning_sent", true).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(task.ID).
					IsFalse("is_pre_warning_sent")
			}).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("mark pre-warning sent: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("pre-warning rows affected: %w", err)
		}

		if affected == 0 {
			return nil
		}

		task.IsPreWarningSent = true

		evt := approval.NewTaskDeadlineWarningEvent(
			task.ID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			task.AssigneeName,
			*task.Deadline,
			hoursLeft,
		)

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{evt})
	})
}
