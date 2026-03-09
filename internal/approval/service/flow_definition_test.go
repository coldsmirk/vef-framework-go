package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// --- Test helpers ---

func node(id string, kind approval.NodeKind) approval.NodeDefinition {
	return approval.NodeDefinition{ID: id, Kind: kind}
}

func conditionNode(id string, branches ...approval.ConditionBranch) approval.NodeDefinition {
	data, _ := json.Marshal(&approval.ConditionNodeData{Branches: branches})

	return approval.NodeDefinition{ID: id, Kind: approval.NodeCondition, Data: data}
}

func branch(id string, isDefault bool) approval.ConditionBranch {
	return approval.ConditionBranch{ID: id, IsDefault: isDefault}
}

func edge(id, source, target string) approval.EdgeDefinition {
	return approval.EdgeDefinition{ID: id, Source: source, Target: target}
}

func edgeWithHandle(id, source, target, handle string) approval.EdgeDefinition {
	return approval.EdgeDefinition{ID: id, Source: source, Target: target, SourceHandle: &handle}
}

// minimalFlow returns the simplest valid flow: start → end.
func minimalFlow() *approval.FlowDefinition {
	return &approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			node("start", approval.NodeStart),
			node("end", approval.NodeEnd),
		},
		Edges: []approval.EdgeDefinition{
			edge("e1", "start", "end"),
		},
	}
}

// linearFlow returns: start → approval → end.
func linearFlow() *approval.FlowDefinition {
	return &approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			node("start", approval.NodeStart),
			node("a1", approval.NodeApproval),
			node("end", approval.NodeEnd),
		},
		Edges: []approval.EdgeDefinition{
			edge("e1", "start", "a1"),
			edge("e2", "a1", "end"),
		},
	}
}

// conditionFlow returns: start → condition → [approval, handle] → end.
func conditionFlow() *approval.FlowDefinition {
	return &approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			node("start", approval.NodeStart),
			conditionNode("cond",
				branch("b1", false),
				branch("b2", true),
			),
			node("a1", approval.NodeApproval),
			node("h1", approval.NodeHandle),
			node("end", approval.NodeEnd),
		},
		Edges: []approval.EdgeDefinition{
			edge("e1", "start", "cond"),
			edgeWithHandle("e2", "cond", "a1", "b1"),
			edgeWithHandle("e3", "cond", "h1", "b2"),
			edge("e4", "a1", "end"),
			edge("e5", "h1", "end"),
		},
	}
}

