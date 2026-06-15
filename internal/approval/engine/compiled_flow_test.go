package engine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// deleteEngineFlowData removes flow-definition rows seeded by the
// compiled-flow suite, in FK order.
func deleteEngineFlowData(ctx context.Context, db orm.DB) {
	for _, model := range []any{
		(*approval.FlowEdge)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
		(*approval.Flow)(nil),
		(*approval.FlowCategory)(nil),
	} {
		_, _ = db.NewDelete().Model(model).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	}
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &CompiledFlowTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// --- FindOutgoing (pure, no DB) ---

// TestFindOutgoing exercises the in-memory edge-selection logic that the
// production traversal path always uses (FX wires a FlowCache unconditionally).
func TestFindOutgoing(t *testing.T) {
	handle := func(s string) *string { return &s }

	t.Run("NilBranchUniqueEdge", func(t *testing.T) {
		cf := &engine.CompiledFlow{
			EdgesBySource: map[string][]*approval.FlowEdge{
				"n1": {{TargetNodeID: "n2"}},
			},
		}

		edge, err := cf.FindOutgoing("n1", nil)
		require.NoError(t, err, "Unique unguarded out-edge should resolve")
		assert.Equal(t, "n2", edge.TargetNodeID, "Should return the single out-edge target")
	})

	t.Run("NilBranchAmbiguousEdges", func(t *testing.T) {
		cf := &engine.CompiledFlow{
			EdgesBySource: map[string][]*approval.FlowEdge{
				"n1": {{TargetNodeID: "n2"}, {TargetNodeID: "n3"}},
			},
		}

		_, err := cf.FindOutgoing("n1", nil)
		require.Error(t, err, "Two unguarded edges from one node is ambiguous")
		assert.Contains(t, err.Error(), "ambiguous", "Error should describe the ambiguity")
	})

	t.Run("BranchMatchesSourceHandle", func(t *testing.T) {
		cf := &engine.CompiledFlow{
			EdgesBySource: map[string][]*approval.FlowEdge{
				"n1": {
					{TargetNodeID: "yes", SourceHandle: handle("b1")},
					{TargetNodeID: "no", SourceHandle: handle("b2")},
				},
			},
		}

		edge, err := cf.FindOutgoing("n1", handle("b1"))
		require.NoError(t, err, "Edge with the matching source handle should resolve")
		assert.Equal(t, "yes", edge.TargetNodeID, "Should pick the edge whose SourceHandle matches the branch")
	})

	t.Run("NoEdgesForSource", func(t *testing.T) {
		cf := &engine.CompiledFlow{EdgesBySource: map[string][]*approval.FlowEdge{}}

		_, err := cf.FindOutgoing("missing", nil)
		assert.ErrorIs(t, err, engine.ErrNoMatchingEdge, "A node with no out-edges returns ErrNoMatchingEdge")
	})

	t.Run("BranchWithNoMatchingHandle", func(t *testing.T) {
		cf := &engine.CompiledFlow{
			EdgesBySource: map[string][]*approval.FlowEdge{
				"n1": {{TargetNodeID: "n2", SourceHandle: handle("b1")}},
			},
		}

		_, err := cf.FindOutgoing("n1", handle("nope"))
		assert.ErrorIs(t, err, engine.ErrNoMatchingEdge, "No edge matching the branch handle returns ErrNoMatchingEdge")
	})

	t.Run("BranchAmbiguousWhenHandlesCollide", func(t *testing.T) {
		cf := &engine.CompiledFlow{
			EdgesBySource: map[string][]*approval.FlowEdge{
				"n1": {
					{TargetNodeID: "a", SourceHandle: handle("b1")},
					{TargetNodeID: "b", SourceHandle: handle("b1")},
				},
			},
		}

		_, err := cf.FindOutgoing("n1", handle("b1"))
		require.Error(t, err, "Two edges sharing the same source handle is ambiguous")
		assert.Contains(t, err.Error(), "ambiguous", "Error should describe the ambiguity")
	})
}

// --- FlowCache.compile / Get (DB-backed) ---

// CompiledFlowTestSuite covers the production compile path: FlowCache.Get
// loading nodes/edges from the database and building a CompiledFlow.
type CompiledFlowTestSuite struct {
	suite.Suite

	ctx context.Context
	db  orm.DB
}

func (s *CompiledFlowTestSuite) TearDownTest() {
	deleteEngineFlowData(s.ctx, s.db)
}

func (s *CompiledFlowTestSuite) TearDownSuite() {
	deleteEngineFlowData(s.ctx, s.db)
}

func (s *CompiledFlowTestSuite) newCache() *engine.FlowCache {
	return engine.NewFlowCache(s.db, cache.NewMemory[*engine.CompiledFlow]())
}

// seedVersion inserts a category → flow → version chain and returns the
// version ID. Nodes/edges are seeded per-test against this version.
func (s *CompiledFlowTestSuite) seedVersion(code string) string {
	cat := &approval.FlowCategory{TenantID: "default", Code: code + "-cat", Name: "Cat"}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "should insert category")

	flow := &approval.Flow{TenantID: "default", CategoryID: cat.ID, Code: code, Name: "Flow", BindingMode: approval.BindingStandalone}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "should insert flow")

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionPublished}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "should insert version")

	return version.ID
}

