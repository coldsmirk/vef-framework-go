package command_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &WithdrawTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// WithdrawTestSuite tests the WithdrawHandler.
type WithdrawTestSuite struct {
	suite.Suite

	ctx         context.Context
	db          orm.DB
	handler     *command.WithdrawHandler
	fixture     *MinimalFixture
	nodeID      string
	instanceSeq int
}

func (s *WithdrawTestSuite) SetupSuite() {
	s.handler = command.NewWithdrawHandler(s.db, service.NewTaskService(), dispatcher.NewEventPublisher())
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "withdraw")

	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "wd-node",
		Kind:          approval.NodeApproval,
		Name:          "Withdraw Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.nodeID = node.ID
}

func (s *WithdrawTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *WithdrawTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *WithdrawTestSuite) insertInstance(applicantID string, status approval.InstanceStatus) *approval.Instance {
	s.instanceSeq++
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Withdraw Test",
		InstanceNo:    fmt.Sprintf("WD-%03d", s.instanceSeq),
		ApplicantID:   applicantID,
		Status:        status,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	return inst
}

func (s *WithdrawTestSuite) insertTask(instanceID string, status approval.TaskStatus) {
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instanceID,
		NodeID:     s.nodeID,
		AssigneeID: "approver-1",
		SortOrder:  1,
		Status:     status,
	}
	_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
}

func (s *WithdrawTestSuite) TestWithdrawSuccess() {
	inst := s.insertInstance("applicant-1", approval.InstanceRunning)
	s.insertTask(inst.ID, approval.TaskPending)

	operator := approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().NoError(err, "Should withdraw instance without error")

	// Verify instance status
	var updated approval.Instance

	updated.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.InstanceWithdrawn, updated.Status, "Should set status to withdrawn")

	// Verify tasks canceled
	var tasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "Should not return error")

	for _, t := range tasks {
		s.Assert().Equal(approval.TaskCanceled, t.Status, "All tasks should be canceled")
	}

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "Should not return error")
	s.Assert().Len(logs, 1, "Should insert one action log")
	s.Assert().Equal(approval.ActionWithdraw, logs[0].Action, "Action should be withdraw")
}

func (s *WithdrawTestSuite) TestWithdrawNotApplicant() {
	inst := s.insertInstance("applicant-1", approval.InstanceRunning)

	operator := approval.OperatorInfo{ID: "other-user", Name: "Other"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrNotApplicant, "Should return ErrNotApplicant")
}

func (s *WithdrawTestSuite) TestWithdrawNotAllowed() {
	inst := s.insertInstance("applicant-1", approval.InstanceApproved)

	operator := approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrWithdrawNotAllowed, "Should not allow withdrawal of approved instance")
}

func (s *WithdrawTestSuite) TestWithdrawInstanceNotFound() {
	operator := approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"}
	_, err := s.handler.Handle(s.ctx, command.WithdrawCmd{
		InstanceID: "non-existent",
		Operator:   operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceNotFound, "Should return ErrInstanceNotFound")
}

func (s *WithdrawTestSuite) TestWithdrawShouldBeConcurrencySafe() {
	skipSQLiteConcurrencyTest(s.T(), s.ctx, s.db, "SQLite returns SQLITE_BUSY under write races in this concurrency scenario")

	inst := s.insertInstance("applicant-1", approval.InstanceRunning)
	s.insertTask(inst.ID, approval.TaskPending)

	lockReady, releaseLock, lockDone := holdSharedTableLock(s.ctx, s.db, "apv_instance")

	<-lockReady

	operator := approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"}
	start := make(chan struct{})
	errCh := make(chan error, 2)

	var wg sync.WaitGroup

	runOne := func() {
		<-start

		err := s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := s.handler.Handle(txCtx, command.WithdrawCmd{
				InstanceID: inst.ID,
				Operator:   operator,
			})

			return err
		})
		errCh <- err
	}

	wg.Go(runOne)
	wg.Go(runOne)
	close(start)

	time.Sleep(200 * time.Millisecond)
	close(releaseLock)

	s.Require().NoError(<-lockDone, "Table lock transaction should complete without error")
	wg.Wait()
	close(errCh)

	successCount := 0

	notAllowedCount := 0
	for err := range errCh {
		if err == nil {
			successCount++

			continue
		}

		if errors.Is(err, shared.ErrWithdrawNotAllowed) {
			notAllowedCount++
		}
	}

	s.Assert().Equal(1, successCount, "Concurrent withdraw should allow only one successful operation")
	s.Assert().Equal(1, notAllowedCount, "Concurrent withdraw should reject stale operation by withdraw state transition")

	var actionLogs []approval.ActionLog
	s.Require().NoError(
		s.db.NewSelect().
			Model(&actionLogs).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", inst.ID).
					Equals("action", approval.ActionWithdraw)
			}).
			Scan(s.ctx),
		"Should query withdraw action logs",
	)
	s.Assert().Len(actionLogs, 1, "Concurrent withdraw should insert only one withdraw action log")
}
