package query

import "github.com/coldsmirk/vef-framework-go/approval"

// buildInstanceFlowGraph projects the instance's pinned flow definition into a
// React Flow–ready graph annotated with per-node progress. Node identity is the
// flow-node Key (which equals the design-time React Flow node id). Runtime state
// (tasks, action logs, current-node pointer) is recorded against the flow-node
// DB id, so it is mapped into Key space before any graph reasoning.
//
// Progress is read from real runtime evidence rather than pure topology (see
// deriveProgressSets): a node is current when work is open on it, completed when
// it produced an advancing decision (business nodes) or was provably traversed
// (structural nodes), otherwise pending. This keeps a rolled-back-from node ahead
// of the current pointer, and every node of a paused/closed instance, from
// mis-rendering. The one residual imprecision is a purely structural node on an
// *untaken* branch that reconverges into a reached node, which can still
// over-report as completed — the graph carries no node-visit fact to tell which
// incoming edge a merge was reached through.
func buildInstanceFlowGraph(bundle *instanceDetailBundle) approval.InstanceFlowGraph {
	positions, edges := graphLayout(bundle.FlowSchema)

	idToKey := make(map[string]string, len(bundle.FlowNodes))
	for i := range bundle.FlowNodes {
		idToKey[bundle.FlowNodes[i].ID] = bundle.FlowNodes[i].Key
	}

	tasksByKey := make(map[string][]approval.Task, len(bundle.FlowNodes))
	for _, t := range bundle.Tasks {
		key := idToKey[t.NodeID]
		tasksByKey[key] = append(tasksByKey[key], t)
	}

	// logsByTaskID matches each task to the log that finished it, so repeated
	// visits to a node (rollback → redo) keep their own distinct opinions
	// instead of collapsing onto the latest log for that node + operator.
	logsByTaskID := make(map[string]approval.ActionLog, len(bundle.ActionLogs))
	for _, l := range bundle.ActionLogs {
		if l.TaskID != nil {
			logsByTaskID[*l.TaskID] = l
		}
	}

	active, completed := deriveProgressSets(bundle, idToKey, tasksByKey)

	nodes := make([]approval.FlowGraphNode, len(bundle.FlowNodes))
	for i := range bundle.FlowNodes {
		fn := bundle.FlowNodes[i]

		data := approval.FlowGraphNodeData{
			Name:   fn.Name,
			Status: nodeProgressStatus(fn.Key, active, completed),
		}

		switch fn.Kind {
		case approval.NodeApproval:
			data.ExecutionType = string(fn.ExecutionType)
			data.ApprovalMethod = string(fn.ApprovalMethod)
			data.PassRule = string(fn.PassRule)

			if fn.PassRule == approval.PassRatio {
				ratio := fn.PassRatio
				data.PassRatio = &ratio
			}

			data.Participants = buildNodeParticipants(tasksByKey[fn.Key], logsByTaskID)

		case approval.NodeHandle:
			// Handle nodes claim-and-do; approvalMethod / passRule do not apply.
			data.ExecutionType = string(fn.ExecutionType)
			data.Participants = buildNodeParticipants(tasksByKey[fn.Key], logsByTaskID)
		}

		nodes[i] = approval.FlowGraphNode{
			ID:       fn.Key,
			NodeID:   fn.ID,
			Kind:     string(fn.Kind),
			Position: positions[fn.Key],
			Data:     data,
		}
	}

	return approval.InstanceFlowGraph{Nodes: nodes, Edges: edges}
}

// graphLayout extracts node positions (keyed by node key) and edges from the
// pinned React Flow definition. Returns an empty map and nil edges when the
// version stored no graph.
func graphLayout(def *approval.FlowDefinition) (map[string]approval.Position, []approval.FlowGraphEdge) {
	if def == nil {
		return map[string]approval.Position{}, nil
	}

	positions := make(map[string]approval.Position, len(def.Nodes))
	for _, n := range def.Nodes {
		positions[n.ID] = n.Position
	}

	edges := make([]approval.FlowGraphEdge, len(def.Edges))
	for i, e := range def.Edges {
		edges[i] = approval.FlowGraphEdge{
			ID:           e.ID,
			Source:       e.Source,
			Target:       e.Target,
			SourceHandle: e.SourceHandle,
		}
	}

	return positions, edges
}

