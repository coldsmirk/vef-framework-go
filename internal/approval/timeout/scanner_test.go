package timeout_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/approval/timeout"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ScannerTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// ScannerTestSuite tests timeout scanner behavior.
type ScannerTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	bus     *eventtest.FakeBus
	scanner *timeout.Scanner
	seq     int
}

func (s *ScannerTestSuite) SetupSuite() {
	passRules := []approval.PassRuleStrategy{
		strategy.NewAllPassStrategy(),
		strategy.NewOnePassStrategy(),
		strategy.NewRatioPassStrategy(),
		strategy.NewOneRejectStrategy(),
	}
	assigneeResolvers := []strategy.AssigneeResolver{
		strategy.NewUserAssigneeResolver(),
		strategy.NewSelfAssigneeResolver(),
	}
	registry := strategy.NewStrategyRegistry(passRules, assigneeResolvers, nil)
	s.bus = eventtest.NewFakeBus()
	eng := engine.NewFlowEngine(registry, []engine.NodeProcessor{
		engine.NewStartProcessor(),
		engine.NewEndProcessor(),
		engine.NewConditionProcessor(),
		engine.NewApprovalProcessor(nil),
		engine.NewHandleProcessor(nil),
		engine.NewCCProcessor(shared.NewCCRecipientResolver(nil)),
	}, s.bus, nil, nil, nil)
	taskSvc := service.NewTaskService()
	nodeSvc := service.NewNodeService(eng, s.bus, taskSvc, nil, shared.NewCCRecipientResolver(nil))

	cfg := new(config.ApprovalConfig)
	cfg.ApplyDefaults()
	s.scanner = timeout.NewScanner(s.db, s.bus, taskSvc, nodeSvc, nil, cfg)
}

func (s *ScannerTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.ActionLog)(nil),
		(*approval.CCRecord)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
		(*approval.FlowEdge)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
		(*approval.Flow)(nil),
		(*approval.FlowCategory)(nil),
	)
	s.bus.Reset()
}

func deleteAll(ctx context.Context, db orm.DB, models ...any) {
	for _, model := range models {
		_, _ = db.NewDelete().
			Model(model).
			Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
			Exec(ctx)
	}
}

func isSQLite(ctx context.Context, db orm.DB) bool {
	var version string

	return db.NewRaw("SELECT sqlite_version()").Scan(ctx, &version) == nil
}

func (s *ScannerTestSuite) nextCode(prefix string) string {
	s.seq++

	return fmt.Sprintf("%s-%03d", prefix, s.seq)
}

func (s *ScannerTestSuite) createTimeoutScenario(timeoutAction approval.TimeoutAction) (*approval.Instance, *approval.Task) {
	return s.createTimeoutScenarioWithTaskStatus(timeoutAction, approval.TaskPending)
}

