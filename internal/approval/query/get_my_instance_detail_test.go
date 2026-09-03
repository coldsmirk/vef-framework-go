package query_test

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
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
	flowID     string
	formSchema json.RawMessage
}

func (s *GetMyInstanceDetailTestSuite) SetupSuite() {
	s.handler = query.NewGetMyInstanceDetailHandler(s.db, service.NewTaskService())

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mid-flow", 1)
	s.nodeID = fix.NodeIDs[0]
	s.flowID = fix.FlowID

	// Stamp host-owned labels on the flow; the detail must surface them
	// beside the other flow-identity fields.
	_, err := s.db.NewUpdate().Model((*approval.Flow)(nil)).
		Set("labels", map[string]string{"app": "crm"}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(fix.FlowID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set labels on the fixture flow")

	// Pin a host form-designer document on the instance's version; the detail
	// must return it verbatim — the framework never interprets it.
	s.formSchema = json.RawMessage(`{"version":2,"presentations":{"pc":{"children":[` +
		`{"id":"F1","type":"textarea","key":"reason","label":"Reason"},` +
		`{"id":"F2","type":"number","key":"days","label":"Days"}]}}}`)
	_, err = s.db.NewUpdate().Model((*approval.FlowVersion)(nil)).
		Set("form_schema", s.formSchema).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(fix.VersionID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set form schema on version")

	department := "Engineering"
	inst := &approval.Instance{
		TenantID: "t1", FlowID: fix.FlowID, FlowCode: "mid-flow", FlowVersionID: fix.VersionID,
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
	s.Assert().Equal(s.flowID, detail.Instance.FlowID, "Detail should carry the instance's flow id")
	s.Assert().Equal("mid-flow", detail.Instance.FlowCode, "Detail should carry the flow code snapshot")
	s.Assert().Equal(map[string]string{"app": "crm"}, detail.Instance.Labels, "Detail should surface the flow's labels")
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

// TestDelegatorAccess covers the viewer whose approval slot a delegation
// handed to someone else. They may watch and nudge — the slot is still theirs —
// but the delegate holds the task, so no decision action is offered.
func (s *GetMyInstanceDetailTestSuite) TestDelegatorAccess() {
	detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: s.instanceID,
		UserID:     "user-deleg",
	})
	s.Require().NoError(err, "Delegator should have access")
	s.Assert().Contains(detail.AvailableActions, "urge",
		"A delegator may urge the delegate holding their slot — mirrors IsUrgeAuthorized")
	s.Assert().NotContains(detail.AvailableActions, "approve",
		"The delegate holds the task, so the delegator is offered no decision action")
	s.Assert().NotContains(detail.AvailableActions, "reject",
		"The delegate holds the task, so the delegator is offered no decision action")
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

// TestParticipantAccess enumerates every way a person becomes involved in an
// instance and pins that each one opens the detail rather than being denied.
// The set is the access contract, so it is asserted exhaustively in one place:
// the applicant, the pending approver, the handler of a handle node, both sides
// of a transfer, both sides of a delegation, an assignee who was removed, and a
// CC recipient — closed off by an outsider who must still be refused.
//
// Each leg's read strength is asserted alongside its access, because the two
// have to move together: IsInstanceParticipant admitting a viewer that
// resolveViewerFieldPermissions does not recognize opens the detail onto a form
// with every field stripped.
func (s *GetMyInstanceDetailTestSuite) TestParticipantAccess() {
	category := &approval.FlowCategory{TenantID: "default", Code: "party-cat", Name: "Participant Category"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category")

	flow := &approval.Flow{
		TenantID: "default", CategoryID: category.ID, Code: "party-flow", Name: "Participant Flow",
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

	// "reason" editable / "secret" hidden on both nodes, so every leg's clamp is
	// observable from the same two keys.
	perms := map[string]approval.Permission{
		"reason": approval.PermissionEditable,
		"secret": approval.PermissionHidden,
	}

	approvalNode := &approval.FlowNode{
		FlowVersionID: version.ID, Key: "party-approve", Kind: approval.NodeApproval,
		Name: "Approval Node", FieldPermissions: perms,
	}
	_, err = s.db.NewInsert().Model(approvalNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert approval node")

	handleNode := &approval.FlowNode{
		FlowVersionID: version.ID, Key: "party-handle", Kind: approval.NodeHandle,
		Name: "Handle Node", FieldPermissions: perms,
	}
	_, err = s.db.NewInsert().Model(handleNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert handle node")

	inst := &approval.Instance{
		TenantID: "default", FlowID: flow.ID, FlowVersionID: version.ID,
		Title: "Participant Instance", InstanceNo: "PARTY-001", ApplicantID: "party-applicant",
		Status: approval.InstanceRunning, CurrentNodeID: &approvalNode.ID,
		FormData: map[string]any{"reason": "please approve", "secret": "confidential"},
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")

	approvalVisit := ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, approvalNode.ID)
	handleVisit := ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, handleNode.ID)

	delegatorID, delegatorName := "party-delegator", "Original Approver"

	// One row per involvement shape. Transfer keeps the original row at
	// "transferred" and inserts the replacement; remove-assignee keeps the row at
	// "removed" — both are how the commands actually leave the table, so the
	// access contract is asserted against real shapes, not invented ones.
	tasks := []approval.Task{
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: approvalNode.ID, VisitID: approvalVisit.ID,
			AssigneeID: "party-approver", SortOrder: 1, Status: approval.TaskPending,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: approvalNode.ID, VisitID: approvalVisit.ID,
			AssigneeID: "party-transferor", SortOrder: 2, Status: approval.TaskTransferred,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: approvalNode.ID, VisitID: approvalVisit.ID,
			AssigneeID: "party-transferee", SortOrder: 2, Status: approval.TaskPending,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: approvalNode.ID, VisitID: approvalVisit.ID,
			AssigneeID: "party-delegate", SortOrder: 3, Status: approval.TaskPending,
			DelegatorID: &delegatorID, DelegatorName: &delegatorName,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: approvalNode.ID, VisitID: approvalVisit.ID,
			AssigneeID: "party-removed", SortOrder: 4, Status: approval.TaskRemoved,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: handleNode.ID, VisitID: handleVisit.ID,
			AssigneeID: "party-handler", SortOrder: 5, Status: approval.TaskPending,
		},
	}
	for i := range tasks {
		_, err = s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task for %s", tasks[i].AssigneeID)
	}

	_, err = s.db.NewInsert().Model(&approval.CCRecord{
		InstanceID: inst.ID, NodeID: &approvalNode.ID, VisitID: &approvalVisit.ID,
		CCUserID: "party-cc", CCUserName: "CC User",
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert CC record")

	// wantReason is each leg's read strength on the node's editable field: full
	// strength only for whoever the write path would accept, which is the holder
	// of a pending task (the applicant would qualify too, but only while the
	// instance is resubmittable — it is running here).
	//
	// wantSecret carries the one asymmetry worth pinning: the node's hidden field
	// is hidden from every node-scoped context, but not from the applicant, whose
	// context comes from the start node and covers the form they submitted
	// themselves — hiding their own answers back from them would be absurd.
	granted := []struct {
		name       string
		userID     string
		wantReason approval.Permission
		wantSecret approval.Permission
		why        string
	}{
		{"Applicant", "party-applicant", approval.PermissionVisible, approval.PermissionVisible, "started the instance and owns the whole form"},
		{"PendingApprover", "party-approver", approval.PermissionEditable, approval.PermissionHidden, "holds the pending task"},
		{"Handler", "party-handler", approval.PermissionEditable, approval.PermissionHidden, "holds the pending task on the handle node"},
		{"Transferor", "party-transferor", approval.PermissionVisible, approval.PermissionHidden, "handed their task on, keeping the concluded row"},
		{"Transferee", "party-transferee", approval.PermissionEditable, approval.PermissionHidden, "received the replacement task"},
		{"Delegator", "party-delegator", approval.PermissionVisible, approval.PermissionHidden, "delegated their slot; the delegate acts"},
		{"Delegate", "party-delegate", approval.PermissionEditable, approval.PermissionHidden, "acts on the delegated task"},
		{"RemovedAssignee", "party-removed", approval.PermissionVisible, approval.PermissionHidden, "was removed but took part"},
		{"CCRecipient", "party-cc", approval.PermissionVisible, approval.PermissionHidden, "received the instance as CC"},
	}

	for _, tc := range granted {
		s.Run(tc.name, func() {
			detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
				InstanceID: inst.ID,
				UserID:     tc.userID,
			})
			s.Require().NoError(err, "%s must reach the detail — %s", tc.name, tc.why)
			s.Assert().Equal(inst.ID, detail.Instance.InstanceID, "%s should get the requested instance", tc.name)

			s.Assert().Equal(tc.wantReason, detail.FieldPermissions["reason"],
				"%s should resolve the expected read strength — %s", tc.name, tc.why)
			s.Assert().Equal(tc.wantSecret, detail.FieldPermissions["secret"],
				"%s should resolve the expected strength on the node's hidden field", tc.name)

			s.Assert().Contains(detail.Instance.FormData, "reason",
				"%s must not be admitted onto a stripped form", tc.name)

			if tc.wantSecret == approval.PermissionHidden {
				s.Assert().NotContains(detail.Instance.FormData, "secret",
					"A hidden field must be stripped for %s", tc.name)
			} else {
				s.Assert().Contains(detail.Instance.FormData, "secret",
					"%s resolves the field visible, so its value must survive stripping", tc.name)
			}
		})
	}

	s.Run("OutsiderStillDenied", func() {
		_, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
			InstanceID: inst.ID,
			UserID:     "party-outsider",
		})
		s.Require().ErrorIs(err, approval.ErrAccessDenied,
			"Widening the participant set must not open the instance to non-participants")
	})
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
		FlowVersionID:           baseInstance.FlowVersionID,
		Key:                     "vt-current",
		Kind:                    approval.NodeApproval,
		Name:                    "Current Node",
		IsOpinionRequired:       true,
		IsAddAssigneeAllowed:    true,
		AddAssigneeTypes:        []approval.AddAssigneeType{approval.AddAssigneeBefore, approval.AddAssigneeParallel},
		IsRollbackAllowed:       true,
		RollbackType:            approval.RollbackAny,
		IsRemoveAssigneeAllowed: true,
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
	priorVisit := &approval.NodeVisit{
		TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: priorNode.ID,
		Sequence: 1, Status: approval.NodeVisitPassed,
	}
	_, err = s.db.NewInsert().Model(priorVisit).Exec(s.ctx)
	s.Require().NoError(err, "Should insert concluded prior visit")

	activeVisitID := ensureActiveVisit(s.T(), s.ctx, s.db, inst.TenantID, inst.ID, currentNode.ID).ID

	task := &approval.Task{
		TenantID:   inst.TenantID,
		InstanceID: inst.ID,
		NodeID:     currentNode.ID,
		VisitID:    activeVisitID,
		AssigneeID: "user-vt",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should insert pending task")

	// Peers across statuses and visits: the pending and waiting peers of the
	// active visit are removable; the finished peer and the pending leftover
	// from the concluded prior visit are not.
	peers := []approval.Task{
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: currentNode.ID,
			VisitID: activeVisitID, AssigneeID: "user-vt-peer", AssigneeName: "Peer",
			SortOrder: 2, Status: approval.TaskPending,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: currentNode.ID,
			VisitID: activeVisitID, AssigneeID: "user-vt-queued", AssigneeName: "Queued",
			SortOrder: 3, Status: approval.TaskWaiting,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: currentNode.ID,
			VisitID: activeVisitID, AssigneeID: "user-vt-done", AssigneeName: "Done",
			SortOrder: 4, Status: approval.TaskApproved,
		},
		{
			TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: priorNode.ID,
			VisitID: priorVisit.ID, AssigneeID: "user-vt-stale", AssigneeName: "Stale",
			SortOrder: 5, Status: approval.TaskPending,
		},
	}
	for i := range peers {
		_, err = s.db.NewInsert().Model(&peers[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert peer task")
	}

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

	s.Run("RemovableAssigneesFollowVisitAndStatus", func() {
		detail, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
			InstanceID: inst.ID,
			UserID:     "user-vt",
		})
		s.Require().NoError(err, "Assignee should load the detail")
		s.Require().NotNil(detail.MyTask, "Pending assignee should get a viewer task context")
		s.Assert().Contains(detail.AvailableActions, "remove_assignee", "Node toggle should offer the remove action")

		s.Require().Len(detail.MyTask.RemovableAssignees, 2, "Only still-actionable peers of the active visit are removable")

		statusByAssignee := make(map[string]string, len(detail.MyTask.RemovableAssignees))
		for _, removable := range detail.MyTask.RemovableAssignees {
			s.Assert().NotEmpty(removable.TaskID, "Removable entry should carry the peer task id")
			statusByAssignee[removable.Assignee.ID] = removable.Status
		}

		s.Assert().Equal(string(approval.TaskPending), statusByAssignee["user-vt-peer"], "Pending peer should be removable")
		s.Assert().Equal(string(approval.TaskWaiting), statusByAssignee["user-vt-queued"], "Waiting peer should be removable")
		s.Assert().NotContains(statusByAssignee, "user-vt", "The viewer's own task is never offered")
		s.Assert().NotContains(statusByAssignee, "user-vt-done", "A finished peer is not removable")
		s.Assert().NotContains(statusByAssignee, "user-vt-stale", "A leftover task from a concluded visit is not removable")
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
	s.Require().ErrorIs(err, approval.ErrAccessDenied, "Should return access denied for non-participant")
}

func (s *GetMyInstanceDetailTestSuite) TestInstanceNotFound() {
	_, err := s.handler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: "non-existent",
		UserID:     "user-a",
	})
	s.Require().ErrorIs(err, approval.ErrInstanceNotFound, "Should return instance not found error")
}
