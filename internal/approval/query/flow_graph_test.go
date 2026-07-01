package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// flowNode builds an in-memory FlowNode with the given DB id, React Flow key, and kind.
func flowNode(id, key string, kind approval.NodeKind) approval.FlowNode {
	n := approval.FlowNode{Key: key, Kind: kind, Name: key}
	n.ID = id

	return n
}

// task builds an in-memory Task with the given id/node/assignee/status.
func task(id, nodeID, assigneeID string, status approval.TaskStatus) approval.Task {
	t := approval.Task{NodeID: nodeID, AssigneeID: assigneeID, AssigneeName: assigneeID, Status: status}
	t.ID = id

	return t
}

// nodesByKey indexes the graph's nodes by their React Flow id (== node key).
func nodesByKey(g approval.InstanceFlowGraph) map[string]approval.FlowGraphNode {
	m := make(map[string]approval.FlowGraphNode, len(g.Nodes))
	for _, n := range g.Nodes {
		m[n.ID] = n
	}

	return m
}

// linearBundle builds a start → approval → end bundle with a matching React Flow
// schema (positions + edges). Callers set Instance status/current-node, Tasks,
// and ActionLogs per scenario. DB node ids are ns/na/ne; keys are kstart/kappr/kend.
func linearBundle() *instanceDetailBundle {
	return &instanceDetailBundle{
		FlowNodes: []approval.FlowNode{
			flowNode("ns", "kstart", approval.NodeStart),
			flowNode("na", "kappr", approval.NodeApproval),
			flowNode("ne", "kend", approval.NodeEnd),
		},
		FlowSchema: &approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{ID: "kstart", Kind: approval.NodeStart, Position: approval.Position{X: 0, Y: 0}},
				{ID: "kappr", Kind: approval.NodeApproval, Position: approval.Position{X: 0, Y: 100}},
				{ID: "kend", Kind: approval.NodeEnd, Position: approval.Position{X: 0, Y: 200}},
			},
			Edges: []approval.EdgeDefinition{
				{ID: "e1", Source: "kstart", Target: "kappr"},
				{ID: "e2", Source: "kappr", Target: "kend"},
			},
		},
	}
}