func (s *ScannerTestSuite) createTimeoutScenarioWithTaskStatus(
	timeoutAction approval.TimeoutAction,
	taskStatus approval.TaskStatus,
) (*approval.Instance, *approval.Task) {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     s.nextCode("timeout-cat"),
		Name:     "Timeout Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert timeout test category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   s.nextCode("timeout-flow"),
		Name:                   "Timeout Flow",
		BindingMode:            approval.BindingStandalone,
		InstanceTitleTemplate:  "Timeout Instance",
		IsAllInitiationAllowed: true,
		IsActive:               true,
		CurrentVersion:         1,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert timeout test flow")

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "Should insert timeout test flow version")

	startNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		Key:           s.nextCode("start"),
		Kind:          approval.NodeStart,
		Name:          "Start",
	}
	_, err = s.db.NewInsert().Model(startNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert start node")

	approvalNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		Key:           s.nextCode("approval"),
		Kind:          approval.NodeApproval,
		Name:          "Approval",
		PassRule:      approval.PassAll,
		TimeoutAction: timeoutAction,
	}
	_, err = s.db.NewInsert().Model(approvalNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert approval node")

	endNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		Key:           s.nextCode("end"),
		Kind:          approval.NodeEnd,
		Name:          "End",
	}
	_, err = s.db.NewInsert().Model(endNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert end node")

	_, err = s.db.NewInsert().Model(&approval.FlowEdge{
		FlowVersionID: version.ID,
		Key:           s.nextCode("edge"),
		SourceNodeID:  startNode.ID,
		SourceNodeKey: startNode.Key,
		TargetNodeID:  approvalNode.ID,
		TargetNodeKey: approvalNode.Key,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert start-to-approval edge")

	_, err = s.db.NewInsert().Model(&approval.FlowEdge{
		FlowVersionID: version.ID,
		Key:           s.nextCode("edge"),
		SourceNodeID:  approvalNode.ID,
		SourceNodeKey: approvalNode.Key,
		TargetNodeID:  endNode.ID,
		TargetNodeKey: endNode.Key,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert approval-to-end edge")

	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        flow.ID,
		FlowVersionID: version.ID,
		Title:         "Timeout Instance",
		InstanceNo:    s.nextCode("TIMEOUT-INS"),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &approvalNode.ID,
	}
	_, err = s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert running instance")

	deadline := timex.Now().AddHours(-1)
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instance.ID,
		NodeID:     approvalNode.ID,
		AssigneeID: "approver-1",
		Status:     taskStatus,
		Deadline:   &deadline,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should insert timed-out task")

	return instance, task
}

func (s *ScannerTestSuite) TestAutoPassTimeoutShouldAdvanceFlow() {
	instance, task := s.createTimeoutScenario(approval.TimeoutActionAutoPass)

	s.scanner.ScanTimeouts(s.ctx)

	var updatedTask approval.Task

	updatedTask.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&updatedTask).WherePK().Scan(s.ctx),
		"Should load updated timed-out task",
	)
	s.Assert().Equal(approval.TaskApproved, updatedTask.Status, "Timed-out task should be auto-approved")

	var updatedInstance approval.Instance

	updatedInstance.ID = instance.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&updatedInstance).WherePK().Scan(s.ctx),
		"Should load updated instance after auto-pass",
	)
	s.Assert().Equal(approval.InstanceApproved, updatedInstance.Status, "Instance should advance to approved after timeout auto-pass")
}

func (s *ScannerTestSuite) TestAutoRejectTimeoutShouldCompleteFlowAsRejected() {
	instance, task := s.createTimeoutScenario(approval.TimeoutActionAutoReject)

	s.scanner.ScanTimeouts(s.ctx)

	var updatedTask approval.Task

	updatedTask.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&updatedTask).WherePK().Scan(s.ctx),
		"Should load updated timed-out task",
	)
	s.Assert().Equal(approval.TaskRejected, updatedTask.Status, "Timed-out task should be auto-rejected")

	var updatedInstance approval.Instance

	updatedInstance.ID = instance.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&updatedInstance).WherePK().Scan(s.ctx),
		"Should load updated instance after auto-reject",
	)
	s.Assert().Equal(approval.InstanceRejected, updatedInstance.Status, "Instance should become rejected after timeout auto-reject")
}

