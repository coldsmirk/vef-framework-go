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

// visit builds an in-memory NodeVisit with the given id/node/sequence/status.
func visit(id, nodeID string, sequence int, status approval.NodeVisitStatus) approval.NodeVisit {
	v := approval.NodeVisit{NodeID: nodeID, Sequence: sequence, Status: status}
	v.ID = id

	return v
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
// schema (positions + edges). Callers set Instance status/current-node, Visits,
// Tasks, and ActionLogs per scenario. DB node ids are ns/na/ne; keys are
// kstart/kappr/kend.
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
	t.Run("RunningReadsVisitTrail", func(t *testing.T) {
		b := linearBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitActive),
		}
		tk := task("t1", "na", "u1", approval.TaskPending)
		tk.VisitID = "v2"
		b.Tasks = []approval.Task{tk}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		require.Contains(t, byKey, "kstart", "Graph should contain the start node")
		assert.Equal(t, approval.NodeProgressPassed, byKey["kstart"].Data.Status, "Start node reports its passed visit")
		assert.Equal(t, approval.NodeProgressActive, byKey["kappr"].Data.Status, "Approval node with an open visit should be active")
		assert.Equal(t, approval.NodeProgressPending, byKey["kend"].Data.Status, "Unvisited end node should be pending")

		require.Len(t, byKey["kappr"].Data.Participants, 1, "Approval node should list its assignee")
		assert.Equal(t, "u1", byKey["kappr"].Data.Participants[0].User.ID, "Participant should be the assignee")
		assert.Equal(t, string(approval.TaskPending), byKey["kappr"].Data.Participants[0].Status, "Pending participant status")
		assert.InDelta(t, 100.0, byKey["kappr"].Position.Y, 0, "Node position should come from the schema")
		assert.Equal(t, "na", byKey["kappr"].NodeID, "Node must expose its persistent DB id for rollback targeting and action-log correlation")
	})

	t.Run("FinalMarksTrailPassedIncludingEnd", func(t *testing.T) {
		b := linearBundle()
		b.Instance.Status = approval.InstanceApproved
		b.Instance.CurrentNodeID = new("ne")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitPassed),
			visit("v3", "ne", 3, approval.NodeVisitPassed),
		}
		approved := task("t1", "na", "u1", approval.TaskApproved)
		approved.VisitID = "v2"
		b.Tasks = []approval.Task{approved}
		log := approval.ActionLog{Action: approval.ActionApprove, OperatorID: "u1", NodeID: new("na"), TaskID: new("t1"), Opinion: new("approved-opinion")}
		log.ID = "l1"
		b.ActionLogs = []approval.ActionLog{log}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressPassed, byKey["kstart"].Data.Status, "Start node passed")
		assert.Equal(t, approval.NodeProgressPassed, byKey["kappr"].Data.Status, "Approval node passed")
		assert.Equal(t, approval.NodeProgressPassed, byKey["kend"].Data.Status, "End node of a finished instance passed")

		require.Len(t, byKey["kappr"].Data.Participants, 1, "Approval node should list its assignee")
		require.NotNil(t, byKey["kappr"].Data.Participants[0].Opinion, "Participant should carry the finishing log's opinion")
		assert.Equal(t, "approved-opinion", *byKey["kappr"].Data.Participants[0].Opinion, "Opinion should come from the log that finished this task")
	})

	t.Run("RejectedNodeReportsRejected", func(t *testing.T) {
		b := linearBundle()
		b.Instance.Status = approval.InstanceRejected
		b.Instance.CurrentNodeID = new("na")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitRejected),
		}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressRejected, byKey["kappr"].Data.Status, "Rejecting node should render as rejected")
		assert.Equal(t, approval.NodeProgressPending, byKey["kend"].Data.Status, "End node never reached stays pending")
	})

	t.Run("ConditionVisitMarksItPassed", func(t *testing.T) {
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				flowNode("ns", "kstart", approval.NodeStart),
				flowNode("nc", "kcond", approval.NodeCondition),
				flowNode("na", "kappr", approval.NodeApproval),
			},
		}
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "nc", 2, approval.NodeVisitPassed),
			visit("v3", "na", 3, approval.NodeVisitActive),
		}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressPassed, byKey["kcond"].Data.Status, "Traversed condition node reports its recorded visit — no inference")
	})

	t.Run("NodePayloadMirrorsTimeline", func(t *testing.T) {
		// The diagram's node cards carry the same execution payload the
		// timeline entries do: the start node holds the submit activity, and
		// visited nodes report their CC recipients and time span.
		b := linearBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		startVisit := visit("v1", "ns", 1, approval.NodeVisitPassed)
		startVisit.FinishedAt = new(at(0))
		b.Visits = []approval.NodeVisit{
			startVisit,
			visit("v2", "na", 2, approval.NodeVisitActive),
		}

		submit := approval.ActionLog{Action: approval.ActionSubmit, OperatorID: "applicant", OperatorName: "Applicant"}
		submit.ID = "l0"
		b.ActionLogs = []approval.ActionLog{submit}

		cc := approval.CCRecord{NodeID: new("na"), CCUserID: "cc-1", CCUserName: "CC One"}
		cc.ID = "ccr-1"
		b.CCRecords = []approval.CCRecord{cc}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		start := byKey["kstart"].Data
		require.Len(t, start.Activities, 1, "Start node should carry the submit activity")
		assert.Equal(t, string(approval.ActionSubmit), start.Activities[0].Action, "Start activity should be the submission")
		require.NotNil(t, start.StartedAt, "Visited start node should carry its entry time")
		require.NotNil(t, start.FinishedAt, "Concluded start node should carry its finish time")

		appr := byKey["kappr"].Data
		require.Len(t, appr.CCRecipients, 1, "Approval node should list its CC recipient")
		assert.Equal(t, "cc-1", appr.CCRecipients[0].User.ID, "CC recipient identity passes through")
		require.NotNil(t, appr.StartedAt, "Executing node carries its entry time")
		assert.Nil(t, appr.FinishedAt, "Executing node has no finish time yet")

		assert.Nil(t, byKey["kend"].Data.StartedAt, "Unvisited node carries no time span")
	})

	t.Run("ReturnedInstanceShowsRollbackTargetActive", func(t *testing.T) {
		b := linearBundle()
		b.Instance.Status = approval.InstanceReturned
		// Rolled back to start: no visit is open there until resubmit, but the
		// instance is waiting on the applicant, so the target renders active.
		b.Instance.CurrentNodeID = new("ns")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitReturned),
		}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressActive, byKey["kstart"].Data.Status, "Rollback target of a returned instance should be active")
		assert.Equal(t, approval.NodeProgressReturned, byKey["kappr"].Data.Status, "The node the flow was sent back from should be returned")
	})

	t.Run("NilSchemaStillBuildsNodesWithoutEdges", func(t *testing.T) {
		b := linearBundle()
		b.FlowSchema = nil
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitActive),
		}

		g := buildInstanceFlowGraph(b)
		byKey := nodesByKey(g)

		require.Len(t, g.Nodes, 3, "Nodes come from the flow-node rows even without a schema")
		assert.Empty(t, g.Edges, "No edges without a schema")
		assert.Equal(t, approval.NodeProgressPassed, byKey["kstart"].Data.Status, "Start node still reports its visit")
		assert.Equal(t, approval.NodeProgressActive, byKey["kappr"].Data.Status, "Open visit still renders active")
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
		b.Visits = []approval.NodeVisit{visit("v1", "nh", 1, approval.NodeVisitActive)}
		tk := task("t1", "nh", "u1", approval.TaskPending)
		tk.VisitID = "v1"
		b.Tasks = []approval.Task{tk}

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
		// Same assignee acted twice on the same node across a rollback: two
		// visits, two task rows, two logs. The node reports its latest visit,
		// and each participant keeps its own pass's opinion.
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitReturned),
			visit("v3", "na", 3, approval.NodeVisitActive),
		}
		first := task("t1", "na", "u1", approval.TaskRolledBack)
		first.VisitID = "v2"
		second := task("t2", "na", "u1", approval.TaskApproved)
		second.VisitID = "v3"
		b.Tasks = []approval.Task{first, second}
		log1 := approval.ActionLog{Action: approval.ActionRollback, OperatorID: "u1", NodeID: new("na"), TaskID: new("t1"), Opinion: new("first-pass")}
		log1.ID = "l1"
		log2 := approval.ActionLog{Action: approval.ActionApprove, OperatorID: "u1", NodeID: new("na"), TaskID: new("t2"), Opinion: new("second-pass")}
		log2.ID = "l2"
		b.ActionLogs = []approval.ActionLog{log1, log2}

		node := nodesByKey(buildInstanceFlowGraph(b))["kappr"]
		parts := node.Data.Participants

		assert.Equal(t, approval.NodeProgressActive, node.Data.Status, "Latest visit decides the node status")
		require.Len(t, parts, 2, "Both passes should surface as distinct participants")
		require.NotNil(t, parts[0].Opinion, "First pass opinion present")
		require.NotNil(t, parts[1].Opinion, "Second pass opinion present")
		assert.Equal(t, "first-pass", *parts[0].Opinion, "First task's participant keeps the first log's opinion")
		assert.Equal(t, "second-pass", *parts[1].Opinion, "Second task's participant keeps the second log's opinion")
	})

	t.Run("RolledBackFromNodeReportsReturned", func(t *testing.T) {
		// start -> A -> B -> end. The instance reached B, then B was rolled back
		// to A: A re-opens with a fresh visit while B's visit concluded as
		// returned. B must never render as passed on stale evidence — and the
		// trail can say something better than pending: it reports the actual
		// outcome, "returned".
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				flowNode("ns", "kstart", approval.NodeStart),
				flowNode("na", "kA", approval.NodeApproval),
				flowNode("nb", "kB", approval.NodeApproval),
				flowNode("ne", "kend", approval.NodeEnd),
			},
		}
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("na")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitPassed),
			visit("v3", "nb", 3, approval.NodeVisitReturned),
			visit("v4", "na", 4, approval.NodeVisitActive),
		}
		redo := task("t1", "na", "u1", approval.TaskPending)
		redo.VisitID = "v4"
		rolledBack := task("t2", "nb", "u2", approval.TaskRolledBack)
		rolledBack.VisitID = "v3"
		b.Tasks = []approval.Task{redo, rolledBack}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressPassed, byKey["kstart"].Data.Status, "Start node stays passed")
		assert.Equal(t, approval.NodeProgressActive, byKey["kA"].Data.Status, "Rolled-back-to node re-opens as active")
		assert.Equal(t, approval.NodeProgressReturned, byKey["kB"].Data.Status, "Rolled-back-from node reports returned — never passed on stale evidence")
		assert.Equal(t, approval.NodeProgressPending, byKey["kend"].Data.Status, "Unreached end stays pending")
	})

	t.Run("WithdrawnInstanceRendersCanceledNotActive", func(t *testing.T) {
		// Withdraw cancels the open visits: no node may render as active, and
		// the node the instance was paused on reports canceled — the trail's
		// recorded outcome — rather than degrading to pending.
		b := linearBundle()
		b.Instance.Status = approval.InstanceWithdrawn
		b.Instance.CurrentNodeID = new("na")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "na", 2, approval.NodeVisitCanceled),
		}
		canceled := task("t1", "na", "u1", approval.TaskCanceled)
		canceled.VisitID = "v2"
		b.Tasks = []approval.Task{canceled}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		for key, n := range byKey {
			assert.NotEqual(t, approval.NodeProgressActive, n.Data.Status, "Withdrawn instance must not mark any node active, got active on %q", key)
		}

		assert.Equal(t, approval.NodeProgressPassed, byKey["kstart"].Data.Status, "Start stays passed")
		assert.Equal(t, approval.NodeProgressCanceled, byKey["kappr"].Data.Status, "The node the withdrawal cut short reports canceled")
	})

	t.Run("UntakenBranchStaysPending", func(t *testing.T) {
		// start -> C(condition) -> {A, B} -> M -> end. The instance took the A
		// branch and now sits on the merge M. Under the visit trail this needs
		// no branch heuristics: only traversed nodes have visits, so the
		// untaken branch's approval node B is pending by construction — even
		// though it reconverges into the reached merge.
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				flowNode("ns", "kstart", approval.NodeStart),
				flowNode("nc", "kcond", approval.NodeCondition),
				flowNode("na", "kA", approval.NodeApproval),
				flowNode("nb", "kB", approval.NodeApproval),
				flowNode("nm", "kM", approval.NodeApproval),
				flowNode("ne", "kend", approval.NodeEnd),
			},
		}
		b.Instance.Status = approval.InstanceRunning
		b.Instance.CurrentNodeID = new("nm")
		b.Visits = []approval.NodeVisit{
			visit("v1", "ns", 1, approval.NodeVisitPassed),
			visit("v2", "nc", 2, approval.NodeVisitPassed),
			visit("v3", "na", 3, approval.NodeVisitPassed),
			visit("v4", "nm", 4, approval.NodeVisitActive),
		}
		taken := task("t1", "na", "u1", approval.TaskApproved)
		taken.VisitID = "v3"
		merge := task("t2", "nm", "u2", approval.TaskPending)
		merge.VisitID = "v4"
		b.Tasks = []approval.Task{taken, merge}

		byKey := nodesByKey(buildInstanceFlowGraph(b))

		assert.Equal(t, approval.NodeProgressPassed, byKey["kcond"].Data.Status, "Condition on the taken path passed")
		assert.Equal(t, approval.NodeProgressPassed, byKey["kA"].Data.Status, "Taken-branch approval node passed")
		assert.Equal(t, approval.NodeProgressActive, byKey["kM"].Data.Status, "Merge node with the open visit is active")
		assert.Equal(t, approval.NodeProgressPending, byKey["kB"].Data.Status, "Untaken-branch approval node has no visit → pending")
	})
}