// deriveProgressSets classifies every node key into the active (in-progress) and
// completed sets that nodeProgressStatus reads; a key in neither is pending.
// Progress comes from real runtime evidence, never from raw topology:
//
//   - active: a node with an unfinished task (someone is still acting on it), plus
//     — only while the instance is actually running — the node its current pointer
//     sits on. A paused or closed instance (withdrawn/returned/final) contributes
//     no current pointer, so its last node never renders a stale "current".
//   - completed, business node (approval/handle): only on its own advancing
//     evidence — a task in a decision-final state (approved/rejected/handled/
//     transferred/skipped). A node whose tasks were merely rolled back or canceled
//     is not completed, so a rolled-back-from node ahead of the current pointer
//     falls back to pending instead of showing as passed.
//   - completed, structural node (start/condition/gateway/end): when it is a graph
//     ancestor of the frontier (the start node, active nodes, advanced business
//     nodes, and a finished instance's terminal node). Business nodes are excluded
//     from this closure so an approval node on an untaken branch that reconverges
//     into a reached node is never marked completed on topology alone.
func deriveProgressSets(bundle *instanceDetailBundle, idToKey map[string]string, tasksByKey map[string][]approval.Task) (active, completed map[string]struct{}) {
	running := bundle.Instance.Status == approval.InstanceRunning
	final := bundle.Instance.Status.IsFinal()

	currentKey := ""
	if bundle.Instance.CurrentNodeID != nil {
		currentKey = idToKey[*bundle.Instance.CurrentNodeID]
	}

	// A node with an unfinished task is in progress regardless of instance status;
	// the current pointer only counts while the instance runs, so a closed instance
	// shows no current node.
	active = make(map[string]struct{})
	for key, tasks := range tasksByKey {
		for _, t := range tasks {
			if !t.Status.IsFinal() {
				active[key] = struct{}{}

				break
			}
		}
	}

	if running && currentKey != "" {
		active[currentKey] = struct{}{}
	}

	// A business node counts as passed only when it produced an advancing decision;
	// this is also the frontier seed that drives structural completion.
	progressedBusiness := make(map[string]struct{})
	for key, tasks := range tasksByKey {
		for _, t := range tasks {
			if isAdvancingTaskStatus(t.Status) {
				progressedBusiness[key] = struct{}{}

				break
			}
		}
	}

	seeds := make(map[string]struct{}, len(active)+len(progressedBusiness)+1)
	for k := range active {
		seeds[k] = struct{}{}
	}

	for k := range progressedBusiness {
		seeds[k] = struct{}{}
	}

	for i := range bundle.FlowNodes {
		if bundle.FlowNodes[i].Kind == approval.NodeStart {
			seeds[bundle.FlowNodes[i].Key] = struct{}{}
		}
	}

	if final && currentKey != "" {
		seeds[currentKey] = struct{}{}
	}

	ancestors := reverseReachable(seeds, bundle.FlowSchema)

	// Business nodes earn completion from their own evidence; structural nodes from
	// the ancestor closure. Splitting the two is what keeps an untaken branch's
	// approval node out of the completed set.
	completed = make(map[string]struct{}, len(ancestors))
	for i := range bundle.FlowNodes {
		fn := bundle.FlowNodes[i]

		if isBusinessNode(fn.Kind) {
			if _, ok := progressedBusiness[fn.Key]; ok {
				completed[fn.Key] = struct{}{}
			}

			continue
		}

		if _, ok := ancestors[fn.Key]; ok {
			completed[fn.Key] = struct{}{}
		}
	}

	return active, completed
}

// isBusinessNode reports whether a node kind performs assignee work (approval or
// handle). Only these carry participants and derive completion from their own task
// evidence; every other kind is structural and completes via the ancestor closure.
func isBusinessNode(kind approval.NodeKind) bool {
	return kind == approval.NodeApproval || kind == approval.NodeHandle
}

// isAdvancingTaskStatus reports whether a task status represents a decision that
// carried the flow past its node, as opposed to being rolled back, canceled, or
// removed. It is the evidence that marks a business node completed.
func isAdvancingTaskStatus(s approval.TaskStatus) bool {
	switch s {
	case approval.TaskApproved, approval.TaskRejected, approval.TaskHandled, approval.TaskTransferred, approval.TaskSkipped:
		return true
	default:
		return false
	}
}

// reverseReachable returns the seed set plus every node that can reach a seed by
// following graph edges (all graph ancestors of the seeds), marking structural
// nodes the flow passed through on its way to the frontier.
func reverseReachable(seeds map[string]struct{}, def *approval.FlowDefinition) map[string]struct{} {
	result := make(map[string]struct{}, len(seeds))
	for k := range seeds {
		result[k] = struct{}{}
	}

	if def == nil {
		return result
	}

	predecessors := make(map[string][]string, len(def.Edges))
	for _, e := range def.Edges {
		predecessors[e.Target] = append(predecessors[e.Target], e.Source)
	}

	queue := make([]string, 0, len(seeds))
	for k := range seeds {
		queue = append(queue, k)
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for _, pred := range predecessors[node] {
			if _, seen := result[pred]; !seen {
				result[pred] = struct{}{}
				queue = append(queue, pred)
			}
		}
	}

	return result
}

// nodeProgressStatus classifies a node key against the active and completed sets,
// with active taking precedence so a node still being worked reads as current.
func nodeProgressStatus(key string, active, completed map[string]struct{}) approval.NodeProgressStatus {
	if _, ok := active[key]; ok {
		return approval.NodeProgressCurrent
	}

	if _, ok := completed[key]; ok {
		return approval.NodeProgressCompleted
	}

	return approval.NodeProgressPending
}

// buildNodeParticipants assembles the per-assignee outcome list for an approval
// or handle node from its tasks (order + status) and the log that finished each
// task (opinion + time). Tasks arrive in sort order, which the list preserves.
func buildNodeParticipants(nodeTasks []approval.Task, logsByTaskID map[string]approval.ActionLog) []approval.FlowGraphParticipant {
	if len(nodeTasks) == 0 {
		return nil
	}

	parts := make([]approval.FlowGraphParticipant, len(nodeTasks))
	for i, t := range nodeTasks {
		p := approval.FlowGraphParticipant{
			UserID: t.AssigneeID,
			Name:   t.AssigneeName,
			Status: string(t.Status),
		}

		if l, ok := logsByTaskID[t.ID]; ok {
			p.Opinion = l.Opinion
			p.ActionTime = &l.CreatedAt
		} else if t.FinishedAt != nil {
			p.ActionTime = t.FinishedAt
		}

		parts[i] = p
	}

	return parts
}