func (s *CompiledFlowTestSuite) TestCompile() {
	s.Run("NoNodesReturnsErrFlowNoNodes", func() {
		defer deleteEngineFlowData(s.ctx, s.db)

		versionID := s.seedVersion("cf-empty")

		_, err := s.newCache().Get(s.ctx, versionID)
		s.Require().ErrorIs(err, engine.ErrFlowNoNodes, "A version with no nodes must fail to compile")
	})

	s.Run("IndexesNodesStartAndEdges", func() {
		defer deleteEngineFlowData(s.ctx, s.db)

		versionID := s.seedVersion("cf-ok")

		start := &approval.FlowNode{FlowVersionID: versionID, Key: "start", Kind: approval.NodeStart, Name: "Start"}
		mid := &approval.FlowNode{FlowVersionID: versionID, Key: "mid", Kind: approval.NodeApproval, Name: "Mid"}

		end := &approval.FlowNode{FlowVersionID: versionID, Key: "end", Kind: approval.NodeEnd, Name: "End"}
		for _, n := range []*approval.FlowNode{start, mid, end} {
			_, err := s.db.NewInsert().Model(n).Exec(s.ctx)
			s.Require().NoError(err, "should insert node %s", n.Key)
		}

		for _, e := range []*approval.FlowEdge{
			{FlowVersionID: versionID, Key: "e1", SourceNodeID: start.ID, TargetNodeID: mid.ID},
			{FlowVersionID: versionID, Key: "e2", SourceNodeID: mid.ID, TargetNodeID: end.ID},
		} {
			_, err := s.db.NewInsert().Model(e).Exec(s.ctx)
			s.Require().NoError(err, "should insert edge %s", e.Key)
		}

		compiled, err := s.newCache().Get(s.ctx, versionID)
		s.Require().NoError(err, "Compile should succeed")
		s.Assert().Len(compiled.Nodes, 3, "All nodes should be indexed by ID")
		s.Require().NotNil(compiled.StartNode, "Start node should be identified")
		s.Assert().Equal(start.ID, compiled.StartNode.ID, "StartNode should point at the start-kind node")
		s.Assert().Equal(versionID, compiled.FlowVersionID, "Compiled flow should record its version key")
		s.Assert().Len(compiled.EdgesBySource[start.ID], 1, "Start node should have one outgoing edge")
		s.Assert().Len(compiled.EdgesBySource[mid.ID], 1, "Mid node should have one outgoing edge")

		// FindOutgoing over the compiled (DB-loaded) flow resolves the edge.
		edge, err := compiled.FindOutgoing(start.ID, nil)
		s.Require().NoError(err, "Outgoing from start should resolve over the compiled flow")
		s.Assert().Equal(mid.ID, edge.TargetNodeID, "Start should advance to the mid node")
	})

	s.Run("GetCachesAcrossCalls", func() {
		defer deleteEngineFlowData(s.ctx, s.db)

		versionID := s.seedVersion("cf-cache")
		start := &approval.FlowNode{FlowVersionID: versionID, Key: "start", Kind: approval.NodeStart, Name: "Start"}
		_, err := s.db.NewInsert().Model(start).Exec(s.ctx)
		s.Require().NoError(err, "should insert start node")

		c := s.newCache()
		first, err := c.Get(s.ctx, versionID)
		s.Require().NoError(err, "First compile should succeed")

		// Delete the node; a cache hit must still return the first compilation
		// rather than recompiling (and failing with ErrFlowNoNodes).
		_, err = s.db.NewDelete().Model((*approval.FlowNode)(nil)).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", versionID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "should delete node")

		second, err := c.Get(s.ctx, versionID)
		s.Require().NoError(err, "Cache hit should not recompile")
		s.Assert().Same(first, second, "Repeated Get for the same version should return the cached instance")
	})

	s.Run("InvalidateForcesRecompile", func() {
		defer deleteEngineFlowData(s.ctx, s.db)

		versionID := s.seedVersion("cf-invalidate")
		start := &approval.FlowNode{FlowVersionID: versionID, Key: "start", Kind: approval.NodeStart, Name: "Start"}
		_, err := s.db.NewInsert().Model(start).Exec(s.ctx)
		s.Require().NoError(err, "should insert start node")

		c := s.newCache()
		_, err = c.Get(s.ctx, versionID)
		s.Require().NoError(err, "First compile should succeed")

		s.Require().NoError(c.Invalidate(s.ctx, versionID), "Invalidate should succeed")

		// After invalidation, deleting all nodes makes the recompile fail —
		// proving the cached entry was actually dropped.
		_, err = s.db.NewDelete().Model((*approval.FlowNode)(nil)).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", versionID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "should delete node")

		_, err = c.Get(s.ctx, versionID)
		s.Require().ErrorIs(err, engine.ErrFlowNoNodes, "After Invalidate, Get must recompile and observe the empty version")
	})
}