func TestValidateFlowDefinition(t *testing.T) {
	svc := NewFlowDefinitionService()

	// --- Phase 1: Node validation ---

	t.Run("EmptyNodes", func(t *testing.T) {
		err := svc.ValidateFlowDefinition(&approval.FlowDefinition{})
		assert.EqualError(t, err, "flow must have at least one node",
			"Should reject flow with no nodes")
	})

	t.Run("EmptyNodeID", func(t *testing.T) {
		def := minimalFlow()
		def.Nodes[0].ID = ""
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "node ID must not be empty",
			"Should reject node with empty ID")
	})

	t.Run("DuplicateNodeID", func(t *testing.T) {
		def := minimalFlow()
		def.Nodes[1].ID = "start"
		def.Nodes[1].Kind = approval.NodeEnd
		def.Edges[0].Target = "start"
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, `duplicate node ID: "start"`,
			"Should reject duplicate node IDs")
	})

	t.Run("InvalidNodeKind", func(t *testing.T) {
		def := minimalFlow()
		def.Nodes[0].Kind = "invalid"
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `invalid node kind: "invalid"`,
			"Should reject unknown node kind")
	})

	t.Run("NoStartNode", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{edge("e1", "a1", "end")},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "flow must have exactly 1 start node, found 0",
			"Should reject flow without start node")
	})

	t.Run("MultipleStartNodes", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("s1", approval.NodeStart),
				node("s2", approval.NodeStart),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "s1", "end"),
				edge("e2", "s2", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "flow must have exactly 1 start node, found 2",
			"Should reject flow with multiple start nodes")
	})

	t.Run("NoEndNode", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				node("a1", approval.NodeApproval),
			},
			Edges: []approval.EdgeDefinition{edge("e1", "start", "a1")},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "flow must have at least 1 end node, found 0",
			"Should reject flow without end node")
	})

	t.Run("MultipleEndNodes", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b2", true)),
				node("end1", approval.NodeEnd),
				node("end2", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "end1", "b1"),
				edgeWithHandle("e3", "cond", "end2", "b2"),
			},
		}
		require.NoError(t, svc.ValidateFlowDefinition(def),
			"Should accept flow with multiple end nodes")
	})

	t.Run("ConditionNodeInvalidData", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				{ID: "cond", Kind: approval.NodeCondition, Data: []byte(`{invalid json`)},
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{edge("e1", "start", "end")},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `parse node "cond" data`,
			"Should reject condition node with invalid JSON data")
	})

	// --- Phase 2: Edge validation ---

	t.Run("EmptyEdgeID", func(t *testing.T) {
		def := minimalFlow()
		def.Edges[0].ID = ""
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "edge ID must not be empty",
			"Should reject edge with empty ID")
	})

	t.Run("DuplicateEdgeID", func(t *testing.T) {
		def := linearFlow()
		def.Edges[1].ID = "e1"
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, `duplicate edge ID: "e1"`,
			"Should reject duplicate edge IDs")
	})

	t.Run("UnknownSourceNode", func(t *testing.T) {
		def := minimalFlow()
		def.Edges[0].Source = "ghost"
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `edge references unknown source node: edge "e1" references "ghost"`,
			"Should reject edge referencing non-existent source node")
	})

	t.Run("UnknownTargetNode", func(t *testing.T) {
		def := minimalFlow()
		def.Edges[0].Target = "ghost"
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `edge references unknown target node: edge "e1" references "ghost"`,
			"Should reject edge referencing non-existent target node")
	})

	// --- Phase 3: Degree constraints ---

	t.Run("StartHasIncomingEdge", func(t *testing.T) {
		def := linearFlow()
		def.Edges = append(def.Edges, edge("e3", "a1", "start"))
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "start node must not have incoming edges",
			"Should reject start node with incoming edges")
	})

	t.Run("StartNoOutgoingEdge", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				node("end", approval.NodeEnd),
			},
			Edges: nil,
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "start node must have exactly 1 outgoing edge, found 0",
			"Should reject start node without outgoing edge")
	})

	t.Run("StartMultipleOutgoingEdges", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				node("a1", approval.NodeApproval),
				node("a2", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "a1"),
				edge("e2", "start", "a2"),
				edge("e3", "a1", "end"),
				edge("e4", "a2", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, "start node must have exactly 1 outgoing edge, found 2",
			"Should reject start node with multiple outgoing edges")
	})

	t.Run("EndHasOutgoingEdge", func(t *testing.T) {
		def := linearFlow()
		def.Edges = append(def.Edges, edge("e3", "end", "a1"))
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, `end node must not have outgoing edges: "end"`,
			"Should reject end node with outgoing edges")
	})

	t.Run("EndNoIncomingEdge", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "a1"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, `end node must have at least 1 incoming edge: "end"`,
			"Should reject end node without incoming edges")
	})

	// Uses condition node to bypass start's single-outgoing-edge constraint.
	t.Run("RegularNodeNoOutgoingEdge", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b2", true)),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "b1"),
				edgeWithHandle("e3", "cond", "end", "b2"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, `node must have exactly 1 outgoing edge: node "a1" has 0`,
			"Should reject regular node without outgoing edge")
	})

	t.Run("RegularNodeMultipleOutgoingEdges", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				node("a1", approval.NodeApproval),
				node("h1", approval.NodeHandle),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "a1"),
				edge("e2", "a1", "h1"),
				edge("e3", "a1", "end"),
				edge("e4", "h1", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.EqualError(t, err, `node must have exactly 1 outgoing edge: node "a1" has 2`,
			"Should reject regular node with multiple outgoing edges")
	})

	t.Run("NonConditionNodeWithSourceHandle", func(t *testing.T) {
		def := linearFlow()
		def.Edges[1].SourceHandle = new("some-handle")
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `must not have sourceHandle`,
			"Should reject sourceHandle on non-condition node's outgoing edge")
	})

	t.Run("ConditionLessThan2Branches", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", true)),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "end", "b1"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, "condition node must have at least 2 branches: node \"cond\" has 1",
			"Should reject condition node with less than 2 branches")
	})

	t.Run("ConditionNoDefaultBranch", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b2", false)),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "b1"),
				edgeWithHandle("e3", "cond", "end", "b2"),
				edge("e4", "a1", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, "condition node must have exactly 1 default branch: node \"cond\" has 0",
			"Should reject condition node without default branch")
	})

	t.Run("ConditionMultipleDefaultBranches", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", true), branch("b2", true)),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "b1"),
				edgeWithHandle("e3", "cond", "end", "b2"),
				edge("e4", "a1", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, "condition node must have exactly 1 default branch: node \"cond\" has 2",
			"Should reject condition node with multiple default branches")
	})

	t.Run("ConditionDuplicateBranchID", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b1", true)),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "end", "b1"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `condition node has duplicate branch ID: node "cond" branch "b1"`,
			"Should reject condition node with duplicate branch IDs")
	})

	t.Run("ConditionEdgeMissingSourceHandle", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b2", true)),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edge("e2", "cond", "a1"),
				edgeWithHandle("e3", "cond", "end", "b2"),
				edge("e4", "a1", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, "must have a sourceHandle",
			"Should reject condition outgoing edge without sourceHandle")
	})

	t.Run("ConditionEdgeUnknownSourceHandle", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b2", true)),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "unknown"),
				edgeWithHandle("e3", "cond", "end", "b2"),
				edge("e4", "a1", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `edge has unknown sourceHandle: node "cond" edge "e2" handle "unknown"`,
			"Should reject condition edge with unknown sourceHandle")
	})

	t.Run("ConditionDuplicateEdgeForHandle", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b2", true)),
				node("a1", approval.NodeApproval),
				node("a2", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "b1"),
				edgeWithHandle("e3", "cond", "a2", "b1"),
				edgeWithHandle("e4", "cond", "end", "b2"),
				edge("e5", "a1", "end"),
				edge("e6", "a2", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `duplicate outgoing edge for handle: node "cond" handle "b1"`,
			"Should reject condition node with duplicate edge for same handle")
	})

	t.Run("ConditionBranchMissingEdge", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond", branch("b1", false), branch("b2", true)),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "b1"),
				edge("e3", "a1", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		assert.ErrorContains(t, err, `branch has no outgoing edge: node "cond" branch "b2"`,
			"Should reject condition node with branch missing outgoing edge")
	})

	// --- Phase 4: Topology ---

	t.Run("CycleDetection", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond",
					branch("b1", false),
					branch("b2", true),
				),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "b1"),
				edgeWithHandle("e3", "cond", "end", "b2"),
				edge("e4", "a1", "cond"), // back-edge creates cycle
			},
		}
		err := svc.ValidateFlowDefinition(def)
		require.Error(t, err, "Should return an error for cyclic flow")
		assert.ErrorContains(t, err, "cycle", "Error should mention cycle")
	})

	t.Run("UnreachableNode", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "end"),
				edge("e2", "a1", "end"),
			},
		}
		err := svc.ValidateFlowDefinition(def)
		require.Error(t, err, "Should return an error for unreachable node")
		assert.ErrorContains(t, err, `not reachable from start node`,
			"Error should indicate unreachable node")
	})

	// --- Valid flows ---

	t.Run("MinimalFlow", func(t *testing.T) {
		require.NoError(t, svc.ValidateFlowDefinition(minimalFlow()),
			"Minimal flow (start → end) should be valid")
	})

	t.Run("LinearFlow", func(t *testing.T) {
		require.NoError(t, svc.ValidateFlowDefinition(linearFlow()),
			"Linear flow (start → approval → end) should be valid")
	})

	t.Run("ConditionFlow", func(t *testing.T) {
		require.NoError(t, svc.ValidateFlowDefinition(conditionFlow()),
			"Condition flow with two branches should be valid")
	})

	t.Run("ComplexFlow", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				node("a1", approval.NodeApproval),
				conditionNode("cond",
					branch("high", false),
					branch("low", true),
				),
				node("a2", approval.NodeApproval),
				node("h1", approval.NodeHandle),
				node("cc1", approval.NodeCC),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "a1"),
				edge("e2", "a1", "cond"),
				edgeWithHandle("e3", "cond", "a2", "high"),
				edgeWithHandle("e4", "cond", "h1", "low"),
				edge("e5", "a2", "cc1"),
				edge("e6", "cc1", "end"),
				edge("e7", "h1", "end"),
			},
		}
		require.NoError(t, svc.ValidateFlowDefinition(def),
			"Complex flow with all node types should be valid")
	})

	t.Run("MultipleConditionNodes", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond1", branch("b1", false), branch("b2", true)),
				conditionNode("cond2", branch("c1", false), branch("c2", true)),
				node("a1", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond1"),
				edgeWithHandle("e2", "cond1", "cond2", "b1"),
				edgeWithHandle("e3", "cond1", "end", "b2"),
				edgeWithHandle("e4", "cond2", "a1", "c1"),
				edgeWithHandle("e5", "cond2", "end", "c2"),
				edge("e6", "a1", "end"),
			},
		}
		require.NoError(t, svc.ValidateFlowDefinition(def),
			"Flow with nested condition nodes should be valid")
	})

	t.Run("ConditionThreeBranches", func(t *testing.T) {
		def := &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				node("start", approval.NodeStart),
				conditionNode("cond",
					branch("b1", false),
					branch("b2", false),
					branch("b3", true),
				),
				node("a1", approval.NodeApproval),
				node("a2", approval.NodeApproval),
				node("end", approval.NodeEnd),
			},
			Edges: []approval.EdgeDefinition{
				edge("e1", "start", "cond"),
				edgeWithHandle("e2", "cond", "a1", "b1"),
				edgeWithHandle("e3", "cond", "a2", "b2"),
				edgeWithHandle("e4", "cond", "end", "b3"),
				edge("e5", "a1", "end"),
				edge("e6", "a2", "end"),
			},
		}
		require.NoError(t, svc.ValidateFlowDefinition(def),
			"Condition node with 3 branches should be valid")
	})
}

