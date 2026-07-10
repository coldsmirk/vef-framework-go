package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &RetryBusinessProjectionTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

type RetryBusinessProjectionTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.RetryBusinessProjectionHandler
}

func (s *RetryBusinessProjectionTestSuite) SetupSuite() {
	cfg := &config.ApprovalConfig{BusinessBinding: config.ApprovalBusinessBindingConfig{
		Consistency: config.ApprovalBindingEventual,
	}}
	worker := binding.NewWorker(s.db, eventtest.NewFakeBus(), binding.NewWriter(), cfg)
	s.handler = command.NewRetryBusinessProjectionHandler(s.db, worker)

	_, err := s.db.NewRaw(`CREATE TABLE cmd_projection_order (
		id VARCHAR(64) PRIMARY KEY,
		approval_status VARCHAR(32),
		apv_instance_id VARCHAR(32)
	)`).Exec(s.ctx)
	s.Require().NoError(err, "Should create the projection retry business table")
}

func (s *RetryBusinessProjectionTestSuite) TearDownTest() {
	_, _ = s.db.NewDelete().Model((*approval.BusinessProjection)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
	_, _ = s.db.NewRaw(`DELETE FROM cmd_projection_order`).Exec(s.ctx)
}

func (s *RetryBusinessProjectionTestSuite) TearDownSuite() {
	_, _ = s.db.NewRaw(`DROP TABLE cmd_projection_order`).Exec(s.ctx)
}

func (s *RetryBusinessProjectionTestSuite) insertProjection(
	tenantID, targetHash, recordID string,
	consistency config.ApprovalBindingConsistency,
) *approval.BusinessProjection {
	s.T().Helper()

	instanceIDColumn := "apv_instance_id"
	now := timex.Now()
	projection := &approval.BusinessProjection{
		TenantID:        tenantID,
		FlowID:          "flow-1",
		FlowVersionID:   "version-1",
		OwnerInstanceID: "instance-1",
		TargetHash:      targetHash,
		Consistency:     consistency,
		Binding: &approval.BusinessBindingConfig{
			TableName:        "cmd_projection_order",
			KeyColumns:       []string{"id"},
			StatusColumn:     "approval_status",
			InstanceIDColumn: &instanceIDColumn,
		},
		RecordKey:         []byte(`[{"column":"id","kind":"string","value":"` + recordID + `"}]`),
		DesiredStatus:     approval.InstanceApproved,
		DesiredStartedAt:  now,
		DesiredFinishedAt: &now,
		DesiredRevision:   1,
		Status:            approval.BindingProjectionFailed,
	}
	_, err := s.db.NewInsert().Model(projection).Exec(s.ctx)
	s.Require().NoError(err, "Should insert projection retry fixture")

	return projection
}

func (s *RetryBusinessProjectionTestSuite) TestRetrySuccess() {
	_, err := s.db.NewRaw(`INSERT INTO cmd_projection_order (id, approval_status)
		VALUES ('order-1', 'submitted')`).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the retry target")
	projection := s.insertProjection("tenant-1", "retry-success", "order-1", config.ApprovalBindingEventual)

	_, err = s.handler.Handle(s.ctx, command.RetryBusinessProjectionCmd{
		ProjectionID: projection.ID,
		Caller:       approval.CallerContext{TenantID: "tenant-1"},
	})
	s.Require().NoError(err, "Immediate retry should apply an available business target")

	reloaded := new(approval.BusinessProjection)
	reloaded.ID = projection.ID
	s.Require().NoError(s.db.NewSelect().Model(reloaded).WherePK().Scan(s.ctx),
		"Should reload the retried projection")
	s.Assert().Equal(approval.BindingProjectionApplied, reloaded.Status,
		"Successful immediate retry should mark the projection applied")
	s.Assert().Equal(reloaded.DesiredRevision, reloaded.AppliedRevision,
		"Successful immediate retry should converge the desired revision")

	var status, owner string
	s.Require().NoError(s.db.NewRaw(`SELECT approval_status, apv_instance_id
		FROM cmd_projection_order WHERE id = 'order-1'`).Scan(s.ctx, &status, &owner),
		"Should read the immediately retried target")
	s.Assert().Equal("approved", status, "Immediate retry should project the desired status")
	s.Assert().Equal(projection.OwnerInstanceID, owner, "Immediate retry should stamp the projection owner")
}

func (s *RetryBusinessProjectionTestSuite) TestRetryFailureIsPersisted() {
	projection := s.insertProjection("tenant-1", "retry-failure", "missing-order", config.ApprovalBindingEventual)

	_, err := s.handler.Handle(s.ctx, command.RetryBusinessProjectionCmd{
		ProjectionID: projection.ID,
		Caller:       approval.CallerContext{TenantID: "tenant-1"},
	})
	s.Require().NoError(err,
		"Business write failure should commit failed projection state instead of rolling the retry command back")

	reloaded := new(approval.BusinessProjection)
	reloaded.ID = projection.ID
	s.Require().NoError(s.db.NewSelect().Model(reloaded).WherePK().Scan(s.ctx),
		"Should reload the failed immediate retry")
	s.Assert().Equal(approval.BindingProjectionFailed, reloaded.Status,
		"Failed immediate retry should remain scheduled for convergence")
	s.Assert().Equal(1, reloaded.AttemptCount, "Failed immediate retry should increment attempt count")
	s.Assert().NotNil(reloaded.NextAttemptAt, "Failed immediate retry should schedule its next attempt")
	s.Require().NotNil(reloaded.LastError, "Failed immediate retry should retain its error")
	s.Assert().NotEmpty(*reloaded.LastError, "Failed immediate retry should retain a non-empty error")
}

func (s *RetryBusinessProjectionTestSuite) TestRetrySQLFailureIsPersisted() {
	_, err := s.db.NewRaw(`INSERT INTO cmd_projection_order (id, approval_status)
		VALUES ('order-sql-error', 'submitted')`).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the retry target")

	projection := s.insertProjection(
		"tenant-1", "retry-sql-error", "order-sql-error", config.ApprovalBindingEventual,
	)
	projection.Binding.StatusColumn = "missing_approval_status"
	_, err = s.db.NewUpdate().Model(projection).Select("binding").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should persist the intentionally invalid runtime binding")

	_, err = s.handler.Handle(s.ctx, command.RetryBusinessProjectionCmd{
		ProjectionID: projection.ID,
		Caller:       approval.CallerContext{TenantID: "tenant-1"},
	})
	s.Require().NoError(err,
		"Host SQL failure should roll back its savepoint and commit failed projection state")

	reloaded := new(approval.BusinessProjection)
	reloaded.ID = projection.ID
	s.Require().NoError(s.db.NewSelect().Model(reloaded).WherePK().Scan(s.ctx),
		"Should reload the projection after its host SQL failure")
	s.Assert().Equal(approval.BindingProjectionFailed, reloaded.Status,
		"Host SQL failure should persist failed convergence state")
	s.Assert().Equal(1, reloaded.AttemptCount,
		"Host SQL failure should increment the durable attempt count")
	s.Require().NotNil(reloaded.LastError, "Host SQL failure should be retained for operators")
	s.Assert().NotEmpty(*reloaded.LastError, "Retained host SQL failure should not be blank")
}

func (s *RetryBusinessProjectionTestSuite) TestRetryNotFoundAndCrossTenantAreOpaque() {
	_, err := s.handler.Handle(s.ctx, command.RetryBusinessProjectionCmd{
		ProjectionID: "missing-projection",
		Caller:       approval.CallerContext{TenantID: "tenant-1"},
	})
	s.Require().ErrorIs(err, shared.ErrBindingProjectionNotFound,
		"Missing projection should return the public not-found sentinel")

	projection := s.insertProjection("tenant-2", "retry-cross-tenant", "order-2", config.ApprovalBindingEventual)
	_, err = s.handler.Handle(s.ctx, command.RetryBusinessProjectionCmd{
		ProjectionID: projection.ID,
		Caller:       approval.CallerContext{TenantID: "tenant-1"},
	})
	s.Require().ErrorIs(err, shared.ErrBindingProjectionNotFound,
		"Cross-tenant projection should be hidden behind the same not-found sentinel")
}

func (s *RetryBusinessProjectionTestSuite) TestRetryRejectsSynchronousProjection() {
	projection := s.insertProjection("tenant-1", "retry-synchronous", "order-3", config.ApprovalBindingSynchronous)

	_, err := s.handler.Handle(s.ctx, command.RetryBusinessProjectionCmd{
		ProjectionID: projection.ID,
		Caller:       approval.CallerContext{TenantID: "tenant-1"},
	})
	s.Require().ErrorIs(err, binding.ErrProjectionStateInvalid,
		"Synchronous projection must never enter the eventual retry path")
}
