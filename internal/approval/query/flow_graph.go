package query

import "github.com/coldsmirk/vef-framework-go/approval"

// buildInstanceFlowGraph projects the instance's pinned flow definition into a
// React Flow–ready graph annotated with per-node progress. Node identity is the
// flow-node Key (which equals the design-time React Flow node id). Runtime state
// (tasks, action logs, current-node pointer) is recorded against the flow-node
// DB id, so it is mapped into Key space before any graph reasoning.
//
// Progress is derived topologically, not from log side-effects: a node is
// current when the instance sits on it, completed when the flow has demonstrably
// passed it (the start node, a node with task/log evidence, the terminal node of
// a finished instance, or a graph ancestor of any of those), otherwise pending.
// This is correct for linear flows and single-path branches; a structural node
// on an *untaken* branch that reconverges into a reached node may over-report as
// completed — a known limitation of inferring visitation from topology rather
// than from a recorded node-visit fact.
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

	active, passed := deriveProgressSets(bundle, idToKey, tasksByKey)

	nodes := make([]approval.FlowGraphNode, len(bundle.FlowNodes))
	for i := range bundle.FlowNodes {
		fn := bundle.FlowNodes[i]

		data := approval.FlowGraphNodeData{
			Name:   fn.Name,
			Status: nodeProgressStatus(fn.Key, active, passed),
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

// deriveProgressSets computes the active (in-progress) and passed (completed or
// active) node-key sets. active is empty for a finished instance. passed is the
// reverse-reachability closure of the frontier — current node + pending-task
// nodes + directly-evidenced nodes (any task or action log), the start node, and
// the terminal node of a finished instance — over the graph edges, so structural
// nodes the flow traversed are marked completed even though they leave no task
// or log.
func deriveProgressSets(bundle *instanceDetailBundle, idToKey map[string]string, tasksByKey map[string][]approval.Task) (active, passed map[string]struct{}) {
	final := bundle.Instance.Status.IsFinal()

	currentKey := ""
	if bundle.Instance.CurrentNodeID != nil {
		currentKey = idToKey[*bundle.Instance.CurrentNodeID]
	}

	active = make(map[string]struct{})
	if !final {
		if currentKey != "" {
			active[currentKey] = struct{}{}
		}

		for key, tasks := range tasksByKey {
			for _, t := range tasks {
				if t.Status == approval.TaskPending {
					active[key] = struct{}{}

					break
				}
			}
		}
	}

	seeds := make(map[string]struct{}, len(active)+len(tasksByKey))
	for k := range active {
		seeds[k] = struct{}{}
	}

	for key := range tasksByKey {
		seeds[key] = struct{}{}
	}

	for _, l := range bundle.ActionLogs {
		if l.NodeID != nil {
			if key := idToKey[*l.NodeID]; key != "" {
				seeds[key] = struct{}{}
			}
		}
	}

	for i := range bundle.FlowNodes {
		if bundle.FlowNodes[i].Kind == approval.NodeStart {
			seeds[bundle.FlowNodes[i].Key] = struct{}{}
		}
	}

	if final && currentKey != "" {
		seeds[currentKey] = struct{}{}
	}

	return active, reverseReachable(seeds, bundle.FlowSchema)
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

// nodeProgressStatus classifies a node key against the active and passed sets.
func nodeProgressStatus(key string, active, passed map[string]struct{}) approval.NodeProgressStatus {
	if _, ok := active[key]; ok {
		return approval.NodeProgressCurrent
	}

	if _, ok := passed[key]; ok {
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