func (s *ScannerTestSuite) TestTransferAdminTimeoutShouldAlignCreatedTasksWithTransferEventsAndLogs() {
	_, task := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionTransferAdmin, approval.TaskPending)

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("admin_user_ids", []string{"admin-1", "admin-2"}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node admin users for timeout transfer")

	s.scanner.ScanTimeouts(s.ctx)

	var tasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", task.InstanceID) }).
			Scan(s.ctx),
		"Should load tasks after timeout transfer",
	)
	s.Require().Len(tasks, 3, "Timeout transfer to two admins should keep original task and create two admin tasks")

	adminPending := make([]string, 0, 2)

	var original approval.Task
	for i := range tasks {
		if tasks[i].ID == task.ID {
			original = tasks[i]

			continue
		}

		if tasks[i].Status == approval.TaskPending {
			adminPending = append(adminPending, tasks[i].AssigneeID)
		}
	}

	s.Assert().Equal(approval.TaskTransferred, original.Status, "Original timed-out task should become transferred")
	s.Assert().True(original.IsTimeout, "Original timed-out task should be marked as timeout")
	s.Assert().ElementsMatch([]string{"admin-1", "admin-2"}, adminPending, "Pending tasks should be created for all node admins")

	s.Assert().Len(
		s.bus.CapturedByType("approval.task.transferred"),
		2,
		"Should emit one transfer event per created admin task",
	)
	s.Assert().Len(
		s.bus.CapturedByType("approval.task.created"),
		2,
		"Should emit one task-created event per admin task",
	)

	var logs []approval.ActionLog
	s.Require().NoError(
		s.db.NewSelect().
			Model(&logs).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", task.InstanceID).
					Equals("action", approval.ActionTransfer)
			}).
			Scan(s.ctx),
		"Should load transfer action logs after timeout transfer",
	)
	s.Require().Len(logs, 2, "Should record one transfer action log per created admin task")

	transferTargets := make([]string, 0, 2)
	for i := range logs {
		s.Require().NotNil(logs[i].TransferToID, "Transfer action log should record transfer target")
		transferTargets = append(transferTargets, *logs[i].TransferToID)
	}

	s.Assert().ElementsMatch([]string{"admin-1", "admin-2"}, transferTargets, "Transfer action logs should match all admin transfer targets")
}

func (s *ScannerTestSuite) TestTransferAdminTimeoutShouldStartDeadlineForNewAdminTasks() {
	_, task := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionTransferAdmin, approval.TaskPending)

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("admin_user_ids", []string{"admin-deadline"}).
		Set("timeout_hours", 4).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node admin users and timeout hours for transfer deadline test")

	s.scanner.ScanTimeouts(s.ctx)

	var adminTasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&adminTasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", task.InstanceID).
					Equals("assignee_id", "admin-deadline").
					Equals("status", approval.TaskPending)
			}).
			Scan(s.ctx),
		"Should query admin pending tasks created by timeout transfer",
	)
	s.Require().Len(adminTasks, 1, "Timeout transfer should create one pending admin task")
	s.Require().NotNil(adminTasks[0].Deadline, "Transferred pending admin task should start timeout deadline")
	// Timezone-agnostic: CreatedAt and Deadline come from the same row via the same driver
	// Scan path, so any timezone drift cancels out. TimeoutHours=4, so the admin task's
	// deadline must sit at least 3h past CreatedAt.
	s.Assert().True(
		adminTasks[0].Deadline.Unwrap().After(adminTasks[0].CreatedAt.AddHours(3).Unwrap()),
		"Transferred pending admin task deadline should be computed from transfer time",
	)
}

func (s *ScannerTestSuite) TestScanTimeoutsShouldLockInstanceBeforeTask() {
	if isSQLite(s.ctx, s.db) {
		s.T().Skip("SQLite does not support row-level FOR UPDATE lock semantics required by this test")
	}

	instance, task := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionNotify, approval.TaskPending)

	taskLocked := make(chan struct{})
	releaseTask := make(chan struct{})
	lockDone := make(chan error, 1)

	go func() {
		lockDone <- s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error {
			lockedTask := approval.Task{}

			lockedTask.ID = task.ID
			if err := tx.NewSelect().
				Model(&lockedTask).
				WherePK().
				ForUpdate().
				Scan(ctx); err != nil {
				return err
			}

			close(taskLocked)
			<-releaseTask

			return nil
		})
	}()

	<-taskLocked

	scanDone := make(chan struct{})
	go func() {
		s.scanner.ScanTimeouts(s.ctx)
		close(scanDone)
	}()

	instanceLockBlocked := false
	for range 20 {
		lockCtx, cancel := context.WithTimeout(s.ctx, 120*time.Millisecond)
		err := s.db.RunInTx(lockCtx, func(ctx context.Context, tx orm.DB) error {
			lockedInstance := approval.Instance{}
			lockedInstance.ID = instance.ID

			return tx.NewSelect().
				Model(&lockedInstance).
				WherePK().
				ForUpdate().
				Scan(ctx)
		})

		cancel()

		if err != nil {
			errText := strings.ToLower(err.Error())
			isLockWaitTimeout := errors.Is(err, context.DeadlineExceeded) ||
				strings.Contains(errText, "deadline") ||
				strings.Contains(errText, "timeout")
			s.Require().True(
				isLockWaitTimeout,
				"Lock probe should only fail due to lock wait timeout, got: %v",
				err,
			)

			instanceLockBlocked = true

			break
		}

		time.Sleep(30 * time.Millisecond)
	}

	s.Assert().True(
		instanceLockBlocked,
		"Timeout scanner should lock instance before waiting task lock to maintain global lock order",
	)

	close(releaseTask)

	s.Require().NoError(<-lockDone, "Task lock transaction should complete without error")

	select {
	case <-scanDone:
	case <-time.After(5 * time.Second):
		s.FailNow("Timeout scanner should complete after task lock is released")
	}
}

