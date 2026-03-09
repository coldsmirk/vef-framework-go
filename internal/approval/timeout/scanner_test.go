package timeout_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/approval/timeout"
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
	publisher := dispatcher.NewEventPublisher()
	eng := engine.NewFlowEngine(registry, []engine.NodeProcessor{
		engine.NewStartProcessor(),
		engine.NewEndProcessor(),
		engine.NewConditionProcessor(),
		engine.NewApprovalProcessor(nil),
		engine.NewHandleProcessor(nil),
		engine.NewCCProcessor(),
	}, publisher, nil)
	taskSvc := service.NewTaskService()
	nodeSvc := service.NewNodeService(eng, publisher, taskSvc, nil)

	s.scanner = timeout.NewScanner(s.db, publisher, taskSvc, nodeSvc, nil)
}

func (s *ScannerTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.EventOutbox)(nil),
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

	transferredEventCount, err := s.db.NewSelect().
		Model((*approval.EventOutbox)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.task.transferred") }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count transferred events after timeout transfer")
	s.Assert().Equal(int64(2), transferredEventCount, "Should emit one transfer event per created admin task")

	createdEventCount, err := s.db.NewSelect().
		Model((*approval.EventOutbox)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.task.created") }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count created events after timeout transfer")
	s.Assert().Equal(int64(2), createdEventCount, "Should emit one task-created event per admin task")

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

	startedAt := timex.Now()

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
	s.Assert().True(
		adminTasks[0].Deadline.Unwrap().After(startedAt.AddHours(3).Unwrap()),
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
		lockDone <- s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
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
		err := s.db.RunInTX(lockCtx, func(ctx context.Context, tx orm.DB) error {
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

	warningCount, err := s.db.NewSelect().
		Model((*approval.EventOutbox)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.task.deadline_warning") }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count warning events after pre-warning scan")
	s.Assert().Equal(int64(1), warningCount, "Pending task in warning window should emit one warning event")
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

	outboxCount, err := s.db.NewSelect().Model((*approval.EventOutbox)(nil)).Count(s.ctx)
	s.Require().NoError(err, "Should count outbox events after timeout scan")
	s.Assert().Equal(int64(0), outboxCount, "Ignoring waiting task should not publish timeout events")
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

	warningCount, err := s.db.NewSelect().
		Model((*approval.EventOutbox)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.task.deadline_warning") }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count warning events after double scan")
	s.Assert().Equal(int64(1), warningCount, "Idempotent pre-warning should emit only one event across two scans")
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

	warningCount, err := s.db.NewSelect().
		Model((*approval.EventOutbox)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.task.deadline_warning") }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count warning events after pre-warning scan")
	s.Assert().Equal(int64(0), warningCount, "Ignoring waiting task should not emit warning events")
}
