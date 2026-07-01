package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
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

	// Pin a form schema on the instance's version so the detail can project the
	// metadata the UI needs to render form data (labels, field kinds, order).
	formSchema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "reason", Kind: approval.FieldTextarea, Label: "Reason", IsRequired: true, SortOrder: 1},
			{Key: "days", Kind: approval.FieldNumber, Label: "Days", SortOrder: 2},
		},
	}
	_, err := s.db.NewUpdate().Model((*approval.FlowVersion)(nil)).
		Set("form_schema", formSchema).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(fix.VersionID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set form schema on version")

	department := "Engineering"
	inst := &approval.Instance{
		TenantID: "t1", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "Detail Instance", InstanceNo: "MID-001", ApplicantID: "user-a",
		ApplicantDepartmentName: &department,
		Status:                  approval.InstanceRunning, CurrentNodeID: &fix.NodeIDs[0],
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")
	s.instanceID = inst.ID

	delegatorID, delegatorName := "user-deleg", "Delegator D"
	deadline := timex.Now()

	tasks := []approval.Task{
		{
			TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-b",
			SortOrder: 1, Status: approval.TaskPending,
			DelegatorID: &delegatorID, DelegatorName: &delegatorName, Deadline: &deadline, IsTimeout: true,
		},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	transferToID, transferToName := "user-x", "Transfer Target"
	rollbackToNodeID := fix.NodeIDs[0]

	logs := []approval.ActionLog{
		{InstanceID: inst.ID, Action: approval.ActionSubmit, OperatorID: "user-a", OperatorName: "Applicant A", NodeID: &fix.NodeIDs[0]},
		{
			InstanceID:     inst.ID,
			Action:         approval.ActionTransfer,
			OperatorID:     "user-b",
			OperatorName:   "Approver B",
			NodeID:         &fix.NodeIDs[0],
			TransferToID:   &transferToID,
			TransferToName: &transferToName,
		},
		{InstanceID: inst.ID, Action: approval.ActionRollback, OperatorID: "user-b", OperatorName: "Approver B", NodeID: &fix.NodeIDs[0], RollbackToNodeID: &rollbackToNodeID},
		{
			InstanceID:         inst.ID,
			Action:             approval.ActionAddAssignee,
			OperatorID:         "user-b",
			OperatorName:       "Approver B",
			NodeID:             &fix.NodeIDs[0],
			AddedAssigneeIDs:   []string{"user-e"},
			AddedAssigneeNames: []string{"Ellen"},
			Attachments:        []string{"file-1.pdf"},
		},
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
	s.Assert().Len(detail.ActionLogs, 4, "Should return all 4 action logs")
	s.Assert().Contains(detail.AvailableActions, "withdraw", "Applicant should be able to withdraw")
	s.Assert().Contains(detail.AvailableActions, "urge", "Applicant should be able to urge when the instance has pending tasks")

	// Form metadata must ship with the detail so the UI can render form data.
	s.Require().NotNil(detail.Instance.FormSchema, "Detail should carry the version's form schema")
	s.Assert().Len(detail.Instance.FormSchema.Fields, 2, "Form schema should carry both fields")
	s.Require().NotNil(detail.Instance.ApplicantDepartmentName, "Detail should carry the applicant department")
	s.Assert().Equal("Engineering", *detail.Instance.ApplicantDepartmentName, "Applicant department should pass through")
	s.Require().NotNil(detail.Instance.CurrentNodeID, "Detail should carry the current node id")

	// The flow graph is React Flow–ready and marks the node the instance sits on
	// together with that node's participants.
	s.Require().NotEmpty(detail.FlowGraph.Nodes, "Detail should carry the flow graph nodes")

	var current *approval.FlowGraphNode

	for i := range detail.FlowGraph.Nodes {
		if detail.FlowGraph.Nodes[i].Data.Status == approval.NodeProgressCurrent {
			current = &detail.FlowGraph.Nodes[i]
		}
	}

	s.Require().NotNil(current, "Graph should mark the current node")
	s.Require().Len(current.Data.Participants, 1, "Current node should list its pending assignee")
	s.Assert().Equal("user-b", current.Data.Participants[0].UserID, "Participant should be the pending assignee")
}

func (s *GetMyInstanceDetailTestSuite) TestProjectsTaskAndLogDetails() {
	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: s.instanceID,
		UserID:     "user-a",
	})
	s.Require().NoError(err, "Applicant should get detail")

	s.Require().Len(detail.Tasks, 1, "Should return the single task")
	task := detail.Tasks[0]
	s.Require().NotNil(task.DelegatorName, "Task should carry the delegator name")
	s.Assert().Equal("Delegator D", *task.DelegatorName, "Delegator name should pass through")
	s.Require().NotNil(task.Deadline, "Task should carry the deadline")
	s.Assert().True(task.IsTimeout, "Task timeout flag should pass through")

	// Locate logs by action rather than index (created_at order is not asserted).
	var transfer, rollback, addAssignee *my.ActionLogInfo

	for i := range detail.ActionLogs {
		switch detail.ActionLogs[i].Action {
		case string(approval.ActionTransfer):
			transfer = &detail.ActionLogs[i]
		case string(approval.ActionRollback):
			rollback = &detail.ActionLogs[i]
		case string(approval.ActionAddAssignee):
			addAssignee = &detail.ActionLogs[i]
		}
	}

	s.Require().NotNil(transfer, "Detail should include the transfer log")
	s.Assert().NotEmpty(transfer.LogID, "Log should carry a stable id for list rendering")
	s.Assert().Equal("user-b", transfer.OperatorID, "Operator id should pass through")
	s.Require().NotNil(transfer.NodeID, "Transfer log should carry the node id")
	s.Require().NotNil(transfer.TransferToName, "Transfer log should carry the transfer target name")
	s.Assert().Equal("Transfer Target", *transfer.TransferToName, "Transfer target name should pass through")

	s.Require().NotNil(rollback, "Detail should include the rollback log")
	s.Require().NotNil(rollback.RollbackToNodeID, "Rollback log should carry the rollback target node id")
	s.Assert().Equal(s.nodeID, *rollback.RollbackToNodeID, "Rollback target node id should pass through")

	// Name snapshots and attachments must reach the DTO for display.
	s.Require().NotNil(addAssignee, "Detail should include the add-assignee log")
	s.Require().Len(addAssignee.AddedAssignees, 1, "Add-assignee log should carry the added user")
	s.Assert().Equal("user-e", addAssignee.AddedAssignees[0].ID, "Added assignee id should pass through")
	s.Assert().Equal("Ellen", addAssignee.AddedAssignees[0].Name, "Added assignee name snapshot should pass through")
	s.Assert().Equal([]string{"file-1.pdf"}, addAssignee.Attachments, "Attachments should pass through")
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