func (s *ScannerTestSuite) TestScanPreWarningsShouldSendWarningForPendingTasks() {
	_, task := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionNotify, approval.TaskPending)

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("timeout_notify_before_hours", 2).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node pre-warning window")

	// Use a clearly expired deadline to avoid DB/session timezone differences in date arithmetic.
	deadline := timex.Now().AddHours(-24)
	_, err = s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("deadline", deadline).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure pending task deadline for pre-warning range")

	s.scanner.ScanPreWarnings(s.ctx)

	var updatedTask approval.Task

	updatedTask.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&updatedTask).WherePK().Scan(s.ctx),
		"Should load task after pre-warning scan",
	)
	s.Assert().True(updatedTask.IsPreWarningSent, "Pending task should be marked as pre-warning sent")

	s.Assert().Len(
		s.bus.CapturedByType("approval.task.deadline_warning"),
		1,
		"Pending task in warning window should emit one warning event",
	)
}

func (s *ScannerTestSuite) TestScanTimeoutsShouldIgnoreWaitingTasks() {
	_, task := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionAutoReject, approval.TaskWaiting)

	s.scanner.ScanTimeouts(s.ctx)

	var updatedTask approval.Task

	updatedTask.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&updatedTask).WherePK().Scan(s.ctx),
		"Should load task after timeout scan",
	)
	s.Assert().Equal(approval.TaskWaiting, updatedTask.Status, "Waiting task should not be auto-processed by timeout scanner")
	s.Assert().False(updatedTask.IsTimeout, "Waiting task should not be marked as timeout")

	s.Assert().Empty(
		s.bus.Captured(),
		"Ignoring waiting task should not publish timeout events",
	)
}

func (s *ScannerTestSuite) TestScanPreWarningsShouldBeIdempotent() {
	_, task := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionNotify, approval.TaskPending)

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("timeout_notify_before_hours", 2).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node pre-warning window")

	deadline := timex.Now().AddHours(-24)
	_, err = s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("deadline", deadline).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure task deadline for pre-warning range")

	s.scanner.ScanPreWarnings(s.ctx)
	s.scanner.ScanPreWarnings(s.ctx)

	s.Assert().Len(
		s.bus.CapturedByType("approval.task.deadline_warning"),
		1,
		"Idempotent pre-warning should emit only one event across two scans",
	)
}

func (s *ScannerTestSuite) TestScanPreWarningsShouldIgnoreWaitingTasks() {
	_, task := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionNotify, approval.TaskWaiting)

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("timeout_notify_before_hours", 2).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node pre-warning window")

	deadline := timex.Now().AddHours(1)
	_, err = s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("deadline", deadline).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure waiting task deadline for pre-warning range")

	s.scanner.ScanPreWarnings(s.ctx)

	var updatedTask approval.Task

	updatedTask.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&updatedTask).WherePK().Scan(s.ctx),
		"Should load task after pre-warning scan",
	)
	s.Assert().False(updatedTask.IsPreWarningSent, "Waiting task should not be marked as pre-warning sent")

	s.Assert().Empty(
		s.bus.CapturedByType("approval.task.deadline_warning"),
		"Ignoring waiting task should not emit warning events",
	)
}

