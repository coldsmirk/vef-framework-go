package timeout

import (
	"context"
	"errors"
	"fmt"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var errNilDeadline = errors.New("task has nil deadline in timeout notify")

// Scanner scans for timed-out tasks and processes them.
type Scanner struct {
	db           orm.DB
	bus          event.Bus
	taskSvc      *service.TaskService
	nodeSvc      *service.NodeService
	userResolver approval.UserInfoResolver
	cfg          *config.ApprovalConfig
}

// NewScanner creates a new timeout scanner.
func NewScanner(
	db orm.DB,
	bus event.Bus,
	taskSvc *service.TaskService,
	nodeSvc *service.NodeService,
	userResolver approval.UserInfoResolver,
	cfg *config.ApprovalConfig,
) *Scanner {
	return &Scanner{
		db:           db,
		bus:          bus,
		taskSvc:      taskSvc,
		nodeSvc:      nodeSvc,
		userResolver: userResolver,
		cfg:          cfg,
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
	return s.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
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

		return engine.PublishEventsTx(ctx, s.bus, tx, events...)
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
		return s.autoFinishTask(ctx, tx, task, instance, node, true)
	case approval.TimeoutActionAutoReject:
		return s.autoFinishTask(ctx, tx, task, instance, node, false)
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
		approval.NewTaskTimedOutEvent(
			task.ID,
			task.TenantID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			task.AssigneeName,
			*task.Deadline,
		),
	}, nil
}

// timeoutResolution describes how a timed-out task is auto-finished: the
// terminal status to apply, the audit action, the system opinion, and the
// matching domain event. Deriving it from the node kind keeps the timeout
// path's semantics identical to a human completing the same task — a handle
// task finishes as "handled", never as "approved".
type timeoutResolution struct {
	status   approval.TaskStatus
	action   approval.ActionType
	opinion  string
	newEvent func(task *approval.Task, nodeID, opinion string) approval.DomainEvent
}

// resolveTimeoutCompletion maps the configured auto action onto the node
// kind's natural completion semantics.
func resolveTimeoutCompletion(node *approval.FlowNode, autoApprove bool) timeoutResolution {
	switch {
	case !autoApprove:
		return timeoutResolution{
			status:  approval.TaskRejected,
			action:  approval.ActionReject,
			opinion: "任务处理超时，系统自动驳回",
			newEvent: func(task *approval.Task, nodeID, opinion string) approval.DomainEvent {
				return approval.NewTaskRejectedEvent(task.ID, task.TenantID, task.InstanceID, nodeID, shared.SystemOperator.ID, opinion)
			},
		}

	case node.Kind == approval.NodeHandle:
		return timeoutResolution{
			status:  approval.TaskHandled,
			action:  approval.ActionHandle,
			opinion: "任务处理超时，系统自动办结",
			newEvent: func(task *approval.Task, nodeID, opinion string) approval.DomainEvent {
				return approval.NewTaskHandledEvent(task.ID, task.TenantID, task.InstanceID, nodeID, shared.SystemOperator.ID, opinion)
			},
		}

	default:
		return timeoutResolution{
			status:  approval.TaskApproved,
			action:  approval.ActionApprove,
			opinion: "任务处理超时，系统自动通过",
			newEvent: func(task *approval.Task, nodeID, opinion string) approval.DomainEvent {
				return approval.NewTaskApprovedEvent(task.ID, task.TenantID, task.InstanceID, nodeID, shared.SystemOperator.ID, opinion)
			},
		}
	}
}

// autoFinishTask finishes a timed-out task with the node-appropriate terminal
// status and logs the action.
func (s *Scanner) autoFinishTask(
	ctx context.Context,
	tx orm.DB,
	task *approval.Task,
	instance *approval.Instance,
	node *approval.FlowNode,
	autoApprove bool,
) ([]approval.DomainEvent, error) {
	resolution := resolveTimeoutCompletion(node, autoApprove)

	if err := s.taskSvc.FinishTask(ctx, tx, task, resolution.status); err != nil {
		return nil, fmt.Errorf("finish task: %w", err)
	}

	// Unblock whatever this task's completion enables — the next task in a
	// sequential queue, or a suspended "before" parent / queued "after" child
	// on a parallel node — before evaluating node completion. If the node
	// completes, HandleNodeCompletion cancels all remaining tasks anyway.
	if err := s.taskSvc.ActivateDependentTasks(ctx, tx, instance, node, task); err != nil {
		return nil, fmt.Errorf("activate dependent tasks: %w", err)
	}

	events := make([]approval.DomainEvent, 0, 2)
	events = append(events, resolution.newEvent(task, node.ID, resolution.opinion))

	completionEvents, err := s.nodeSvc.HandleNodeCompletion(ctx, tx, instance, node)
	if err != nil {
		return nil, fmt.Errorf("handle node completion: %w", err)
	}

	events = append(events, completionEvents...)

	// HandleNodeCompletion already persisted any status / current_node_id /
	// finished_at change through the state machine — no extra UPDATE is
	// required here. autoFinishTask never touches form_data.

	actionLog := shared.SystemOperator.NewActionLog(task.InstanceID, resolution.action)
	actionLog.NodeID = new(task.NodeID)
	actionLog.TaskID = new(task.ID)

	actionLog.Opinion = new(resolution.opinion)
	if _, err := tx.NewInsert().
		Model(actionLog).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert action log: %w", err)
	}

	return events, nil
}

