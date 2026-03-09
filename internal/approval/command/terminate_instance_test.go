package command_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
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
	handler     *command.TerminateInstanceHandler
	fixture     *MinimalFixture
	nodeID      string
	instanceSeq int
}

func (s *TerminateInstanceTestSuite) SetupSuite() {
	s.handler = command.NewTerminateInstanceHandler(s.db, service.NewTaskService(), dispatcher.NewEventPublisher())
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "terminate")

	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "term-node",
		Kind:          approval.NodeApproval,
		Name:          "Terminate Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
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
	s.Require().NoError(err, "Should not return error")

	return inst
}

func (s *TerminateInstanceTestSuite) insertTask(instanceID, assigneeID string, status approval.TaskStatus) {
	task := &approval.Task{
		TenantID:   "default",
		InstanceID: instanceID,
		NodeID:     s.nodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     status,
	}
	_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
}

func (s *TerminateInstanceTestSuite) TestTerminateSuccess() {
	inst := s.insertInstance(approval.InstanceRunning)
	s.insertTask(inst.ID, "approver-1", approval.TaskPending)
	s.insertTask(inst.ID, "approver-2", approval.TaskWaiting)

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: inst.ID,
		Operator:   operator,
		Reason:     "违规终止",
	})
	s.Require().NoError(err, "Should terminate instance without error")

	// Verify instance status
	var updated approval.Instance

	updated.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.InstanceTerminated, updated.Status, "Should set status to terminated")
	s.Assert().NotNil(updated.FinishedAt, "Should set finished_at")

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
	s.Assert().Equal(approval.ActionTerminate, logs[0].Action, "Action should be terminate")
	s.Assert().Equal("违规终止", *logs[0].Opinion, "Should record reason in opinion")
}

func (s *TerminateInstanceTestSuite) TestTerminateInstanceNotFound() {
	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: "non-existent",
		Operator:   operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceNotFound, "Should return ErrInstanceNotFound")
}

func (s *TerminateInstanceTestSuite) TestTerminateAlreadyCompleted() {
	inst := s.insertInstance(approval.InstanceApproved)

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceNotRunning, "Should not allow terminating approved instance")
}

func (s *TerminateInstanceTestSuite) TestTerminateAlreadyTerminated() {
	inst := s.insertInstance(approval.InstanceTerminated)

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: inst.ID,
		Operator:   operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceNotRunning, "Should not allow terminating already terminated instance")
}
