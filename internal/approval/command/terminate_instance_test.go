package command_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &TerminateInstanceTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// TerminateInstanceTestSuite tests the TerminateInstanceHandler.
type TerminateInstanceTestSuite struct {
	suite.Suite

	ctx         context.Context
	db          orm.DB
	handler     cqrs.Handler[command.TerminateInstanceCmd, cqrs.Unit]
	fixture     *MinimalFixture
	nodeID      string
	instanceSeq int
}

func (s *TerminateInstanceTestSuite) SetupSuite() {
	s.handler = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewTerminateInstanceHandler(s.db, service.NewTaskService(), service.NewInstanceService(nil)))
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "terminate")

	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "term-node",
		Kind:          approval.NodeApproval,
		Name:          "Terminate Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Terminate instance should complete without error")
	s.nodeID = node.ID
}

func (s *TerminateInstanceTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *TerminateInstanceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *TerminateInstanceTestSuite) insertInstance(status approval.InstanceStatus) *approval.Instance {
	s.instanceSeq++
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Terminate Test",
		InstanceNo:    fmt.Sprintf("TERM-%03d", s.instanceSeq),
		ApplicantID:   "applicant-1",
		Status:        status,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Terminate instance should complete without error")

	return inst
}

func (s *TerminateInstanceTestSuite) insertTask(instanceID, assigneeID string, status approval.TaskStatus) {
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instanceID,
		NodeID:     s.nodeID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", instanceID, s.nodeID).ID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     status,
	}
	_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Terminate instance should complete without error")
}

func (s *TerminateInstanceTestSuite) TestTerminateSuccess() {
	inst := s.insertInstance(approval.InstanceRunning)
	s.insertTask(inst.ID, "approver-1", approval.TaskPending)
	s.insertTask(inst.ID, "approver-2", approval.TaskWaiting)

	operator := approval.UserInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: inst.ID,
		Operator:   operator,
		Reason:     "违规终止",
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "Should terminate instance without error")

	// Verify instance status
	var updated approval.Instance

	updated.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "TestTerminateSuccess should complete without error")
	s.Assert().Equal(approval.InstanceTerminated, updated.Status, "Should set status to terminated")
	s.Assert().NotNil(updated.FinishedAt, "Should set finished_at")

	// Verify tasks canceled
	var tasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "TestTerminateSuccess should complete without error")

	for _, t := range tasks {
		s.Assert().Equal(approval.TaskCanceled, t.Status, "All tasks should be canceled")
	}

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "TestTerminateSuccess should complete without error")
	s.Assert().Len(logs, 1, "Should insert one action log")
	s.Assert().Equal(approval.ActionTerminate, logs[0].Action, "Action should be terminate")
	s.Assert().Equal("违规终止", *logs[0].Opinion, "Should record reason in opinion")
}

func (s *TerminateInstanceTestSuite) TestTerminateInstanceNotFound() {
	operator := approval.UserInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: "non-existent",
		Operator:   operator,
		Caller:     approval.SystemCaller,
	})
	s.Require().Error(err, "TestTerminateInstanceNotFound should return an error")
	s.Assert().ErrorIs(err, shared.ErrInstanceNotFound, "Should return ErrInstanceNotFound")
}

func (s *TerminateInstanceTestSuite) TestTerminatePausedInstances() {
	// Returned and withdrawn instances are paused, not final — an admin must
	// be able to close them, otherwise an abandoned instance lingers forever.
	for _, status := range []approval.InstanceStatus{approval.InstanceReturned, approval.InstanceWithdrawn} {
		s.Run(string(status), func() {
			inst := s.insertInstance(status)

			operator := approval.UserInfo{ID: "admin-1", Name: "Admin"}
			_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
				InstanceID: inst.ID,
				Operator:   operator,
				Reason:     "清理滞留实例",
				Caller:     approval.SystemCaller,
			})
			s.Require().NoError(err, "Should terminate a paused instance")

			var updated approval.Instance

			updated.ID = inst.ID
			s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should reload instance")
			s.Assert().Equal(approval.InstanceTerminated, updated.Status, "Paused instance should close as terminated")
			s.Assert().NotNil(updated.FinishedAt, "Should set finished_at")
		})
	}
}

