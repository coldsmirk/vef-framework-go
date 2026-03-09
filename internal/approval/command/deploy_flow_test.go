package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &DeployFlowTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// DeployFlowTestSuite tests the DeployFlowHandler.
type DeployFlowTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.DeployFlowHandler
	flowID  string
}

func (s *DeployFlowTestSuite) SetupSuite() {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "deploy-test",
		Name:     "Deploy Test Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "deploy-test-flow",
		Name:                   "Deploy Test Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
		CurrentVersion:         0,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow")

	s.flowID = flow.ID
	s.handler = command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService())
}

func (s *DeployFlowTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.FlowEdge)(nil),
		(*approval.FlowNodeCC)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
	)
}

func (s *DeployFlowTestSuite) TearDownSuite() {
	_, err := s.db.NewDelete().
		Model((*approval.Flow)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean flows")

	_, err = s.db.NewDelete().
		Model((*approval.FlowCategory)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean flow categories")
}

func (s *DeployFlowTestSuite) TestDeploySuccess() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow without error")
	s.Require().NotNil(result, "Should return created version")

	s.Assert().NotEmpty(result.ID, "Should generate version ID")
	s.Assert().Equal(s.flowID, result.FlowID, "Should set FlowID")
	s.Assert().Equal(1, result.Version, "Should set Version to CurrentVersion+1")
	s.Assert().Equal(approval.VersionDraft, result.Status, "Should set Status to draft")
	s.Assert().NotNil(result.FlowSchema, "Should set FlowSchema")

	// Verify version in DB
	var version approval.FlowVersion

	version.ID = result.ID
	err = s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find version in DB")
	s.Assert().Equal(s.flowID, version.FlowID, "DB record should have correct FlowID")

	// Verify nodes created
	var nodes []approval.FlowNode

	err = s.db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query nodes")
	s.Assert().Len(nodes, 2, "Should insert two nodes (start + end)")

	// Verify edge created
	var edges []approval.FlowEdge

	err = s.db.NewSelect().
		Model(&edges).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query edges")
	s.Assert().Len(edges, 1, "Should insert one edge")
}

func (s *DeployFlowTestSuite) TestDeployWithDescription() {
	desc := "版本描述"
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		Description:    &desc,
		FlowDefinition: simpleFlowDef(),
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow without error")
	s.Require().NotNil(result.Description, "Should have Description")
	s.Assert().Equal("版本描述", *result.Description, "Should set Description")

	// Verify in DB
	var version approval.FlowVersion

	version.ID = result.ID
	err = s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find version in DB")
	s.Require().NotNil(version.Description, "DB should have Description")
	s.Assert().Equal("版本描述", *version.Description, "DB should persist Description")
}

func (s *DeployFlowTestSuite) TestDeployFlowNotFound() {
	cmd := command.DeployFlowCmd{
		FlowID:         "non-existent-flow-id",
		FlowDefinition: simpleFlowDef(),
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should fail for non-existent flow")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return ErrFlowNotFound")
}

func (s *DeployFlowTestSuite) TestDeployInvalidFlowDesign() {
	cmd := command.DeployFlowCmd{
		FlowID: s.flowID,
		FlowDefinition: approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{ID: "orphan-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
					BaseNodeData: approval.BaseNodeData{Name: "审批"},
				})},
			},
		},
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should fail for invalid flow design")
	s.Assert().ErrorIs(err, shared.ErrInvalidFlowDesign, "Should return ErrInvalidFlowDesign")
}

func (s *DeployFlowTestSuite) TestDeployInvalidAddAssigneeTypeInNodeData() {
	cmd := command.DeployFlowCmd{
		FlowID: s.flowID,
		FlowDefinition: approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
				{ID: "approval-1", Kind: approval.NodeApproval, Data: []byte(`{"name":"审批","isAddAssigneeAllowed":true,"addAssigneeTypes":["before","invalid"]}`)},
				{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
			},
			Edges: []approval.EdgeDefinition{
				{ID: "edge-1", Source: "start-1", Target: "approval-1"},
				{ID: "edge-2", Source: "approval-1", Target: "end-1"},
			},
		},
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should fail for invalid add assignee type in node data")
	s.Assert().ErrorContains(err, "invalid AddAssigneeType", "Should surface invalid add assignee type")
}

func (s *DeployFlowTestSuite) TestDeployWithAssigneesAndCCs() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: approvalFlowDef(),
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow with assignees and CCs")

	// Find the approval node
	var nodes []approval.FlowNode

	err = s.db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID).
				Equals("kind", approval.NodeApproval)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query approval nodes")
	s.Require().Len(nodes, 1, "Should have one approval node")

	approvalNodeID := nodes[0].ID

	// Verify assignees
	var assignees []approval.FlowNodeAssignee

	err = s.db.NewSelect().
		Model(&assignees).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", approvalNodeID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query assignees")
	s.Require().Len(assignees, 1, "Should insert one assignee")
	s.Assert().Equal(approval.AssigneeUser, assignees[0].Kind, "Should set assignee kind")
	s.Assert().Equal([]string{"user-1", "user-2"}, assignees[0].IDs, "Should set assignee IDs")
	s.Assert().Equal(1, assignees[0].SortOrder, "Should set assignee sort order")

	// Verify CCs
	var ccs []approval.FlowNodeCC

	err = s.db.NewSelect().
		Model(&ccs).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", approvalNodeID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query CCs")
	s.Require().Len(ccs, 1, "Should insert one CC")
	s.Assert().Equal(approval.CCUser, ccs[0].Kind, "Should set CC kind")
	s.Assert().Equal([]string{"cc-user-1"}, ccs[0].IDs, "Should set CC IDs")
	s.Assert().Equal(approval.CCTimingAlways, ccs[0].Timing, "Should set CC timing")
}

func (s *DeployFlowTestSuite) TestDeployEdgesWithNodeKeys() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow")

	var edges []approval.FlowEdge

	err = s.db.NewSelect().
		Model(&edges).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query edges")
	s.Require().Len(edges, 1, "Should have one edge")

	s.Assert().Equal("start-1", edges[0].SourceNodeKey, "Should set SourceNodeKey")
	s.Assert().Equal("end-1", edges[0].TargetNodeKey, "Should set TargetNodeKey")
	s.Assert().NotEmpty(edges[0].SourceNodeID, "Should set SourceNodeID")
	s.Assert().NotEmpty(edges[0].TargetNodeID, "Should set TargetNodeID")
}

func (s *DeployFlowTestSuite) TestDeployDoesNotUpdateFlowCurrentVersion() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow")

	// Verify flow's CurrentVersion is unchanged (still 0)
	var flow approval.Flow

	flow.ID = s.flowID
	err = s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find flow")
	s.Assert().Equal(0, flow.CurrentVersion, "Deploy should not update flow CurrentVersion")
}
