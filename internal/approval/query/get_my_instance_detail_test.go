package query_test

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
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
	formSchema json.RawMessage
}

func (s *GetMyInstanceDetailTestSuite) SetupSuite() {
	s.handler = query.NewGetMyInstanceDetailHandler(s.db, service.NewTaskService())

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mid-flow", 1)
	s.nodeID = fix.NodeIDs[0]

	// Pin a host form-designer document on the instance's version; the detail
	// must return it verbatim — the framework never interprets it.
	s.formSchema = json.RawMessage(`{"version":2,"presentations":{"pc":{"children":[` +
		`{"id":"F1","type":"textarea","key":"reason","label":"Reason"},` +
		`{"id":"F2","type":"number","key":"days","label":"Days"}]}}}`)
	_, err := s.db.NewUpdate().Model((*approval.FlowVersion)(nil)).
		Set("form_schema", s.formSchema).
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

	// Record the visit the engine would have opened on the approval node.
	nodeVisit := &approval.NodeVisit{
		TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0],
		Sequence: 1, Status: approval.NodeVisitActive,
	}
	_, err = s.db.NewInsert().Model(nodeVisit).Exec(s.ctx)
	s.Require().NoError(err, "Should insert node visit")

	delegatorID, delegatorName, delegatorDept := "user-deleg", "Delegator D", "Ops"
	deadline := timex.Now()

	tasks := []approval.Task{
		{
			TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], VisitID: nodeVisit.ID,
			AssigneeID: "user-b", SortOrder: 1, Status: approval.TaskPending,
			DelegatorID: &delegatorID, DelegatorName: &delegatorName, DelegatorDepartmentName: &delegatorDept,
			Deadline: &deadline, IsTimeout: true,
		},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	transferToID, transferToName := "user-x", "Transfer Target"
	rollbackToNodeID := fix.NodeIDs[0]

	logs := []approval.ActionLog{
		{InstanceID: inst.ID, Action: approval.ActionSubmit, OperatorID: "user-a", OperatorName: "Applicant A"},
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
			InstanceID:     inst.ID,
			Action:         approval.ActionAddAssignee,
			OperatorID:     "user-b",
			OperatorName:   "Approver B",
			NodeID:         &fix.NodeIDs[0],
			AddedAssignees: []approval.UserInfo{{ID: "user-e", Name: "Ellen"}},
			Attachments:    []string{"file-1.pdf"},
		},
	}
	for i := range logs {
		_, err := s.db.NewInsert().Model(&logs[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert action log")
	}

	// Add CC record for user-c (auto CC configured on the approval node).
	ccDept := "Legal"

	ccRecords := []approval.CCRecord{
		{InstanceID: inst.ID, NodeID: &fix.NodeIDs[0], VisitID: &nodeVisit.ID, CCUserID: "user-c", CCUserName: "CC User", CCUserDepartmentName: &ccDept, IsManual: false},
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
	s.Assert().Contains(detail.AvailableActions, "withdraw", "Applicant should be able to withdraw")
	s.Assert().Contains(detail.AvailableActions, "urge", "Applicant should be able to urge when the instance has pending tasks")

	// The designer document must ship with the detail verbatim so the UI can
	// render form data against the exact schema the instance was submitted under.
	s.Require().NotEmpty(detail.FormSchema, "Detail should carry the version's form schema")
	s.Assert().JSONEq(string(s.formSchema), string(detail.FormSchema), "Form schema should pass through verbatim")
	s.Assert().Equal("user-a", detail.Instance.Applicant.ID, "Applicant snapshot should carry the id")
	s.Require().NotNil(detail.Instance.Applicant.DepartmentName, "Applicant snapshot should carry the department")
	s.Assert().Equal("Engineering", *detail.Instance.Applicant.DepartmentName, "Applicant department should pass through")
	s.Require().NotNil(detail.Instance.CurrentNodeID, "Detail should carry the current node id")

	// The flow graph is React Flow–ready and marks the node the instance sits on
	// together with that node's participants.
	s.Require().NotEmpty(detail.FlowGraph.Nodes, "Detail should carry the flow graph nodes")

	var current *approval.FlowGraphNode

	for i := range detail.FlowGraph.Nodes {
		if detail.FlowGraph.Nodes[i].Data.Status == approval.NodeProgressActive {
			current = &detail.FlowGraph.Nodes[i]
		}
	}

	s.Require().NotNil(current, "Graph should mark the node with the open visit as active")
	s.Require().Len(current.Data.Participants, 1, "Active node should list its pending assignee")
	s.Assert().Equal("user-b", current.Data.Participants[0].User.ID, "Participant should be the pending assignee")
}

func (s *GetMyInstanceDetailTestSuite) TestProjectsTimelineDetails() {
	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: s.instanceID,
		UserID:     "user-a",
	})
	s.Require().NoError(err, "Applicant should get detail")

	// One approval visit → one timeline entry carrying everything that
	// happened at the node: the participant, the CC delivery, and the side
	// activities.
	s.Require().Len(detail.Timeline, 1, "Timeline should carry the single visited node")
	entry := detail.Timeline[0]
	s.Assert().Equal(approval.TimelineEntryApproval, entry.Kind, "Entry kind should mirror the node kind")
	s.Assert().Equal(approval.NodeVisitActive, entry.Status, "Entry should report the open visit")

	s.Require().Len(entry.Participants, 1, "Entry should list the single participant")
	participant := entry.Participants[0]
	s.Assert().Equal("user-b", participant.User.ID, "Participant should be the assignee")
	s.Require().NotNil(participant.Delegator, "Participant should carry the delegator snapshot")
	s.Assert().Equal("Delegator D", participant.Delegator.Name, "Delegator name should pass through")
	s.Require().NotNil(participant.Delegator.DepartmentName, "Delegator department should pass through")
	s.Require().NotNil(participant.Deadline, "Participant should carry the deadline")
	s.Assert().True(participant.IsTimeout, "Participant timeout flag should pass through")

	s.Require().Len(entry.CCRecipients, 1, "Entry should list the CC recipient")
	s.Assert().Equal("user-c", entry.CCRecipients[0].User.ID, "CC recipient id should pass through")
	s.Require().NotNil(entry.CCRecipients[0].User.DepartmentName, "CC recipient department should pass through")
	s.Assert().Equal("Legal", *entry.CCRecipients[0].User.DepartmentName, "CC recipient department should match the snapshot")

	// Locate activities by action rather than index (same-second inserts).
	var transfer, rollback, addAssignee *approval.Activity

	for i := range entry.Activities {
		switch entry.Activities[i].Action {
		case string(approval.ActionTransfer):
			transfer = &entry.Activities[i]
		case string(approval.ActionRollback):
			rollback = &entry.Activities[i]
		case string(approval.ActionAddAssignee):
			addAssignee = &entry.Activities[i]
		}
	}

	s.Require().NotNil(transfer, "Entry should include the transfer activity")
	s.Assert().Equal("user-b", transfer.Operator.ID, "Operator id should pass through")
	s.Require().NotNil(transfer.TransferTo, "Transfer activity should carry the transfer target")
	s.Assert().Equal("Transfer Target", transfer.TransferTo.Name, "Transfer target name should pass through")

	s.Require().NotNil(rollback, "Entry should include the rollback activity")
	s.Require().NotNil(rollback.RollbackToNodeID, "Rollback activity should carry the target node id")
	s.Assert().Equal(s.nodeID, *rollback.RollbackToNodeID, "Rollback target node id should pass through")

	// Person snapshots and attachments must reach the DTO for display.
	s.Require().NotNil(addAssignee, "Entry should include the add-assignee activity")
	s.Require().Len(addAssignee.AddedAssignees, 1, "Add-assignee activity should carry the added user")
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
	s.Assert().NotContains(detail.AvailableActions, "urge", "CC-only participants must not be offered urge — mirrors IsUrgeAuthorized")
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
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, node.ID).ID,
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
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, node.ID).ID,
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