// transferToAdmin transfers a timed-out task to the node's admin users.
//
// Unresolvable transfers — no admins configured, or every admin already
// holding an active task — degrade to a timeout notification instead of
// failing: returning an error would roll back the is_timeout marker and turn
// every scanner tick into a fresh, identical failure. The task stays pending
// with its timeout flagged, and the emitted TaskTimedOutEvent gives operators
// the signal to intervene.
func (s *Scanner) transferToAdmin(ctx context.Context, tx orm.DB, task *approval.Task, node *approval.FlowNode) ([]approval.DomainEvent, error) {
	targetAdminIDs := shared.NormalizeUniqueIDs(node.AdminUserIDs)
	if len(targetAdminIDs) == 0 {
		logger.Warnf("Node %q configured transfer_admin timeout but has no admin users; marking task %s timeout only", node.Key, task.ID)

		return s.recordTimeoutNotify(ctx, tx, task)
	}

	// Resolve eligible admins (those without an active task on this node)
	// before finishing the original task, so a fully-occupied admin pool
	// degrades cleanly while the task is still pending.
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
		logger.Warnf("All admin users of node %q already hold active tasks; marking task %s timeout only", node.Key, task.ID)

		return s.recordTimeoutNotify(ctx, tx, task)
	}

	// Finish the original task as transferred via the state machine so the
	// transition is validated and the optimistic-lock UPDATE is consistent
	// with every other finish path.
	if err := s.taskSvc.FinishTask(ctx, tx, task, approval.TaskTransferred); err != nil {
		return nil, fmt.Errorf("finish transferred task: %w", err)
	}

	events := make([]approval.DomainEvent, 0, len(eligibleAdminIDs)*2)
	pendingDeadline := shared.ComputeTaskDeadline(node.TimeoutHours)

	adminNames := shared.ResolveUserNameMapSilent(ctx, s.userResolver, eligibleAdminIDs)

	// standInTaskID is the first admin replacement; the timed-out task's active
	// "after" children are re-parented onto it (below) so they are not orphaned.
	var standInTaskID string

	// Create new tasks for eligible admin users
	for _, adminID := range eligibleAdminIDs {
		adminName := adminNames[adminID]

		// Each replacement inherits the timed-out task's parent link and sort
		// order, so auto-transferring a "before" add-assignee child still
		// reactivates its suspended parent once an admin acts (mirrors the
		// user-initiated transfer path).
		newTask := &approval.Task{
			TenantID:        task.TenantID,
			InstanceID:      task.InstanceID,
			NodeID:          task.NodeID,
			AssigneeID:      adminID,
			AssigneeName:    adminName,
			SortOrder:       task.SortOrder,
			Status:          approval.TaskPending,
			Deadline:        pendingDeadline,
			ParentTaskID:    task.ParentTaskID,
			AddAssigneeType: task.AddAssigneeType,
		}
		if _, err := tx.NewInsert().
			Model(newTask).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("create admin task: %w", err)
		}

		if standInTaskID == "" {
			standInTaskID = newTask.ID
		}

		events = append(events, approval.NewTaskTransferredEvent(
			task.ID,
			task.TenantID,
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
			newTask.TenantID,
			task.InstanceID,
			task.NodeID,
			adminID,
			adminName,
			pendingDeadline,
		))

		actionLog := shared.SystemOperator.NewActionLog(task.InstanceID, approval.ActionTransfer)
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

	// Adopt the timed-out task's active "after" children onto the stand-in so a
	// parent-of-after-children that times out does not orphan them.
	if err := s.taskSvc.RepointAddAssigneeChildren(ctx, tx, task.ID, standInTaskID, task.InstanceID); err != nil {
		return nil, err
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
	return s.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
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
			task.TenantID,
			task.InstanceID,
			task.NodeID,
			task.AssigneeID,
			task.AssigneeName,
			*task.Deadline,
			hoursLeft,
		)

		return engine.PublishEventsTx(ctx, s.bus, tx, evt)
	})
}

// CleanupExpiredRecords prunes retention-bounded record tables: form
// snapshots, urge records, and read CC records. Each table has its own
// retention window configured via ApprovalConfig; rows older than the
// cutoff are deleted in a single statement.
//
// Action logs are kept indefinitely as the canonical audit trail.
func (s *Scanner) CleanupExpiredRecords(ctx context.Context) {
	now := timex.Now()

	formCutoff := now.Add(-s.cfg.FormSnapshotRetention)
	if n, err := s.db.NewDelete().
		Model((*approval.FormSnapshot)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.LessThan("created_at", formCutoff)
		}).
		Exec(ctx); err != nil {
		logger.Errorf("cleanup form snapshots failed: %v", err)
	} else if affected, _ := n.RowsAffected(); affected > 0 {
		logger.Infof("cleanup form snapshots: deleted %d rows older than %s", affected, formCutoff)
	}

	urgeCutoff := now.Add(-s.cfg.UrgeRecordRetention)
	if n, err := s.db.NewDelete().
		Model((*approval.UrgeRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.LessThan("created_at", urgeCutoff)
		}).
		Exec(ctx); err != nil {
		logger.Errorf("cleanup urge records failed: %v", err)
	} else if affected, _ := n.RowsAffected(); affected > 0 {
		logger.Infof("cleanup urge records: deleted %d rows older than %s", affected, urgeCutoff)
	}

	ccCutoff := now.Add(-s.cfg.CCRecordRetention)
	if n, err := s.db.NewDelete().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.LessThan("created_at", ccCutoff).
				IsNotNull("read_at")
		}).
		Exec(ctx); err != nil {
		logger.Errorf("cleanup cc records failed: %v", err)
	} else if affected, _ := n.RowsAffected(); affected > 0 {
		logger.Infof("cleanup cc records: deleted %d read rows older than %s", affected, ccCutoff)
	}
}
