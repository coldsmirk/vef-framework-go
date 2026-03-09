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
		return &TransferTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// TransferTaskTestSuite tests the TransferTaskHandler.
type TransferTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.TransferTaskHandler
	fixture *MinimalFixture
	nodeID  string

	instanceSeq int
}

func (s *TransferTaskTestSuite) SetupSuite() {
	taskSvc := service.NewTaskService()
	validSvc := service.NewValidationService(nil)
	pub := dispatcher.NewEventPublisher()
	s.handler = command.NewTransferTaskHandler(s.db, taskSvc, validSvc, pub, nil)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "transfer")

	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "transfer-node",
		Kind:              approval.NodeApproval,
		Name:              "Transfer Node",
		IsTransferAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.nodeID = node.ID
}

func (s *TransferTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *TransferTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *TransferTaskTestSuite) setupData(assigneeID string) (*approval.Instance, *approval.Task) {
	s.instanceSeq++
	instanceNo := fmt.Sprintf("TR-%03d", s.instanceSeq)

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Transfer Test",
		InstanceNo:    instanceNo,
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &s.nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	return inst, task
}

func (s *TransferTaskTestSuite) TestTransferSuccess() {
	_, task := s.setupData("assignee-1")

	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Original Assignee"}
	_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TransferToID: "new-assignee-1",
		Opinion:      "Need expert review",
	})
	s.Require().NoError(err, "Should transfer task without error")

	// Verify original task transferred
	var original approval.Task

	original.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&original).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.TaskTransferred, original.Status, "Original task should be transferred")

	// Verify new task created for transferee
	var newTasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&newTasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", task.InstanceID).
				Equals("assignee_id", "new-assignee-1")
		}).
		Scan(s.ctx), "Should not return error")
	s.Assert().Len(newTasks, 1, "Should create one new task for transferee")
	s.Assert().Equal(approval.TaskPending, newTasks[0].Status, "New task should be pending")
}

func (s *TransferTaskTestSuite) TestTransferNotAllowed() {
	// Create node with transfer disabled
	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "no-transfer-node",
		Kind:              approval.NodeApproval,
		Name:              "No Transfer Node",
		IsTransferAllowed: false,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)

	s.Require().NoError(err, "Should not return error")
	defer func() {
		_, _ = s.db.NewDelete().Model(node).WherePK().Exec(s.ctx)
	}()

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "No Transfer Test",
		InstanceNo:    "TR-002",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &node.ID,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "assignee-2",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	operator := approval.OperatorInfo{ID: "assignee-2", Name: "Assignee"}
	_, err = s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TransferToID: "new-assignee",
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTransferNotAllowed, "Should return expected error")
}

func (s *TransferTaskTestSuite) TestTransferTaskNotFound() {
	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Assignee"}
	_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       "non-existent",
		Operator:     operator,
		TransferToID: "new-assignee",
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return expected error")
}

func (s *TransferTaskTestSuite) TestTransferNotAssignee() {
	_, task := s.setupData("assignee-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TransferToID: "new-assignee",
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should return expected error")
}

func (s *TransferTaskTestSuite) TestTransferTaskNotCurrentNode() {
	inst, task := s.setupData("assignee-current")

	otherNode := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "transfer-other-current-node",
		Kind:              approval.NodeApproval,
		Name:              "Transfer Other Current Node",
		IsTransferAllowed: true,
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create another transfer node")

	_, err = s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", otherNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should move instance current node away from task node")

	operator := approval.OperatorInfo{ID: "assignee-current", Name: "Assignee"}
	_, err = s.handler.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TransferToID: "new-assignee",
	})
	s.Require().Error(err, "Should fail when transferring a task not in current node")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should return task not pending for stale node task")
}

func (s *TransferTaskTestSuite) TestTransferTargetValidation() {
	s.Run("EmptyTarget", func() {
		_, task := s.setupData("assignee-empty")

		operator := approval.OperatorInfo{ID: "assignee-empty", Name: "Assignee"}
		_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
			TaskID:       task.ID,
			Operator:     operator,
			TransferToID: "   ",
		})
		s.Require().Error(err, "Should fail when transfer target is empty")
		s.Assert().ErrorIs(err, shared.ErrInvalidTransferTarget, "Should return invalid transfer target for empty target")
	})

	s.Run("SelfTarget", func() {
		_, task := s.setupData("assignee-self")

		operator := approval.OperatorInfo{ID: "assignee-self", Name: "Assignee"}
		_, err := s.handler.Handle(s.ctx, command.TransferTaskCmd{
			TaskID:       task.ID,
			Operator:     operator,
			TransferToID: "assignee-self",
		})
		s.Require().Error(err, "Should fail when transferring to self")
		s.Assert().ErrorIs(err, shared.ErrInvalidTransferTarget, "Should return invalid transfer target for self transfer")
	})

	s.Run("DuplicateActiveTarget", func() {
		_, task := s.setupData("assignee-source")

		existingTargetTask := &approval.Task{
			TenantID:   "default",
			InstanceID: task.InstanceID,
			NodeID:     task.NodeID,
			AssigneeID: "assignee-target",
			SortOrder:  2,
			Status:     approval.TaskPending,
		}
		_, err := s.db.NewInsert().Model(existingTargetTask).Exec(s.ctx)
		s.Require().NoError(err, "Should create existing active task for transfer target")

		operator := approval.OperatorInfo{ID: "assignee-source", Name: "Assignee"}
		_, err = s.handler.Handle(s.ctx, command.TransferTaskCmd{
			TaskID:       task.ID,
			Operator:     operator,
			TransferToID: "assignee-target",
		})
		s.Require().Error(err, "Should fail when target already has active task on node")
		s.Assert().ErrorIs(err, shared.ErrInvalidTransferTarget, "Should return invalid transfer target for duplicate active assignee")

		var original approval.Task

		original.ID = task.ID
		s.Require().NoError(
			s.db.NewSelect().Model(&original).WherePK().Scan(s.ctx),
			"Should reload original task after failed transfer",
		)
		s.Assert().Equal(approval.TaskPending, original.Status, "Original task should remain pending on validation failure")
	})
}