func (s *GetMyInstanceDetailTestSuite) TestFieldPermissionsProjection() {
	// A dedicated flow whose version carries form fields and whose approval node
	// declares field permissions — the fixture flow has neither, so the viewer
	// projection needs its own isolated chain.
	category := &approval.FlowCategory{TenantID: "default", Code: "perm-cat", Name: "Perm Category"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category")

	flow := &approval.Flow{
		TenantID: "default", CategoryID: category.ID, Code: "perm-flow", Name: "Perm Flow",
		BindingMode: approval.BindingStandalone, IsAllInitiationAllowed: true,
		InstanceTitleTemplate: "Test", IsActive: true,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow")

	version := &approval.FlowVersion{
		FlowID: flow.ID, Version: 1, Status: approval.VersionPublished,
		FormFields: []approval.FormFieldDefinition{{Key: "reason"}, {Key: "secret"}},
	}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "Should insert version with form fields")

	node := &approval.FlowNode{
		FlowVersionID: version.ID, Key: "perm-node", Kind: approval.NodeApproval, Name: "Perm Node",
		FieldPermissions: map[string]approval.Permission{
			"reason": approval.PermissionEditable,
			"secret": approval.PermissionHidden,
		},
	}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert node with field permissions")

	inst := &approval.Instance{
		TenantID: "default", FlowID: flow.ID, FlowVersionID: version.ID,
		Title: "Perm Instance", InstanceNo: "PERM-001", ApplicantID: "perm-applicant",
		Status: approval.InstanceRunning, CurrentNodeID: &node.ID,
		FormData: map[string]any{"reason": "please approve", "secret": "confidential", "legacy": "kept"},
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance with form data")

	_, err = s.db.NewInsert().Model(&approval.Task{
		TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: node.ID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, node.ID).ID,
		AssigneeID: "perm-approver", SortOrder: 1, Status: approval.TaskPending,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert pending task")

	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: inst.ID,
		UserID:     "perm-approver",
	})
	s.Require().NoError(err, "Pending approver should get detail")

	// The response carries the viewer projection over every top-level field: the
	// pending approver edits "reason" (full strength from the node) and cannot
	// see "secret".
	s.Require().NotNil(detail.FieldPermissions, "Detail should carry the field-permission projection")
	s.Assert().Equal(map[string]approval.Permission{
		"reason": approval.PermissionEditable,
		"secret": approval.PermissionHidden,
	}, detail.FieldPermissions, "Projection should reflect the node's field permissions for a pending approver")

	// The hidden field is stripped from the returned form data; the visible field
	// and the schemaless legacy key both survive.
	s.Assert().Contains(detail.Instance.FormData, "reason", "Visible field should reach the viewer")
	s.Assert().Contains(detail.Instance.FormData, "legacy", "Schemaless legacy field should survive stripping")
	s.Assert().NotContains(detail.Instance.FormData, "secret", "Hidden field must be stripped from the returned form data")

	// The stored form data is untouched — stripping is a read-path projection.
	var stored approval.Instance

	stored.ID = inst.ID
	err = s.db.NewSelect().Model(&stored).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should reload the stored instance")
	s.Assert().Contains(stored.FormData, "secret", "Hidden field must remain in the database")
}

// TestTableFieldHiddenStripped pins that a table-kind form field resolved hidden
// for the viewer has its whole row-array value stripped from the returned form
// data — a table counts as one permission key, so hiding it drops the array
// wholesale — while a visible scalar field on the same node survives.
func (s *GetMyInstanceDetailTestSuite) TestTableFieldHiddenStripped() {
	category := &approval.FlowCategory{TenantID: "default", Code: "tbl-hidden-cat", Name: "Table Hidden Category"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category")

	flow := &approval.Flow{
		TenantID: "default", CategoryID: category.ID, Code: "tbl-hidden-flow", Name: "Table Hidden Flow",
		BindingMode: approval.BindingStandalone, IsAllInitiationAllowed: true,
		InstanceTitleTemplate: "Test", IsActive: true,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow")

	version := &approval.FlowVersion{
		FlowID: flow.ID, Version: 1, Status: approval.VersionPublished,
		FormFields: []approval.FormFieldDefinition{
			{Key: "items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "qty", Kind: approval.FieldNumber}}},
			{Key: "reason", Kind: approval.FieldInput},
		},
	}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "Should insert version with a table and a scalar field")

	node := &approval.FlowNode{
		FlowVersionID: version.ID, Key: "tbl-hidden-node", Kind: approval.NodeApproval, Name: "Table Hidden Node",
		FieldPermissions: map[string]approval.Permission{
			"items":  approval.PermissionHidden,
			"reason": approval.PermissionEditable,
		},
	}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert node hiding the table field")

	inst := &approval.Instance{
		TenantID: "default", FlowID: flow.ID, FlowVersionID: version.ID,
		Title: "Table Hidden Instance", InstanceNo: "TBLHID-001", ApplicantID: "tbl-applicant",
		Status: approval.InstanceRunning, CurrentNodeID: &node.ID,
		FormData: map[string]any{
			"items":  []any{map[string]any{"qty": 2}, map[string]any{"qty": 5}},
			"reason": "please approve",
			"legacy": "kept",
		},
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance with table row data")

	_, err = s.db.NewInsert().Model(&approval.Task{
		TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: node.ID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, node.ID).ID,
		AssigneeID: "tbl-approver", SortOrder: 1, Status: approval.TaskPending,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert pending task")

	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: inst.ID,
		UserID:     "tbl-approver",
	})
	s.Require().NoError(err, "Pending approver should get detail")

	s.Assert().Equal(approval.PermissionHidden, detail.FieldPermissions["items"], "The table field should resolve hidden for the viewer")
	s.Assert().NotContains(detail.Instance.FormData, "items", "The hidden table field's row-array value must be stripped wholesale")
	s.Assert().Contains(detail.Instance.FormData, "reason", "The visible scalar field must survive stripping")
	s.Assert().Contains(detail.Instance.FormData, "legacy", "A schemaless legacy field must survive stripping")

	// Stripping is a read-path projection; the stored row-array is untouched.
	var stored approval.Instance

	stored.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&stored).WherePK().Scan(s.ctx), "Should reload stored instance")
	s.Assert().Contains(stored.FormData, "items", "The table row data must remain in the database")
}

func (s *GetMyInstanceDetailTestSuite) TestViewerTaskContext() {
	var baseInstance approval.Instance

	baseInstance.ID = s.instanceID
	err := s.db.NewSelect().Model(&baseInstance).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should load base instance")

	// A prior decision node the instance already passed — the only valid
	// rollback destination under RollbackAny.
	priorNode := &approval.FlowNode{
		FlowVersionID: baseInstance.FlowVersionID,
		Key:           "vt-prior",
		Kind:          approval.NodeApproval,
		Name:          "Prior Node",
	}
	_, err = s.db.NewInsert().Model(priorNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert prior node")

	currentNode := &approval.FlowNode{
		FlowVersionID:        baseInstance.FlowVersionID,
		Key:                  "vt-current",
		Kind:                 approval.NodeApproval,
		Name:                 "Current Node",
		IsOpinionRequired:    true,
		IsAddAssigneeAllowed: true,
		AddAssigneeTypes:     []approval.AddAssigneeType{approval.AddAssigneeBefore, approval.AddAssigneeParallel},
		IsRollbackAllowed:    true,
		RollbackType:         approval.RollbackAny,
	}
	_, err = s.db.NewInsert().Model(currentNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert current node")

	inst := &approval.Instance{
		TenantID:      baseInstance.TenantID,
		FlowID:        baseInstance.FlowID,
		FlowVersionID: baseInstance.FlowVersionID,
		Title:         "Viewer Task Instance",
		InstanceNo:    "MID-004",
		ApplicantID:   "user-vt-applicant",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &currentNode.ID,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert viewer-task instance")

	// The concluded traversal of the prior node, then the active one.
	_, err = s.db.NewInsert().Model(&approval.NodeVisit{
		TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: priorNode.ID,
		Sequence: 1, Status: approval.NodeVisitPassed,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert concluded prior visit")

	task := &approval.Task{
		TenantID:   inst.TenantID,
		InstanceID: inst.ID,
		NodeID:     currentNode.ID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, currentNode.ID).ID,
		AssigneeID: "user-vt",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should insert pending task")

	s.Run("PackagesNodeConfig", func() {
		detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
			InstanceID: inst.ID,
			UserID:     "user-vt",
		})
		s.Require().NoError(err, "Assignee should load the detail")
		s.Require().NotNil(detail.MyTask, "Pending assignee should get a viewer task context")
		s.Assert().Equal(task.ID, detail.MyTask.TaskID, "Viewer task should target the pending task")
		s.Assert().Equal(currentNode.ID, detail.MyTask.NodeID, "Viewer task should carry the task's node")
		s.Assert().True(detail.MyTask.IsOpinionRequired, "Opinion requirement should mirror the node config")
		s.Assert().Equal(
			[]approval.AddAssigneeType{approval.AddAssigneeBefore, approval.AddAssigneeParallel},
			detail.MyTask.AddAssigneeTypes,
			"Add-assignee positions should mirror the node config",
		)
	})

	s.Run("RollbackTargetsFollowVisitTrail", func() {
		detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
			InstanceID: inst.ID,
			UserID:     "user-vt",
		})
		s.Require().NoError(err, "Assignee should load the detail")
		s.Require().NotNil(detail.MyTask, "Pending assignee should get a viewer task context")
		s.Require().Len(detail.MyTask.RollbackTargets, 1, "Only the concluded prior node should be offered")
		s.Assert().Equal(priorNode.ID, detail.MyTask.RollbackTargets[0].NodeID, "Rollback target should be the traversed prior node")
		s.Assert().Equal("Prior Node", detail.MyTask.RollbackTargets[0].Name, "Rollback target should carry the node name")
	})

	s.Run("NilWithoutPendingTask", func() {
		detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
			InstanceID: inst.ID,
			UserID:     "user-vt-applicant",
		})
		s.Require().NoError(err, "Applicant should load the detail")
		s.Assert().Nil(detail.MyTask, "A viewer without a pending task gets no task context")
	})
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