func (s *TerminateInstanceTestSuite) TestTerminateAlreadyCompleted() {
	inst := s.insertInstance(approval.InstanceApproved)

	operator := approval.UserInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: inst.ID,
		Operator:   operator,
		Caller:     approval.SystemCaller,
	})
	s.Require().Error(err, "TestTerminateAlreadyCompleted should return an error")
	s.Assert().ErrorIs(err, shared.ErrTerminateNotAllowed, "Should not allow terminating approved instance")
}

func (s *TerminateInstanceTestSuite) TestSynchronousProjectionFailureRollsBackFinalStatus() {
	_, err := s.db.NewRaw(`CREATE TABLE biz_terminate_projection (
		id VARCHAR(64) PRIMARY KEY,
		approval_status VARCHAR(32),
		apv_instance_id VARCHAR(32)
	)`).Exec(s.ctx)
	s.Require().NoError(err, "Should create the termination business table")

	_, err = s.db.NewRaw(`INSERT INTO biz_terminate_projection (id, approval_status)
		VALUES ('term-order-1', 'submitted')`).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the termination business row")

	ref := "term-order-1"
	instance := s.insertInstance(approval.InstanceRunning)
	instance.BusinessRef = &ref
	instanceIDColumn := "apv_instance_id"
	businessBinding := &approval.BusinessBindingConfig{
		TableName:        "biz_terminate_projection",
		KeyColumns:       []string{"id"},
		StatusColumn:     "approval_status",
		InstanceIDColumn: &instanceIDColumn,
	}
	flow := &approval.Flow{BindingMode: approval.BindingBusiness, BusinessBinding: businessBinding}
	flow.ID = s.fixture.FlowID
	version := &approval.FlowVersion{BusinessBinding: businessBinding}
	version.ID = s.fixture.VersionID

	projector := binding.NewProjector(binding.NewIdentityResolver(), binding.NewWriter(), nil)
	s.Require().NoError(projector.Bind(s.ctx, s.db, flow, version, instance),
		"Initial synchronous projection should claim the business row")

	_, err = s.db.NewRaw(`DROP TABLE biz_terminate_projection`).Exec(s.ctx)
	s.Require().NoError(err, "Test setup should make the final business write fail")

	hooks := engine.NewLifecycleHookRunner(projector, nil)
	handler := command.NewTerminateInstanceHandler(
		s.db,
		service.NewTaskService(),
		service.NewInstanceService(hooks),
	)
	err = s.db.RunInTx(s.ctx, func(txCtx context.Context, tx orm.DB) error {
		_, handleErr := handler.Handle(contextx.SetDB(txCtx, tx), command.TerminateInstanceCmd{
			InstanceID: instance.ID,
			Operator:   approval.UserInfo{ID: "admin-1", Name: "Admin"},
			Reason:     "business write should fail",
			Caller:     approval.SystemCaller,
		})

		return handleErr
	})
	s.Require().Error(err, "Synchronous business write failure should abort the final approval action")

	reloaded := new(approval.Instance)
	reloaded.ID = instance.ID
	s.Require().NoError(s.db.NewSelect().Model(reloaded).WherePK().Scan(s.ctx),
		"Should reload the instance after the failed final transition")
	s.Assert().Equal(approval.InstanceRunning, reloaded.Status,
		"Failed synchronous projection must roll the approval status back")
	s.Assert().Nil(reloaded.FinishedAt,
		"Failed synchronous projection must roll the approval finish time back")

	projection := new(approval.BusinessProjection)
	projection.ID = *instance.BusinessProjectionID
	s.Require().NoError(s.db.NewSelect().Model(projection).WherePK().Scan(s.ctx),
		"Should reload the projection after the failed final transition")
	s.Assert().Equal(approval.InstanceRunning, projection.DesiredStatus,
		"Failed synchronous projection must roll its desired state back")
	s.Assert().Equal(projection.AppliedRevision, projection.DesiredRevision,
		"Projection should remain converged at the last committed revision")
}

func (s *TerminateInstanceTestSuite) TestTerminateAlreadyTerminated() {
	inst := s.insertInstance(approval.InstanceTerminated)

	operator := approval.UserInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: inst.ID,
		Operator:   operator,
		Caller:     approval.SystemCaller,
	})
	s.Require().Error(err, "TestTerminateAlreadyTerminated should return an error")
	s.Assert().ErrorIs(err, shared.ErrTerminateNotAllowed, "Should not allow terminating already terminated instance")
}
