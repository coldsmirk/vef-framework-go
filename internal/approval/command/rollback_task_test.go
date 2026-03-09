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
		return &RollbackTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// RollbackTaskTestSuite tests the RollbackTaskHandler.
type RollbackTaskTestSuite struct {
	suite.Suite

	ctx          context.Context
	db           orm.DB
	handler      *command.RollbackTaskHandler
	fixture      *MinimalFixture
	rollbackNode *approval.FlowNode
	targetNode   *approval.FlowNode

	instanceSeq int
}

func (s *RollbackTaskTestSuite) SetupSuite() {
	eng := buildTestEngine()
	taskSvc := service.NewTaskService()
	validSvc := service.NewValidationService(nil)
	pub := dispatcher.NewEventPublisher()
	s.handler = command.NewRollbackTaskHandler(s.db, taskSvc, validSvc, eng, pub)

	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "rollback")

	s.rollbackNode = &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "rollback-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Rollback Node",
		IsRollbackAllowed:    true,
		RollbackType:         approval.RollbackAny,
		RollbackDataStrategy: approval.RollbackDataClear,
	}
	_, err := s.db.NewInsert().Model(s.rollbackNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create rollback node")

	s.targetNode = &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "rollback-target",
		Kind:          approval.NodeApproval,
		Name:          "Rollback Target Node",
	}
	_, err = s.db.NewInsert().Model(s.targetNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create rollback target node")
}

func (s *RollbackTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *RollbackTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *RollbackTaskTestSuite) setupData(assigneeID string) (*approval.Instance, *approval.Task) {
	s.instanceSeq++
	instanceNo := fmt.Sprintf("RB-%03d", s.instanceSeq)

	currentNodeID := s.rollbackNode.ID
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Rollback Test",
		InstanceNo:    instanceNo,
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &currentNodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.rollbackNode.ID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task on rollback node")

	return inst, task
}

func (s *RollbackTaskTestSuite) TestRollbackTaskNotFound() {
	operator := approval.OperatorInfo{ID: "rollback-operator", Name: "Rollback Operator"}
	_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       "non-existent",
		Operator:     operator,
		TargetNodeID: s.targetNode.ID,
	})
	s.Require().Error(err, "Should fail when task does not exist")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return task not found")
}

func (s *RollbackTaskTestSuite) TestRollbackTaskNotCurrentNode() {
	inst, task := s.setupData("rollback-operator")

	_, err := s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", s.targetNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should move instance current node away from task node")

	operator := approval.OperatorInfo{ID: "rollback-operator", Name: "Rollback Operator"}
	_, err = s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: s.targetNode.ID,
		Opinion:      "rollback",
	})
	s.Require().Error(err, "Should fail when rolling back a task not in current node")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should return task not pending for stale node task")
}

func (s *RollbackTaskTestSuite) TestRollbackTargetRequired() {
	s.Run("EmptyTarget", func() {
		_, task := s.setupData("rollback-empty")

		operator := approval.OperatorInfo{ID: "rollback-empty", Name: "Rollback Operator"}
		_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
			TaskID:       task.ID,
			Operator:     operator,
			TargetNodeID: "",
			Opinion:      "rollback",
		})
		s.Require().Error(err, "Should fail when rollback target is empty")
		s.Assert().ErrorIs(err, shared.ErrInvalidRollbackTarget, "Should return invalid rollback target for empty input")
	})

	s.Run("BlankTarget", func() {
		_, task := s.setupData("rollback-blank")

		operator := approval.OperatorInfo{ID: "rollback-blank", Name: "Rollback Operator"}
		_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
			TaskID:       task.ID,
			Operator:     operator,
			TargetNodeID: "   ",
			Opinion:      "rollback",
		})
		s.Require().Error(err, "Should fail when rollback target is blank")
		s.Assert().ErrorIs(err, shared.ErrInvalidRollbackTarget, "Should return invalid rollback target for blank input")
	})
}

func (s *RollbackTaskTestSuite) TestRollbackTargetShouldNotBeCurrentNode() {
	_, task := s.setupData("rollback-current-node")

	operator := approval.OperatorInfo{ID: "rollback-current-node", Name: "Rollback Operator"}
	_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: s.rollbackNode.ID,
		Opinion:      "rollback to current",
	})
	s.Require().Error(err, "Should fail when rollback target is current node")
	s.Assert().ErrorIs(err, shared.ErrInvalidRollbackTarget, "Should return invalid rollback target when target equals current node")

	var reloaded approval.Task

	reloaded.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx),
		"Should reload task after rollback target validation failure",
	)
	s.Assert().Equal(approval.TaskPending, reloaded.Status, "Task should remain pending when rollback target validation fails")
}
