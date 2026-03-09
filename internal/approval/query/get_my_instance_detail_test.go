package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetMyInstanceDetailTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetMyInstanceDetailTestSuite tests the GetMyInstanceDetailHandler.
type GetMyInstanceDetailTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.GetMyInstanceDetailHandler

	instanceID string
	nodeID     string
}

func (s *GetMyInstanceDetailTestSuite) SetupSuite() {
	s.handler = query.NewGetMyInstanceDetailHandler(s.db, service.NewTaskService())

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mid-flow", 1)
	s.nodeID = fix.NodeIDs[0]

	inst := &approval.Instance{
		TenantID: "t1", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "Detail Instance", InstanceNo: "MID-001", ApplicantID: "user-a",
		Status: approval.InstanceRunning, CurrentNodeID: &fix.NodeIDs[0],
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")
	s.instanceID = inst.ID

	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-b", SortOrder: 1, Status: approval.TaskPending},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	logs := []approval.ActionLog{
		{InstanceID: inst.ID, Action: approval.ActionSubmit, OperatorID: "user-a", OperatorName: "Applicant A"},
	}
	for i := range logs {
		_, err := s.db.NewInsert().Model(&logs[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert action log")
	}

	// Add CC record for user-c.
	ccRecords := []approval.CCRecord{
		{InstanceID: inst.ID, NodeID: &fix.NodeIDs[0], CCUserID: "user-c", IsManual: false},
	}
	for i := range ccRecords {
		_, err := s.db.NewInsert().Model(&ccRecords[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert CC record")
	}
}

func (s *GetMyInstanceDetailTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetMyInstanceDetailTestSuite) TestApplicantAccess() {
	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: s.instanceID,
		UserID:     "user-a",
	})
	s.Require().NoError(err, "Should get detail without error")
	s.Assert().Equal(s.instanceID, detail.Instance.InstanceID, "Should return correct instance")
	s.Assert().Equal("Detail Instance", detail.Instance.Title, "Should return correct title")
	s.Assert().Len(detail.Tasks, 1, "Should return 1 task")
	s.Assert().Len(detail.ActionLogs, 1, "Should return 1 action log")
	s.Assert().Contains(detail.AvailableActions, "withdraw", "Applicant should be able to withdraw")
	s.Assert().Contains(detail.AvailableActions, "urge", "Applicant should be able to urge when the instance has pending tasks")
}

func (s *GetMyInstanceDetailTestSuite) TestAssigneeAccess() {
	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: s.instanceID,
		UserID:     "user-b",
	})
	s.Require().NoError(err, "Assignee should have access")
	s.Assert().Contains(detail.AvailableActions, "approve", "Assignee should be able to approve")
	s.Assert().Contains(detail.AvailableActions, "reject", "Assignee should be able to reject")
	s.Assert().Contains(detail.AvailableActions, "urge", "Assignee should be able to urge when the instance has pending tasks")
}

func (s *GetMyInstanceDetailTestSuite) TestCCAccess() {
	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: s.instanceID,
		UserID:     "user-c",
	})
	s.Require().NoError(err, "CC user should have access")
	s.Assert().Equal(s.instanceID, detail.Instance.InstanceID, "Should return correct instance")
	s.Assert().Contains(detail.AvailableActions, "urge", "CC participant should be able to urge when the instance has pending tasks")
}

func (s *GetMyInstanceDetailTestSuite) TestAssigneeConditionalActions() {
	var baseInstance approval.Instance

	baseInstance.ID = s.instanceID
	err := s.db.NewSelect().Model(&baseInstance).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should load base instance")

	node := &approval.FlowNode{
		FlowVersionID:        baseInstance.FlowVersionID,
		Key:                  "mid-conditional-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Conditional Node",
		IsTransferAllowed:    true,
		IsRollbackAllowed:    true,
		IsAddAssigneeAllowed: true,
		IsManualCCAllowed:    true,
	}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert conditional node")

	inst := &approval.Instance{
		TenantID:      baseInstance.TenantID,
		FlowID:        baseInstance.FlowID,
		FlowVersionID: baseInstance.FlowVersionID,
		Title:         "Conditional Instance",
		InstanceNo:    "MID-002",
		ApplicantID:   "user-z",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &node.ID,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert conditional instance")

	_, err = s.db.NewInsert().Model(&approval.Task{
		TenantID:   inst.TenantID,
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "user-conditional",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert conditional pending task")

	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: inst.ID,
		UserID:     "user-conditional",
	})
	s.Require().NoError(err, "Should get detail for conditional assignee")
	s.Assert().Contains(detail.AvailableActions, "transfer", "Assignee should see transfer when node allows transfer")
	s.Assert().Contains(detail.AvailableActions, "rollback", "Assignee should see rollback when node allows rollback")
	s.Assert().Contains(detail.AvailableActions, "add_assignee", "Assignee should see add_assignee when node allows add assignee")
	s.Assert().Contains(detail.AvailableActions, "add_cc", "Assignee should see add_cc when node allows manual CC")
}

func (s *GetMyInstanceDetailTestSuite) TestHandleNodeShouldExposeHandleAction() {
	var baseInstance approval.Instance

	baseInstance.ID = s.instanceID
	err := s.db.NewSelect().Model(&baseInstance).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should load base instance")

	node := &approval.FlowNode{
		FlowVersionID: baseInstance.FlowVersionID,
		Key:           "mid-handle-node",
		Kind:          approval.NodeHandle,
		Name:          "Handle Node",
	}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert handle node")

	inst := &approval.Instance{
		TenantID:      baseInstance.TenantID,
		FlowID:        baseInstance.FlowID,
		FlowVersionID: baseInstance.FlowVersionID,
		Title:         "Handle Instance",
		InstanceNo:    "MID-003",
		ApplicantID:   "user-h",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &node.ID,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert handle instance")

	_, err = s.db.NewInsert().Model(&approval.Task{
		TenantID:   inst.TenantID,
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "user-handle",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert handle task")

	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: inst.ID,
		UserID:     "user-handle",
	})
	s.Require().NoError(err, "Should get detail for handle assignee")
	s.Assert().Contains(detail.AvailableActions, "handle", "Handle node should expose handle action")
	s.Assert().NotContains(detail.AvailableActions, "approve", "Handle node should not expose approve action")
}

func (s *GetMyInstanceDetailTestSuite) TestAccessDenied() {
	_, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: s.instanceID,
		UserID:     "user-unrelated",
	})
	s.Require().ErrorIs(err, shared.ErrAccessDenied, "Should return access denied for non-participant")
}

func (s *GetMyInstanceDetailTestSuite) TestInstanceNotFound() {
	_, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: "non-existent",
		UserID:     "user-a",
	})
	s.Require().ErrorIs(err, shared.ErrInstanceNotFound, "Should return instance not found error")
}
