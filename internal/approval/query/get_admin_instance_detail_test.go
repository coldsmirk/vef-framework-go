package query_test

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetAdminInstanceDetailTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetAdminInstanceDetailTestSuite tests the GetAdminInstanceDetailHandler.
type GetAdminInstanceDetailTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.GetAdminInstanceDetailHandler

	instanceID string
	formSchema json.RawMessage
}

func (s *GetAdminInstanceDetailTestSuite) SetupSuite() {
	s.handler = query.NewGetAdminInstanceDetailHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "adid", 0)

	// Stamp host-owned labels on the flow; the admin detail must surface them
	// beside the other flow-identity fields.
	_, labelsErr := s.db.NewUpdate().Model((*approval.Flow)(nil)).
		Set("labels", map[string]string{"app": "erp"}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(fix.FlowID) }).
		Exec(s.ctx)
	s.Require().NoError(labelsErr, "Should set labels on the fixture flow")

	// Pin form + flow schema on the instance's version: the host designer
	// document must pass through verbatim, and the flow schema feeds a React
	// Flow–ready graph (positions + edges). The graph node ids match the
	// flow-node Keys created below.
	s.formSchema = json.RawMessage(`{"version":2,"presentations":{"pc":{"children":[` +
		`{"id":"F1","type":"textarea","key":"reason","label":"Reason"}]}}}`)
	flowSchema := &approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Position: approval.Position{X: 0, Y: 0}},
			{ID: "approval-1", Kind: approval.NodeApproval, Position: approval.Position{X: 0, Y: 100}},
			{ID: "end-1", Kind: approval.NodeEnd, Position: approval.Position{X: 0, Y: 200}},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "e1", Source: "start-1", Target: "approval-1"},
			{ID: "e2", Source: "approval-1", Target: "end-1"},
		},
	}
	_, err := s.db.NewUpdate().Model((*approval.FlowVersion)(nil)).
		Set("form_schema", s.formSchema).
		Set("flow_schema", flowSchema).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(fix.VersionID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set form + flow schema on version")

	// Create flow nodes.
	nodes := []approval.FlowNode{
		{FlowVersionID: fix.VersionID, Key: "start-1", Kind: approval.NodeStart, Name: "Start"},
		{FlowVersionID: fix.VersionID, Key: "approval-1", Kind: approval.NodeApproval, Name: "Approval"},
		{FlowVersionID: fix.VersionID, Key: "end-1", Kind: approval.NodeEnd, Name: "End"},
	}
	for i := range nodes {
		_, err := s.db.NewInsert().Model(&nodes[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert node")
	}

	// Create instance.
	department := "Finance"
	instance := &approval.Instance{
		TenantID:                "default",
		FlowID:                  fix.FlowID,
		FlowVersionID:           fix.VersionID,
		Title:                   "Admin Detail Test",
		InstanceNo:              "ADID-001",
		ApplicantID:             "user-1",
		ApplicantDepartmentName: &department,
		Status:                  approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")
	s.instanceID = instance.ID

	// Record the visit trail the engine would have written: start passed,
	// approval currently open.
	visits := []approval.NodeVisit{
		{TenantID: "default", InstanceID: instance.ID, NodeID: nodes[0].ID, Sequence: 1, Status: approval.NodeVisitPassed},
		{TenantID: "default", InstanceID: instance.ID, NodeID: nodes[1].ID, Sequence: 2, Status: approval.NodeVisitActive},
	}
	for i := range visits {
		_, err := s.db.NewInsert().Model(&visits[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert node visit")
	}

	// Create tasks bound to the approval visit.
	tasks := []approval.Task{
		{TenantID: "default", InstanceID: instance.ID, NodeID: nodes[1].ID, VisitID: visits[1].ID, AssigneeID: "user-2", SortOrder: 1, Status: approval.TaskPending},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	// Create action logs.
	rollbackTarget := nodes[0].ID

	logs := []approval.ActionLog{
		{InstanceID: instance.ID, Action: approval.ActionSubmit, OperatorID: "user-1", OperatorName: "Applicant"},
		{InstanceID: instance.ID, Action: approval.ActionRollback, OperatorID: "user-2", OperatorName: "Approver", NodeID: &nodes[1].ID, RollbackToNodeID: &rollbackTarget},
	}
	for i := range logs {
		_, err := s.db.NewInsert().Model(&logs[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert action log")
	}
}

func (s *GetAdminInstanceDetailTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetAdminInstanceDetailTestSuite) TestGetDetailSuccess() {
	detail, err := s.handler.Handle(s.ctx, query.GetAdminInstanceDetailQuery{
		InstanceID: s.instanceID,
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "Should get admin instance detail without error")
	s.Require().NotNil(detail, "TestGetDetailSuccess should return a non-nil value")

	s.Assert().Equal(s.instanceID, detail.Instance.InstanceID, "Should return correct instance")
	s.Assert().Equal("Admin Detail Test", detail.Instance.Title, "Should return correct title")
	s.Assert().Equal("default", detail.Instance.TenantID, "Should include tenant ID")
	s.Assert().Equal(map[string]string{"app": "erp"}, detail.Instance.Labels, "Detail should surface the flow's labels")

	// The flow graph is a React Flow–ready projection: 3 nodes + 2 edges with
	// positions, and the approval node reporting its open visit as active.
	s.Require().Len(detail.FlowGraph.Nodes, 3, "Graph should carry all 3 nodes")
	s.Require().Len(detail.FlowGraph.Edges, 2, "Graph should carry both edges")

	byKey := make(map[string]approval.FlowGraphNode, len(detail.FlowGraph.Nodes))
	for _, n := range detail.FlowGraph.Nodes {
		byKey[n.ID] = n
	}

	approvalNode, ok := byKey["approval-1"]
	s.Require().True(ok, "Graph should contain the approval node keyed by its flow-node key")
	s.Assert().Equal(approval.NodeProgressActive, approvalNode.Data.Status, "Approval node with an open visit should be active")
	s.Assert().Equal(float64(100), approvalNode.Position.Y, "Approval node should carry its designed position")
	s.Require().Len(approvalNode.Data.Participants, 1, "Approval node should list its assignee")
	s.Assert().Equal("user-2", approvalNode.Data.Participants[0].User.ID, "Participant should be the node's assignee")

	// The verbatim designer document and the applicant snapshot must ship with
	// the admin detail too.
	s.Require().NotEmpty(detail.FormSchema, "Detail should carry the version's form schema")
	s.Assert().JSONEq(string(s.formSchema), string(detail.FormSchema), "Form schema should pass through verbatim")
	s.Assert().Equal("user-1", detail.Instance.Applicant.ID, "Applicant snapshot should carry the id")
	s.Require().NotNil(detail.Instance.Applicant.DepartmentName, "Applicant snapshot should carry the department")
	s.Assert().Equal("Finance", *detail.Instance.Applicant.DepartmentName, "Applicant department should pass through")

	// The timeline is the visit trail: start (with the submit activity zipped
	// onto it) then the open approval entry carrying the rollback activity.
	s.Require().Len(detail.Timeline, 2, "Timeline should carry the start and approval entries (end unvisited)")

	start := detail.Timeline[0]
	s.Assert().Equal(approval.TimelineEntryStart, start.Kind, "First entry should be the start node")
	s.Assert().Equal(approval.NodeVisitPassed, start.Status, "Start entry should report its passed visit")
	s.Require().Len(start.Activities, 1, "Start entry should carry the submit activity")
	s.Assert().Equal(string(approval.ActionSubmit), start.Activities[0].Action, "Start activity should be the submission")
	s.Assert().Equal("Applicant", start.Activities[0].Operator.Name, "Submit activity should name the applicant")

	entry := detail.Timeline[1]
	s.Assert().Equal(approval.TimelineEntryApproval, entry.Kind, "Second entry should be the approval node")
	s.Assert().Equal(approval.NodeVisitActive, entry.Status, "Approval entry should be the open visit")
	s.Require().Len(entry.Participants, 1, "Approval entry should list its participant")
	s.Assert().Equal("user-2", entry.Participants[0].User.ID, "Participant should be the assignee")

	var rollback *approval.Activity

	for i := range entry.Activities {
		if entry.Activities[i].Action == string(approval.ActionRollback) {
			rollback = &entry.Activities[i]
		}
	}

	s.Require().NotNil(rollback, "Approval entry should carry the rollback activity")
	s.Require().NotNil(rollback.RollbackToNodeID, "Rollback activity should carry the target node id")
	s.Require().NotNil(rollback.RollbackToNodeName, "Rollback activity should resolve the target node name")
	s.Assert().Equal("Start", *rollback.RollbackToNodeName, "Rollback target name should resolve from the flow nodes")
}

func (s *GetAdminInstanceDetailTestSuite) TestNotFound() {
	_, err := s.handler.Handle(s.ctx, query.GetAdminInstanceDetailQuery{
		InstanceID: "non-existent-instance",
		Caller:     approval.SystemCaller,
	})
	s.Require().ErrorIs(err, shared.ErrInstanceNotFound, "Should return instance-not-found error")
}
