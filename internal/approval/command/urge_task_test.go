package command_test

import (
	"context"
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
	"github.com/coldsmirk/vef-framework-go/result"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &UrgeTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// UrgeTaskTestSuite tests the UrgeTaskHandler.
type UrgeTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.UrgeTaskHandler
	fixture *MinimalFixture
	nodeID  string
	instID  string
}

func (s *UrgeTaskTestSuite) SetupSuite() {
	s.handler = command.NewUrgeTaskHandler(s.db, service.NewTaskService(), dispatcher.NewEventPublisher(), nil)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "urge")

	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "urge-node",
		Kind:          approval.NodeApproval,
		Name:          "Urge Test Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.nodeID = node.ID

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Urge Test",
		InstanceNo:    "URG-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.instID = inst.ID
}

func (s *UrgeTaskTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.EventOutbox)(nil),
		(*approval.UrgeRecord)(nil),
		(*approval.Task)(nil),
	)
}

func (s *UrgeTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *UrgeTaskTestSuite) insertTask(assigneeID string) *approval.Task {
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: s.instID,
		NodeID:     s.nodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	return task
}

func (s *UrgeTaskTestSuite) TestUrgeSuccess() {
	task := s.insertTask("assignee-1")

	_, err := s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  task.ID,
		UrgerID: "applicant-1",
		Message: "Please review ASAP",
	})
	s.Require().NoError(err, "Should urge task without error")

	// Verify urge record created
	var records []approval.UrgeRecord
	s.Require().NoError(s.db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("task_id", task.ID) }).
		Scan(s.ctx), "Should not return error")
	s.Assert().Len(records, 1, "Should create one urge record")

	// Verify event published
	var events []approval.EventOutbox
	s.Require().NoError(s.db.NewSelect().Model(&events).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.task.urged") }).
		Scan(s.ctx), "Should not return error")
	s.Assert().Len(events, 1, "Should publish one urge event")
}

func (s *UrgeTaskTestSuite) TestUrgeCooldown() {
	task := s.insertTask("assignee-2")

	// First urge
	_, err := s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  task.ID,
		UrgerID: "applicant-1",
	})
	s.Require().NoError(err, "First urge should succeed")

	// Immediate second urge - should fail due to cooldown
	_, err = s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  task.ID,
		UrgerID: "applicant-1",
	})
	s.Require().Error(err, "Second urge should fail")

	var re result.Error
	s.Require().ErrorAs(err, &re, "Should be a result.Error")
	s.Assert().Equal(shared.ErrCodeUrgeCooldown, re.Code, "Should return ErrCodeUrgeCooldown")
}

func (s *UrgeTaskTestSuite) TestUrgeTaskNotFound() {
	_, err := s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  "non-existent",
		UrgerID: "applicant-1",
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return ErrTaskNotFound")
}

func (s *UrgeTaskTestSuite) TestUrgeShouldDenyNonParticipant() {
	task := s.insertTask("assignee-outsider")

	_, err := s.handler.Handle(s.ctx, command.UrgeTaskCmd{
		TaskID:  task.ID,
		UrgerID: "outsider-1",
	})
	s.Require().Error(err, "Should reject non-participant urge request")
	s.Assert().ErrorIs(err, shared.ErrAccessDenied, "Should return access denied for non-participant")
}

func (s *UrgeTaskTestSuite) TestUrgeCooldownShouldBeConcurrencySafe() {
	skipSQLiteConcurrencyTest(s.T(), s.ctx, s.db, "SQLite returns SQLITE_BUSY under write races in this concurrency scenario")

	task := s.insertTask("assignee-concurrency")

	lockReady, releaseLock, lockDone := holdSharedTableLock(s.ctx, s.db, "apv_urge_record")

	<-lockReady

	start := make(chan struct{})

	var wg sync.WaitGroup

	errCh := make(chan error, 2)

	runOne := func() {
		<-start

		err := s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := s.handler.Handle(txCtx, command.UrgeTaskCmd{
				TaskID:  task.ID,
				UrgerID: "applicant-1",
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

	cooldownCount := 0
	for err := range errCh {
		if err == nil {
			successCount++

			continue
		}

		var re result.Error
		if s.ErrorAs(err, &re, "Failed urge should return result.Error") {
			if re.Code == shared.ErrCodeUrgeCooldown {
				cooldownCount++
			}
		}
	}

	s.Assert().Equal(1, successCount, "Concurrent urge operations should allow only one successful request")
	s.Assert().Equal(1, cooldownCount, "Concurrent urge operations should reject one request by cooldown")

	var records []approval.UrgeRecord
	s.Require().NoError(
		s.db.NewSelect().
			Model(&records).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("task_id", task.ID).
					Equals("urger_id", "applicant-1")
			}).
			Scan(s.ctx),
		"Should query urge records for concurrent requests",
	)
	s.Assert().Len(records, 1, "Concurrent urge operations should persist only one urge record")
}