// --- Unit tests: detectCycle ---

func TestDetectCycle(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []string
		adjacency map[string][]string
		hasCycle  bool
	}{
		{
			"NoCycleDAG",
			[]string{"a", "b", "c", "d"},
			map[string][]string{"a": {"b", "c"}, "b": {"d"}, "c": {"d"}},
			false,
		},
		{
			"SimpleCycle",
			[]string{"a", "b", "c"},
			map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}},
			true,
		},
		{
			"SelfLoop",
			[]string{"a"},
			map[string][]string{"a": {"a"}},
			true,
		},
		{
			"EmptyGraph",
			nil,
			nil,
			false,
		},
		{
			"SingleNodeNoEdges",
			[]string{"a"},
			map[string][]string{},
			false,
		},
		{
			"DisconnectedNoCycle",
			[]string{"a", "b", "c", "d"},
			map[string][]string{"a": {"b"}, "c": {"d"}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectCycle(tt.nodes, tt.adjacency)
			assert.Equal(t, tt.hasCycle, got,
				"Cycle detection result should match expected")
		})
	}
}

// --- Unit tests: collectReachable ---

func TestCollectReachable(t *testing.T) {
	tests := []struct {
		name      string
		adjacency map[string][]string
		starts    []string
		wantSize  int
		contains  []string
		excludes  []string
	}{
		{
			"LinearGraph",
			map[string][]string{"a": {"b"}, "b": {"c"}},
			[]string{"a"},
			3,
			[]string{"a", "b", "c"},
			nil,
		},
		{
			"DiamondGraph",
			map[string][]string{"a": {"b", "c"}, "b": {"d"}, "c": {"d"}},
			[]string{"a"},
			4,
			[]string{"a", "b", "c", "d"},
			nil,
		},
		{
			"DisconnectedNode",
			map[string][]string{"a": {"b"}},
			[]string{"a"},
			2,
			[]string{"a", "b"},
			[]string{"c"},
		},
		{
			"MultipleStarts",
			map[string][]string{"a": {"c"}, "b": {"c"}, "c": {"d"}},
			[]string{"a", "b"},
			4,
			[]string{"a", "b", "c", "d"},
			nil,
		},
		{
			"SingleNodeNoEdges",
			map[string][]string{},
			[]string{"a"},
			1,
			[]string{"a"},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collectReachable(tt.adjacency, tt.starts...)
			assert.Equal(t, tt.wantSize, result.Size(),
				"Reachable set size should match expected")

			for _, c := range tt.contains {
				assert.True(t, result.Contains(c),
					"Should contain node %q", c)
			}

			for _, e := range tt.excludes {
				assert.False(t, result.Contains(e),
					"Should not contain node %q", e)
			}
		})
	}
}

