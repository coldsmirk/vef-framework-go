package command_test

import (
	"context"
	"fmt"

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
		return &ReassignTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// ReassignTaskTestSuite tests the ReassignTaskHandler.
type ReassignTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.ReassignTaskHandler
	fixture *MinimalFixture
	nodeID  string
	taskSeq int
}

func (s *ReassignTaskTestSuite) SetupSuite() {
	s.handler = command.NewReassignTaskHandler(s.db, dispatcher.NewEventPublisher(), nil)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "reassign")

	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "reassign-node",
		Kind:          approval.NodeApproval,
		Name:          "Reassign Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.nodeID = node.ID
}

func (s *ReassignTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *ReassignTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *ReassignTaskTestSuite) insertInstanceAndTask(assigneeID string, taskStatus approval.TaskStatus) (*approval.Instance, *approval.Task) {
	s.taskSeq++
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Reassign Test",
		InstanceNo:    fmt.Sprintf("RA-%03d", s.taskSeq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     taskStatus,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	return inst, task
}

func (s *ReassignTaskTestSuite) TestReassignSuccess() {
	_, task := s.insertInstanceAndTask("original-user", approval.TaskPending)

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.ReassignTaskCmd{
		TaskID:        task.ID,
		NewAssigneeID: "new-user",
		Operator:      operator,
		Reason:        "人员调整",
	})
	s.Require().NoError(err, "Should reassign task without error")

	// Verify task assignee updated
	var updated approval.Task

	updated.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal("new-user", updated.AssigneeID, "Should update assignee to new user")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", updated.InstanceID) }).
		Scan(s.ctx), "Should not return error")
	s.Assert().Len(logs, 1, "Should insert one action log")
	s.Assert().Equal(approval.ActionReassign, logs[0].Action, "Action should be reassign")
	s.Assert().Equal("new-user", *logs[0].TransferToID, "Should record transfer target")
	s.Assert().Equal("人员调整", *logs[0].Opinion, "Should record reason in opinion")
}

func (s *ReassignTaskTestSuite) TestReassignTaskNotFound() {
	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.ReassignTaskCmd{
		TaskID:        "non-existent",
		NewAssigneeID: "new-user",
		Operator:      operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return ErrTaskNotFound")
}

func (s *ReassignTaskTestSuite) TestReassignTaskNotPending() {
	_, task := s.insertInstanceAndTask("original-user", approval.TaskApproved)

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.ReassignTaskCmd{
		TaskID:        task.ID,
		NewAssigneeID: "new-user",
		Operator:      operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should not allow reassigning non-pending task")
}

func (s *ReassignTaskTestSuite) TestReassignShouldRejectBlankTarget() {
	_, task := s.insertInstanceAndTask("original-user", approval.TaskPending)

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.ReassignTaskCmd{
		TaskID:        task.ID,
		NewAssigneeID: "   ",
		Operator:      operator,
	})
	s.Require().Error(err, "Should reject blank reassignment target")
	s.Assert().ErrorIs(err, shared.ErrInvalidTransferTarget, "Should return invalid target error for blank reassignment target")
}

func (s *ReassignTaskTestSuite) TestReassignShouldRejectSameAssignee() {
	_, task := s.insertInstanceAndTask("original-user", approval.TaskPending)

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err := s.handler.Handle(s.ctx, command.ReassignTaskCmd{
		TaskID:        task.ID,
		NewAssigneeID: "original-user",
		Operator:      operator,
	})
	s.Require().Error(err, "Should reject reassignment to current assignee")
	s.Assert().ErrorIs(err, shared.ErrInvalidTransferTarget, "Should return invalid target error for self reassignment")
}

func (s *ReassignTaskTestSuite) TestReassignShouldRejectExistingActiveTarget() {
	inst, task := s.insertInstanceAndTask("original-user", approval.TaskPending)

	existing := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "existing-user",
		SortOrder:  2,
		Status:     approval.TaskWaiting,
	}
	_, err := s.db.NewInsert().Model(existing).Exec(s.ctx)
	s.Require().NoError(err, "Should insert existing active task for reassignment target")

	operator := approval.OperatorInfo{ID: "admin-1", Name: "Admin"}
	_, err = s.handler.Handle(s.ctx, command.ReassignTaskCmd{
		TaskID:        task.ID,
		NewAssigneeID: "existing-user",
		Operator:      operator,
	})
	s.Require().Error(err, "Should reject reassignment to assignee with active task on the same node")
	s.Assert().ErrorIs(err, shared.ErrInvalidTransferTarget, "Should return invalid target error for duplicate active assignee")
}