func (s *ScannerTestSuite) makeNodeParallel(nodeID string) {
	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("approval_method", approval.ApprovalParallel).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(nodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set node approval method to parallel")
}

func (s *ScannerTestSuite) setNodeAdmins(nodeID string, adminIDs ...string) {
	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("admin_user_ids", adminIDs).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(nodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set node admin users")
}

func (s *ScannerTestSuite) taskByAssignee(instanceID, assigneeID string) approval.Task {
	var t approval.Task

	// Order by id (a sortable XID) so the lookup is deterministic even if a
	// scenario ever produces more than one task for the assignee.
	s.Require().NoError(
		s.db.NewSelect().Model(&t).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instanceID).Equals("assignee_id", assigneeID)
			}).
			OrderBy("id").
			Limit(1).
			Scan(s.ctx),
		"Should load task for assignee "+assigneeID,
	)

	return t
}

// TestTransferAdminTimeoutShouldPreserveBeforeChildParentLink pins the scanner
// half of the transfer fix: auto-transferring a timed-out "before" add-assignee
// child must carry its parent link and sort order onto the admin replacement,
// or the suspended parent could never be reactivated (the parallel deadlock).
func (s *ScannerTestSuite) TestTransferAdminTimeoutShouldPreserveBeforeChildParentLink() {
	// approver-1 stands in as the suspended parent (Waiting, so it is not scanned).
	_, parent := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionTransferAdmin, approval.TaskWaiting)
	s.makeNodeParallel(parent.NodeID)
	s.setNodeAdmins(parent.NodeID, "admin-1")

	before := approval.AddAssigneeBefore
	parentID := parent.ID
	pastDeadline := timex.Now().AddHours(-1)
	child := &approval.Task{
		TenantID:        "default",
		InstanceID:      parent.InstanceID,
		NodeID:          parent.NodeID,
		AssigneeID:      "before-B",
		SortOrder:       7,
		Status:          approval.TaskPending,
		Deadline:        &pastDeadline,
		ParentTaskID:    &parentID,
		AddAssigneeType: &before,
	}
	_, err := s.db.NewInsert().Model(child).Exec(s.ctx)
	s.Require().NoError(err, "Should insert timed-out before-child")

	s.scanner.ScanTimeouts(s.ctx)

	admin := s.taskByAssignee(parent.InstanceID, "admin-1")
	s.Require().NotNil(admin.ParentTaskID, "Admin replacement must inherit the before-child's parent link")
	s.Assert().Equal(parent.ID, *admin.ParentTaskID, "Admin replacement must point at the same suspended parent")
	s.Require().NotNil(admin.AddAssigneeType, "Admin replacement must inherit the add-assignee type")
	s.Assert().Equal(approval.AddAssigneeBefore, *admin.AddAssigneeType, "Admin replacement must remain a before-child")
	s.Assert().Equal(7, admin.SortOrder, "Admin replacement must inherit the sort order, not reset it to 0")
}

// TestTransferAdminTimeoutShouldRepointAfterChildrenToStandIn pins the
// symmetric scanner case: when the parent of "after" children is auto-
// transferred, those children must be re-pointed onto the admin stand-in so
// they are not orphaned.
func (s *ScannerTestSuite) TestTransferAdminTimeoutShouldRepointAfterChildrenToStandIn() {
	_, parent := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionTransferAdmin, approval.TaskPending)
	s.makeNodeParallel(parent.NodeID)
	s.setNodeAdmins(parent.NodeID, "admin-1")

	after := approval.AddAssigneeAfter
	parentID := parent.ID
	child := &approval.Task{
		TenantID:        "default",
		InstanceID:      parent.InstanceID,
		NodeID:          parent.NodeID,
		AssigneeID:      "after-B",
		SortOrder:       7,
		Status:          approval.TaskWaiting,
		ParentTaskID:    &parentID,
		AddAssigneeType: &after,
	}
	_, err := s.db.NewInsert().Model(child).Exec(s.ctx)
	s.Require().NoError(err, "Should insert queued after-child")

	s.scanner.ScanTimeouts(s.ctx)

	admin := s.taskByAssignee(parent.InstanceID, "admin-1")

	var reloadedChild approval.Task

	reloadedChild.ID = child.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedChild).WherePK().Scan(s.ctx), "Should reload after-child")
	s.Require().NotNil(reloadedChild.ParentTaskID, "After-child must keep a parent link")
	s.Assert().Equal(admin.ID, *reloadedChild.ParentTaskID, "After-child must be re-pointed onto the admin stand-in, not orphaned on the transferred parent")
}

