package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetFlowGraphTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetFlowGraphTestSuite tests the GetFlowGraphHandler.
type GetFlowGraphTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.GetFlowGraphHandler

	flowID    string
	versionID string
}

func (s *GetFlowGraphTestSuite) SetupSuite() {
	s.handler = query.NewGetFlowGraphHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "qfg", 0)
	s.flowID = fix.FlowID
	s.versionID = fix.VersionID

	// Create nodes
	nodes := []approval.FlowNode{
		{FlowVersionID: fix.VersionID, Key: "start-1", Kind: approval.NodeStart, Name: "Start"},
		{FlowVersionID: fix.VersionID, Key: "end-1", Kind: approval.NodeEnd, Name: "End"},
	}
	for i := range nodes {
		_, err := s.db.NewInsert().Model(&nodes[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should not return error")
	}

	// Create edge
	edge := &approval.FlowEdge{
		FlowVersionID: fix.VersionID,
		SourceNodeID:  nodes[0].ID,
		TargetNodeID:  nodes[1].ID,
		SourceNodeKey: "start-1",
		TargetNodeKey: "end-1",
	}
	_, err := s.db.NewInsert().Model(edge).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
}

func (s *GetFlowGraphTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetFlowGraphTestSuite) TestGetGraphSuccess() {
	graph, err := s.handler.Handle(s.ctx, query.GetFlowGraphQuery{FlowID: s.flowID})
	s.Require().NoError(err, "Should get flow graph without error")
	s.Require().NotNil(graph, "Should not be nil")

	s.Assert().Equal(s.flowID, graph.Flow.ID, "Should return correct flow")
	s.Assert().Equal(s.versionID, graph.Version.ID, "Should return correct version")
	s.Assert().Len(graph.Nodes, 2, "Should return 2 nodes")
	s.Assert().Len(graph.Edges, 1, "Should return 1 edge")
}

func (s *GetFlowGraphTestSuite) TestFlowNotFound() {
	_, err := s.handler.Handle(s.ctx, query.GetFlowGraphQuery{FlowID: "non-existent"})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return expected error")
}

func (s *GetFlowGraphTestSuite) TestNoPublishedVersion() {
	// Create a flow without published version
	fix2 := setupQueryFixture(s.T(), s.ctx, s.db, "qfg-novers", 0)
	flow := &approval.Flow{}
	flow.ID = fix2.FlowID
	_ = s.db.NewSelect().Model(flow).WherePK().Scan(s.ctx)

	// Delete the published version so flow has no published version
	_, _ = s.db.NewDelete().Model((*approval.FlowVersion)(nil)).Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_id", fix2.FlowID) }).Exec(s.ctx)

	_, err := s.handler.Handle(s.ctx, query.GetFlowGraphQuery{FlowID: fix2.FlowID})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrNoPublishedVersion, "Should return expected error")
}
