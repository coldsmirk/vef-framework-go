package command_test

import (
	"context"
	"errors"
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
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ResubmitTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// ResubmitTestSuite tests the ResubmitHandler.
type ResubmitTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.ResubmitHandler
	fixture *FlowFixture
}

func (s *ResubmitTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)
	s.handler = command.NewResubmitHandler(
		s.db,
		buildTestEngine(),
		service.NewValidationService(nil),
		dispatcher.NewEventPublisher(),
	)
}

func (s *ResubmitTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *ResubmitTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *ResubmitTestSuite) TestResubmitClearsFinishedAt() {
	startNodeID := s.fixture.NodeIDs["start-1"]
	s.Require().NotEmpty(startNodeID, "Should have start node")

	finishedAt := timex.Now().AddHours(-2)
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Returned Instance",
		InstanceNo:    "RESUBMIT-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceReturned,
		CurrentNodeID: &startNodeID,
		FinishedAt:    &finishedAt,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	_, err = s.handler.Handle(s.ctx, command.ResubmitCmd{
		InstanceID: instance.ID,
		Operator:   approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"},
	})
	s.Require().NoError(err, "Should resubmit returned instance")

	var updated approval.Instance

	updated.ID = instance.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should not return error")

	s.Assert().Equal(approval.InstanceRunning, updated.Status, "Instance should be running after resubmit")
	s.Assert().Nil(updated.FinishedAt, "Resubmitted running instance should clear finished_at")
}

func (s *ResubmitTestSuite) TestResubmitShouldBeConcurrencySafe() {
	skipSQLiteConcurrencyTest(s.T(), s.ctx, s.db, "SQLite returns SQLITE_BUSY under write races in this concurrency scenario")

	startNodeID := s.fixture.NodeIDs["start-1"]
	s.Require().NotEmpty(startNodeID, "Should have start node")

	finishedAt := timex.Now().AddHours(-2)
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Concurrent Returned Instance",
		InstanceNo:    "RESUBMIT-002",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceReturned,
		CurrentNodeID: &startNodeID,
		FinishedAt:    &finishedAt,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert returned instance for concurrent resubmit")

	lockReady, releaseLock, lockDone := holdSharedTableLock(s.ctx, s.db, "apv_instance")

	<-lockReady

	start := make(chan struct{})
	errCh := make(chan error, 2)

	var wg sync.WaitGroup

	runOne := func() {
		<-start

		err := s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := s.handler.Handle(txCtx, command.ResubmitCmd{
				InstanceID: instance.ID,
				Operator:   approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"},
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

		if errors.Is(err, shared.ErrResubmitNotAllowed) {
			notAllowedCount++
		}
	}

	s.Assert().Equal(1, successCount, "Concurrent resubmit should allow only one successful operation")
	s.Assert().Equal(1, notAllowedCount, "Concurrent resubmit should reject stale operation by resubmit state transition")

	var tasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instance.ID)
			}).
			Scan(s.ctx),
		"Should query tasks created by concurrent resubmit",
	)
	s.Assert().Len(tasks, 2, "Concurrent resubmit should create only one batch of approval tasks")
}

func (s *ResubmitTestSuite) TestResubmitShouldRejectInvalidFormDataBySchema() {
	setPublishedFormSchema(s.T(), s.ctx, s.db, s.fixture.VersionID, &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "amount", Kind: approval.FieldNumber, Label: "Amount", IsRequired: true},
		},
	})

	startNodeID := s.fixture.NodeIDs["start-1"]
	s.Require().NotEmpty(startNodeID, "Should have start node")

	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Returned Instance",
		InstanceNo:    "RESUBMIT-003",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceReturned,
		CurrentNodeID: &startNodeID,
		FormData:      map[string]any{"amount": 100},
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert returned instance")

	_, err = s.handler.Handle(s.ctx, command.ResubmitCmd{
		InstanceID: instance.ID,
		Operator:   approval.OperatorInfo{ID: "applicant-1", Name: "Applicant"},
		FormData:   map[string]any{"amount": "invalid"},
	})
	s.Require().Error(err, "Should reject invalid resubmit form data")

	var re result.Error
	s.Require().ErrorAs(err, &re, "Should return business error")
	s.Assert().Equal(shared.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
}
