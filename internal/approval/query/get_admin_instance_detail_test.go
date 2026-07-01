package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
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
}

func (s *GetAdminInstanceDetailTestSuite) SetupSuite() {
	s.handler = query.NewGetAdminInstanceDetailHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "adid", 0)

	// Pin form + flow schema on the instance's version so the detail can project
	// the form metadata and a React Flow–ready graph (positions + edges). The
	// graph node ids match the flow-node Keys created below.
	formSchema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "reason", Kind: approval.FieldTextarea, Label: "Reason", SortOrder: 1},
		},
	}
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
		Set("form_schema", formSchema).
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

	// Create tasks.
	tasks := []approval.Task{
		{TenantID: "default", InstanceID: instance.ID, NodeID: nodes[1].ID, AssigneeID: "user-2", SortOrder: 1, Status: approval.TaskPending},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	// Create action logs.
	rollbackTarget := nodes[0].ID

	logs := []approval.ActionLog{
		{InstanceID: instance.ID, Action: approval.ActionSubmit, OperatorID: "user-1", OperatorName: "Applicant", NodeID: &nodes[1].ID},
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
	s.Assert().Len(detail.Tasks, 1, "Should return 1 task")
	s.Assert().Len(detail.ActionLogs, 2, "Should return both action logs")

	// The flow graph is a React Flow–ready projection: 3 nodes + 2 edges with
	// positions, and the approval node marked current (it has a pending task).
	s.Require().Len(detail.FlowGraph.Nodes, 3, "Graph should carry all 3 nodes")
	s.Require().Len(detail.FlowGraph.Edges, 2, "Graph should carry both edges")

	byKey := make(map[string]approval.FlowGraphNode, len(detail.FlowGraph.Nodes))
	for _, n := range detail.FlowGraph.Nodes {
		byKey[n.ID] = n
	}

	approvalNode, ok := byKey["approval-1"]
	s.Require().True(ok, "Graph should contain the approval node keyed by its flow-node key")
	s.Assert().Equal(approval.NodeProgressCurrent, approvalNode.Data.Status, "Approval node with a pending task should be current")
	s.Assert().Equal(float64(100), approvalNode.Position.Y, "Approval node should carry its designed position")
	s.Require().Len(approvalNode.Data.Participants, 1, "Approval node should list its assignee")
	s.Assert().Equal("user-2", approvalNode.Data.Participants[0].UserID, "Participant should be the node's assignee")

	// Form metadata and applicant department must ship with the admin detail too.
	s.Require().NotNil(detail.Instance.FormSchema, "Detail should carry the version's form schema")
	s.Assert().Len(detail.Instance.FormSchema.Fields, 1, "Form schema should carry its field")
	s.Require().NotNil(detail.Instance.ApplicantDepartmentName, "Detail should carry the applicant department")
	s.Assert().Equal("Finance", *detail.Instance.ApplicantDepartmentName, "Applicant department should pass through")

	var rollback *admin.ActionLog

	for i := range detail.ActionLogs {
		if detail.ActionLogs[i].Action == string(approval.ActionRollback) {
			rollback = &detail.ActionLogs[i]
		}
	}

	s.Require().NotNil(rollback, "Detail should include the rollback log")
	s.Require().NotNil(rollback.NodeID, "Rollback log should carry the node id")
	s.Require().NotNil(rollback.RollbackToNodeID, "Rollback log should carry the rollback target node id")
}

func (s *GetAdminInstanceDetailTestSuite) TestNotFound() {
	_, err := s.handler.Handle(s.ctx, query.GetAdminInstanceDetailQuery{
		InstanceID: "non-existent-instance",
		Caller:     approval.SystemCaller,
	})
	s.Require().ErrorIs(err, shared.ErrInstanceNotFound, "Should return instance-not-found error")
}