// TestAutoPassTimeoutShouldActivateAfterChildOnParallelNode covers the scanner's
// parallel dependent-activation path: an auto-pass timeout on a parallel node
// must activate the queued after-child via ActivateDependentTasks.
func (s *ScannerTestSuite) TestAutoPassTimeoutShouldActivateAfterChildOnParallelNode() {
	// PassAll so the node stays running after the auto-pass (the parent approves
	// but the now-activated after-child has not), making the activation observable.
	_, parent := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionAutoPass, approval.TaskPending)
	s.makeNodeParallel(parent.NodeID)

	after := approval.AddAssigneeAfter
	parentID := parent.ID
	child := &approval.Task{
		TenantID:        "default",
		InstanceID:      parent.InstanceID,
		NodeID:          parent.NodeID,
		AssigneeID:      "after-B",
		SortOrder:       7,
		Status:          approval.TaskWaiting,
		ParentTaskID:    &parentID,
		AddAssigneeType: &after,
	}
	_, err := s.db.NewInsert().Model(child).Exec(s.ctx)
	s.Require().NoError(err, "Should insert queued after-child")

	s.scanner.ScanTimeouts(s.ctx)

	var reloadedChild approval.Task

	reloadedChild.ID = child.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedChild).WherePK().Scan(s.ctx), "Should reload after-child")
	s.Assert().Equal(approval.TaskPending, reloadedChild.Status,
		"Auto-pass timeout on a parallel node must activate the queued after-child")

	var inst approval.Instance

	inst.ID = parent.InstanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx), "Should reload instance")
	s.Assert().Equal(approval.InstanceRunning, inst.Status, "PassAll node stays running until the activated after-child also acts")
}

// TestTransferAdminTimeoutRepointsAfterChildToFirstOfMultipleAdmins covers the
// multi-admin re-point branch: when a parent of an "after" child times out and is
// transferred to several admins, the after-child is re-pointed onto the FIRST
// admin stand-in (not orphaned on the transferred parent).
func (s *ScannerTestSuite) TestTransferAdminTimeoutRepointsAfterChildToFirstOfMultipleAdmins() {
	_, parent := s.createTimeoutScenarioWithTaskStatus(approval.TimeoutActionTransferAdmin, approval.TaskPending)
	s.makeNodeParallel(parent.NodeID)
	s.setNodeAdmins(parent.NodeID, "admin-1", "admin-2")

	after := approval.AddAssigneeAfter
	parentID := parent.ID
	child := &approval.Task{
		TenantID:        "default",
		InstanceID:      parent.InstanceID,
		NodeID:          parent.NodeID,
		AssigneeID:      "after-B",
		SortOrder:       7,
		Status:          approval.TaskWaiting,
		ParentTaskID:    &parentID,
		AddAssigneeType: &after,
	}
	_, err := s.db.NewInsert().Model(child).Exec(s.ctx)
	s.Require().NoError(err, "Should insert queued after-child")

	s.scanner.ScanTimeouts(s.ctx)

	// Both admins get a distinct replacement; the after-child re-points onto the
	// first one created (admin-1, first in AdminUserIDs order).
	firstAdmin := s.taskByAssignee(parent.InstanceID, "admin-1")
	secondAdmin := s.taskByAssignee(parent.InstanceID, "admin-2")
	s.Require().NotEqual(firstAdmin.ID, secondAdmin.ID, "Both admins should get a distinct replacement task")

	var reloadedChild approval.Task

	reloadedChild.ID = child.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedChild).WherePK().Scan(s.ctx), "Should reload after-child")
	s.Require().NotNil(reloadedChild.ParentTaskID, "After-child must not be orphaned")
	s.Assert().Equal(firstAdmin.ID, *reloadedChild.ParentTaskID, "After-child must re-point onto the first admin stand-in")
}
