package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ApproveTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// ApproveTaskTestSuite tests the ApproveTaskHandler.
type ApproveTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.ApproveTaskHandler
	fixture *FlowFixture
}

func (s *ApproveTaskTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine()
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	pub := dispatcher.NewEventPublisher()

	s.handler = command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, pub)
}

func (s *ApproveTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *ApproveTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *ApproveTaskTestSuite) newRunningInstance(assigneeID string) (*approval.Instance, *approval.Task) {
	return setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, assigneeID)
}

func (s *ApproveTaskTestSuite) TestApproveSuccess() {
	inst, task := s.newRunningInstance("approver-1")

	operator := approval.OperatorInfo{ID: "approver-1", Name: "Approver"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "Approved",
	})
	s.Require().NoError(err, "Should approve task without error")

	// Verify task status
	var updated approval.Task

	updated.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.TaskApproved, updated.Status, "Task should be approved")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "Should not return error")
	s.Assert().GreaterOrEqual(len(logs), 1, "Should have at least 1 action log")

	found := false
	for _, log := range logs {
		if log.Action == approval.ActionApprove {
			found = true

			s.Assert().Equal("approver-1", log.OperatorID, "Should match expected value")
		}
	}

	s.Assert().True(found, "Should have an approve action log")
}

func (s *ApproveTaskTestSuite) TestApproveTaskNotFound() {
	operator := approval.OperatorInfo{ID: "approver-1", Name: "Approver"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   "non-existent",
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return expected error")
}

func (s *ApproveTaskTestSuite) TestApproveNotAssignee() {
	_, task := s.newRunningInstance("approver-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should return expected error")
}

func (s *ApproveTaskTestSuite) TestApproveAlreadyCompleted() {
	_, task := s.newRunningInstance("approver-1")

	// Set task to already approved
	_, err := s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskApproved).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	operator := approval.OperatorInfo{ID: "approver-1", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should return expected error")
}

func (s *ApproveTaskTestSuite) TestApproveTaskNotCurrentNode() {
	inst, task := s.newRunningInstance("approver-current")

	otherNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "approve-other-current-node",
		Kind:          approval.NodeApproval,
		Name:          "Approve Other Current Node",
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create another node")

	_, err = s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", otherNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should move instance current node away from task node")

	operator := approval.OperatorInfo{ID: "approver-current", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "approved",
	})
	s.Require().Error(err, "Should fail when approving a task not in current node")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should return task not pending for stale node task")
}

func (s *ApproveTaskTestSuite) TestApproveShouldResolveCCFromFormField() {
	inst, task := s.newRunningInstance("approver-cc-form")

	ccField := "ccUsers"
	_, err := s.db.NewInsert().Model(&approval.FlowNodeCC{
		NodeID:    task.NodeID,
		Kind:      approval.CCFormField,
		FormField: &ccField,
		Timing:    approval.CCTimingAlways,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert form-field CC config")

	// Ensure the ccUsers form field is editable so MergeFormData passes it through.
	_, err = s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("field_permissions", map[string]approval.Permission{
			ccField: approval.PermissionEditable,
		}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should update node field permissions")

	operator := approval.OperatorInfo{ID: "approver-cc-form", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "Approved",
		FormData: map[string]any{
			ccField: []string{"cc-user-2", "cc-user-3"},
		},
	})
	s.Require().NoError(err, "Should approve task with form-field CC data")

	var records []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().
		Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "Should not return error")

	userIDs := make([]string, len(records))
	for i, record := range records {
		userIDs[i] = record.CCUserID
	}

	userSet := collections.NewHashSetFrom(userIDs...)

	s.Assert().True(userSet.Contains("cc-user-1"), "Should keep existing static CC recipient")
	s.Assert().True(userSet.Contains("cc-user-2"), "Should include CC recipient resolved from form field")
	s.Assert().True(userSet.Contains("cc-user-3"), "Should include CC recipient resolved from form field")
}