// --- Unit tests: validateConditionEdges ---

func TestValidateConditionEdges(t *testing.T) {
	tests := []struct {
		name     string
		branches []approval.ConditionBranch
		edges    []approval.EdgeDefinition
		wantErr  string
	}{
		{
			"ValidTwoBranches",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: false},
				{ID: "b2", IsDefault: true},
			},
			[]approval.EdgeDefinition{
				edgeWithHandle("e1", "cond", "a1", "b1"),
				edgeWithHandle("e2", "cond", "a2", "b2"),
			},
			"",
		},
		{
			"ZeroBranches",
			nil,
			nil,
			`condition node must have at least 2 branches: node "cond" has 0`,
		},
		{
			"OneBranch",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: true},
			},
			nil,
			`condition node must have at least 2 branches: node "cond" has 1`,
		},
		{
			"EmptyBranchID",
			[]approval.ConditionBranch{
				{ID: "", IsDefault: false},
				{ID: "b2", IsDefault: true},
			},
			nil,
			"empty ID",
		},
		{
			"DuplicateBranchID",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: false},
				{ID: "b1", IsDefault: true},
			},
			nil,
			`condition node has duplicate branch ID: node "cond" branch "b1"`,
		},
		{
			"NoDefaultBranch",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: false},
				{ID: "b2", IsDefault: false},
			},
			[]approval.EdgeDefinition{
				edgeWithHandle("e1", "cond", "a1", "b1"),
				edgeWithHandle("e2", "cond", "a2", "b2"),
			},
			`condition node must have exactly 1 default branch: node "cond" has 0`,
		},
		{
			"MultipleDefaultBranches",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: true},
				{ID: "b2", IsDefault: true},
			},
			[]approval.EdgeDefinition{
				edgeWithHandle("e1", "cond", "a1", "b1"),
				edgeWithHandle("e2", "cond", "a2", "b2"),
			},
			`condition node must have exactly 1 default branch: node "cond" has 2`,
		},
		{
			"EdgeMissingSourceHandle",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: false},
				{ID: "b2", IsDefault: true},
			},
			[]approval.EdgeDefinition{
				edge("e1", "cond", "a1"),
				edgeWithHandle("e2", "cond", "a2", "b2"),
			},
			"must have a sourceHandle",
		},
		{
			"UnknownSourceHandle",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: false},
				{ID: "b2", IsDefault: true},
			},
			[]approval.EdgeDefinition{
				edgeWithHandle("e1", "cond", "a1", "unknown"),
				edgeWithHandle("e2", "cond", "a2", "b2"),
			},
			`edge has unknown sourceHandle: node "cond" edge "e1" handle "unknown"`,
		},
		{
			"DuplicateEdgeForHandle",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: false},
				{ID: "b2", IsDefault: true},
			},
			[]approval.EdgeDefinition{
				edgeWithHandle("e1", "cond", "a1", "b1"),
				edgeWithHandle("e2", "cond", "a2", "b1"),
				edgeWithHandle("e3", "cond", "a3", "b2"),
			},
			`duplicate outgoing edge for handle: node "cond" handle "b1"`,
		},
		{
			"BranchMissingEdge",
			[]approval.ConditionBranch{
				{ID: "b1", IsDefault: false},
				{ID: "b2", IsDefault: true},
			},
			[]approval.EdgeDefinition{
				edgeWithHandle("e1", "cond", "a1", "b1"),
			},
			`branch has no outgoing edge: node "cond" branch "b2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConditionEdges("cond", tt.branches, tt.edges)
			if tt.wantErr == "" {
				require.NoError(t, err, "Should not return error for valid condition edges")
			} else {
				require.Error(t, err, "Should return a validation error")
				assert.ErrorContains(t, err, tt.wantErr,
					"Error message should contain expected text")
			}
		})
	}
}
