package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &RejectTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// RejectTaskTestSuite tests the RejectTaskHandler.
type RejectTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.RejectTaskHandler
	fixture *FlowFixture
}

func (s *RejectTaskTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine()
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	pub := dispatcher.NewEventPublisher()

	s.handler = command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc, pub)
}

func (s *RejectTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *RejectTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *RejectTaskTestSuite) newRunningInstance(assigneeID string) (*approval.Instance, *approval.Task) {
	return setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, assigneeID)
}

func (s *RejectTaskTestSuite) TestRejectSuccess() {
	inst, task := s.newRunningInstance("rejector-1")

	operator := approval.OperatorInfo{ID: "rejector-1", Name: "Rejector"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "Not acceptable",
	})
	s.Require().NoError(err, "Should reject task without error")

	// Verify task status
	var updated approval.Task

	updated.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.TaskRejected, updated.Status, "Task should be rejected")

	// Verify instance status (with PassAll rule, one rejection rejects instance)
	var updatedInst approval.Instance

	updatedInst.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&updatedInst).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.InstanceRejected, updatedInst.Status, "Instance should be rejected")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "Should not return error")

	found := false
	for _, log := range logs {
		if log.Action == approval.ActionReject {
			found = true

			s.Assert().Equal("rejector-1", log.OperatorID, "Should match expected value")
		}
	}

	s.Assert().True(found, "Should have a reject action log")
}

func (s *RejectTaskTestSuite) TestRejectTaskNotFound() {
	operator := approval.OperatorInfo{ID: "rejector-1", Name: "Rejector"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   "non-existent",
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return expected error")
}

func (s *RejectTaskTestSuite) TestRejectNotAssignee() {
	_, task := s.newRunningInstance("rejector-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should return expected error")
}

func (s *RejectTaskTestSuite) TestRejectTaskNotCurrentNode() {
	inst, task := s.newRunningInstance("rejector-current")

	otherNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "reject-other-current-node",
		Kind:          approval.NodeApproval,
		Name:          "Reject Other Current Node",
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create another node")

	_, err = s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", otherNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should move instance current node away from task node")

	operator := approval.OperatorInfo{ID: "rejector-current", Name: "Rejector"}
	_, err = s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "rejected",
	})
	s.Require().Error(err, "Should fail when rejecting a task not in current node")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should return task not pending for stale node task")
}
