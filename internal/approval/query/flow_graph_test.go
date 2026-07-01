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
}