func TestBuildInstanceFlowGraph(t *testing.T) {
	t.Run("RunningMarksStartCompletedCurrentPending", func(t *testing.T) {
		b := linearBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Tasks = []approval.Task{task("t1", "na", "u1", approval.TaskPending)}
		// The submit log carries no NodeID (matches production), so the start
		// node must be completed via topology, not log evidence.
		submit := approval.ActionLog{Action: approval.ActionSubmit, OperatorID: "applicant"}
		submit.ID = "l0"
		b.ActionLogs = []approval.ActionLog{submit}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		require.Contains(t, byKey, "kstart", "Graph should contain the start node")
		assert.Equal(t, approval.NodeProgressCompleted, byKey["kstart"].Data.Status, "Start node should be completed once the instance has progressed")
		assert.Equal(t, approval.NodeProgressCurrent, byKey["kappr"].Data.Status, "Approval node with a pending task should be current")
		assert.Equal(t, approval.NodeProgressPending, byKey["kend"].Data.Status, "Unreached end node should be pending")

		require.Len(t, byKey["kappr"].Data.Participants, 1, "Approval node should list its assignee")
		assert.Equal(t, "u1", byKey["kappr"].Data.Participants[0].UserID, "Participant should be the assignee")
		assert.Equal(t, string(approval.TaskPending), byKey["kappr"].Data.Participants[0].Status, "Pending participant status")
		assert.InDelta(t, 100.0, byKey["kappr"].Position.Y, 0, "Node position should come from the schema")
		assert.Equal(t, "na", byKey["kappr"].NodeID, "Node must expose its persistent DB id for rollback targeting and action-log correlation")
	})

	t.Run("FinalMarksAllCompletedIncludingEnd", func(t *testing.T) {
		b := linearBundle()
		b.Instance.Status = approval.InstanceApproved
		// On completion the engine parks CurrentNodeID on the end node.
		b.Instance.CurrentNodeID = new("ne")
		approved := task("t1", "na", "u1", approval.TaskApproved)
		b.Tasks = []approval.Task{approved}
		log := approval.ActionLog{Action: approval.ActionApprove, OperatorID: "u1", NodeID: new("na"), TaskID: new("t1"), Opinion: new("approved-opinion")}
		log.ID = "l1"
		b.ActionLogs = []approval.ActionLog{log}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressCompleted, byKey["kstart"].Data.Status, "Start node completed")
		assert.Equal(t, approval.NodeProgressCompleted, byKey["kappr"].Data.Status, "Approval node with a finished task should be completed, not current")
		assert.Equal(t, approval.NodeProgressCompleted, byKey["kend"].Data.Status, "End node of a finished instance should be completed, not current")

		require.Len(t, byKey["kappr"].Data.Participants, 1, "Approval node should list its assignee")
		require.NotNil(t, byKey["kappr"].Data.Participants[0].Opinion, "Participant should carry the finishing log's opinion")
		assert.Equal(t, "approved-opinion", *byKey["kappr"].Data.Participants[0].Opinion, "Opinion should come from the log that finished this task")
	})

	t.Run("ConditionNodeOnTakenPathIsCompleted", func(t *testing.T) {
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				flowNode("ns", "kstart", approval.NodeStart),
				flowNode("nc", "kcond", approval.NodeCondition),
				flowNode("na", "kappr", approval.NodeApproval),
			},
			FlowSchema: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{
					{ID: "kstart", Kind: approval.NodeStart},
					{ID: "kcond", Kind: approval.NodeCondition},
					{ID: "kappr", Kind: approval.NodeApproval},
				},
				Edges: []approval.EdgeDefinition{
					{ID: "e1", Source: "kstart", Target: "kcond"},
					{ID: "e2", Source: "kcond", Target: "kappr"},
				},
			},
		}
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Tasks = []approval.Task{task("t1", "na", "u1", approval.TaskPending)}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		// The condition node leaves no task or log, but it is an ancestor of the
		// current node, so it must be completed rather than pending.
		assert.Equal(t, approval.NodeProgressCompleted, byKey["kcond"].Data.Status, "Traversed condition node should be completed")
	})

	t.Run("NilSchemaStillBuildsNodesWithoutEdges", func(t *testing.T) {
		b := linearBundle()
		b.FlowSchema = nil
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Tasks = []approval.Task{task("t1", "na", "u1", approval.TaskPending)}

		g := buildInstanceFlowGraph(b)
		byKey := nodesByKey(g)

		require.Len(t, g.Nodes, 3, "Nodes come from the flow-node rows even without a schema")
		assert.Empty(t, g.Edges, "No edges without a schema")
		assert.Equal(t, approval.NodeProgressCompleted, byKey["kstart"].Data.Status, "Start node still completed via the start rule")
		assert.Equal(t, approval.NodeProgressCurrent, byKey["kappr"].Data.Status, "Current node still current")
	})

	t.Run("HandleNodeOmitsApprovalOnlyConfig", func(t *testing.T) {
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				func() approval.FlowNode {
					n := flowNode("nh", "khandle", approval.NodeHandle)
					n.ExecutionType = approval.ExecutionManual
					n.ApprovalMethod = approval.ApprovalParallel
					n.PassRule = approval.PassAll

					return n
				}(),
			},
		}
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("nh")
		b.Tasks = []approval.Task{task("t1", "nh", "u1", approval.TaskPending)}

		data := nodesByKey(buildInstanceFlowGraph(b))["khandle"].Data

		assert.Equal(t, string(approval.ExecutionManual), data.ExecutionType, "Handle node keeps its execution type")
		assert.Empty(t, data.ApprovalMethod, "Handle node must not expose approvalMethod (it does not decide)")
		assert.Empty(t, data.PassRule, "Handle node must not expose passRule")
		assert.Len(t, data.Participants, 1, "Handle node still lists its assignee")
	})

	t.Run("RollbackRedoKeepsDistinctParticipantOpinions", func(t *testing.T) {
		b := linearBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		// Same assignee acted twice on the same node across a rollback: two task
		// rows, two logs. Each participant must show its own pass's opinion.
		b.Tasks = []approval.Task{
			task("t1", "na", "u1", approval.TaskRolledBack),
			task("t2", "na", "u1", approval.TaskApproved),
		}
		log1 := approval.ActionLog{Action: approval.ActionApprove, OperatorID: "u1", NodeID: new("na"), TaskID: new("t1"), Opinion: new("first-pass")}
		log1.ID = "l1"
		log2 := approval.ActionLog{Action: approval.ActionApprove, OperatorID: "u1", NodeID: new("na"), TaskID: new("t2"), Opinion: new("second-pass")}
		log2.ID = "l2"
		b.ActionLogs = []approval.ActionLog{log1, log2}

		parts := nodesByKey(buildInstanceFlowGraph(b))["kappr"].Data.Participants

		require.Len(t, parts, 2, "Both passes should surface as distinct participants")
		require.NotNil(t, parts[0].Opinion, "First pass opinion present")
		require.NotNil(t, parts[1].Opinion, "Second pass opinion present")
		assert.Equal(t, "first-pass", *parts[0].Opinion, "First task's participant keeps the first log's opinion")
		assert.Equal(t, "second-pass", *parts[1].Opinion, "Second task's participant keeps the second log's opinion")
	})

	t.Run("RollbackKeepsDownstreamNodePending", func(t *testing.T) {
		// start -> A -> B -> end. The instance reached B, then B was rolled back to
		// A: current is A (redo pending) while B keeps a rolled_back task and a
		// rollback log. B is downstream of the current node, so it must fall back to
		// pending — never render as completed on stale evidence.
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				flowNode("ns", "kstart", approval.NodeStart),
				flowNode("na", "kA", approval.NodeApproval),
				flowNode("nb", "kB", approval.NodeApproval),
				flowNode("ne", "kend", approval.NodeEnd),
			},
			FlowSchema: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{
					{ID: "kstart", Kind: approval.NodeStart},
					{ID: "kA", Kind: approval.NodeApproval},
					{ID: "kB", Kind: approval.NodeApproval},
					{ID: "kend", Kind: approval.NodeEnd},
				},
				Edges: []approval.EdgeDefinition{
					{ID: "e1", Source: "kstart", Target: "kA"},
					{ID: "e2", Source: "kA", Target: "kB"},
					{ID: "e3", Source: "kB", Target: "kend"},
				},
			},
		}
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Tasks = []approval.Task{
			task("t1", "na", "u1", approval.TaskPending),
			task("t2", "nb", "u2", approval.TaskRolledBack),
		}
		rollback := approval.ActionLog{Action: approval.ActionRollback, OperatorID: "u2", NodeID: new("nb"), RollbackToNodeID: new("na")}
		rollback.ID = "l1"
		b.ActionLogs = []approval.ActionLog{rollback}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressCompleted, byKey["kstart"].Data.Status, "Start node stays completed")
		assert.Equal(t, approval.NodeProgressCurrent, byKey["kA"].Data.Status, "Rolled-back-to node is current again")
		assert.Equal(t, approval.NodeProgressPending, byKey["kB"].Data.Status, "Rolled-back-from node ahead of the current node must be pending, not completed")
		assert.Equal(t, approval.NodeProgressPending, byKey["kend"].Data.Status, "Unreached end stays pending")
	})

	t.Run("WithdrawnRendersNoCurrentNode", func(t *testing.T) {
		// A withdrawn instance is closed but not "final": its current pointer is
		// retained and its tasks are canceled. No node may render as current, and
		// the paused node falls back to pending rather than completed.
		b := linearBundle()
		b.Instance.Status = approval.InstanceWithdrawn
		b.Instance.CurrentNodeID = new("na")
		b.Tasks = []approval.Task{task("t1", "na", "u1", approval.TaskCanceled)}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		for key, n := range byKey {
			assert.NotEqual(t, approval.NodeProgressCurrent, n.Data.Status, "Closed (withdrawn) instance must not mark any node current, got current on %q", key)
		}

		assert.Equal(t, approval.NodeProgressCompleted, byKey["kstart"].Data.Status, "Start stays completed")
		assert.Equal(t, approval.NodeProgressPending, byKey["kappr"].Data.Status, "Paused node with only a canceled task is pending, not current or completed")
	})

	t.Run("UntakenBranchApprovalNodeStaysPending", func(t *testing.T) {
		// start -> C(condition) -> {A, B} -> M -> end. The instance took the A
		// branch (A approved) and now sits on the merge M. The condition on the
		// taken path is structural and completes; the *untaken* branch's approval
		// node B is a business node with no evidence and must stay pending — the
		// business/structural split is what keeps topology from over-reporting it.
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				flowNode("ns", "kstart", approval.NodeStart),
				flowNode("nc", "kcond", approval.NodeCondition),
				flowNode("na", "kA", approval.NodeApproval),
				flowNode("nb", "kB", approval.NodeApproval),
				flowNode("nm", "kM", approval.NodeApproval),
				flowNode("ne", "kend", approval.NodeEnd),
			},
			FlowSchema: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{
					{ID: "kstart", Kind: approval.NodeStart},
					{ID: "kcond", Kind: approval.NodeCondition},
					{ID: "kA", Kind: approval.NodeApproval},
					{ID: "kB", Kind: approval.NodeApproval},
					{ID: "kM", Kind: approval.NodeApproval},
					{ID: "kend", Kind: approval.NodeEnd},
				},
				Edges: []approval.EdgeDefinition{
					{ID: "e1", Source: "kstart", Target: "kcond"},
					{ID: "e2", Source: "kcond", Target: "kA"},
					{ID: "e3", Source: "kcond", Target: "kB"},
					{ID: "e4", Source: "kA", Target: "kM"},
					{ID: "e5", Source: "kB", Target: "kM"},
					{ID: "e6", Source: "kM", Target: "kend"},
				},
			},
		}
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("nm")
		b.Tasks = []approval.Task{
			task("t1", "na", "u1", approval.TaskApproved),
			task("t2", "nm", "u2", approval.TaskPending),
		}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressCompleted, byKey["kcond"].Data.Status, "Condition on the taken path is a traversed structural node → completed")
		assert.Equal(t, approval.NodeProgressCompleted, byKey["kA"].Data.Status, "Taken-branch approval node with an approved task → completed")
		assert.Equal(t, approval.NodeProgressCurrent, byKey["kM"].Data.Status, "Merge node with a pending task → current")
		assert.Equal(t, approval.NodeProgressPending, byKey["kB"].Data.Status, "Untaken-branch approval node has no evidence → pending, not completed")
	})
}
