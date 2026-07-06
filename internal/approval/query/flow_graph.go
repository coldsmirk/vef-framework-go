package query

import "github.com/coldsmirk/vef-framework-go/approval"

// buildInstanceFlowGraph projects the instance's pinned flow definition into a
// React Flow–ready graph annotated with per-node progress. Node identity is the
// flow-node Key (which equals the design-time React Flow node id). Runtime
// state (visits, tasks, action logs) is recorded against the flow-node DB id,
// so it is mapped into Key space before assembly.
//
// Progress is read from the visit trail the engine records as it traverses
// nodes — no inference: a node with an open visit is active, a concluded node
// reports its latest visit's outcome, and a node with no visit is pending.
// Each node's payload mirrors the timeline entries of the same node —
// participants, CC recipients, and activities aggregated across its visits in
// traversal order, from the same shared index — so the diagram and the
// transit-record view can never disagree about what happened at a node.
func buildInstanceFlowGraph(bundle *instanceDetailBundle) approval.InstanceFlowGraph {
	positions, edges := graphLayout(bundle.FlowSchema)

	idToKey := make(map[string]string, len(bundle.FlowNodes))
	for i := range bundle.FlowNodes {
		idToKey[bundle.FlowNodes[i].ID] = bundle.FlowNodes[i].Key
	}

	idx := newVisitIndex(bundle)
	statusByKey := deriveNodeProgress(bundle, idToKey)

	nodes := make([]approval.FlowGraphNode, len(bundle.FlowNodes))
	for i := range bundle.FlowNodes {
		fn := bundle.FlowNodes[i]

		status, visited := statusByKey[fn.Key]
		if !visited {
			status = approval.NodeProgressPending
		}

		data := approval.FlowGraphNodeData{
			Name:   fn.Name,
			Status: status,
		}

		fillNodeExecution(&data, idx, fn.ID)

		switch fn.Kind {
		case approval.NodeApproval:
			data.ExecutionType = string(fn.ExecutionType)
			data.ApprovalMethod = string(fn.ApprovalMethod)
			data.PassRule = string(fn.PassRule)

			if fn.PassRule == approval.PassRatio {
				data.PassRatio = new(fn.PassRatio)
			}

		case approval.NodeHandle:
			// Handle nodes claim-and-do; approvalMethod / passRule do not apply.
			data.ExecutionType = string(fn.ExecutionType)
		}

		nodes[i] = approval.FlowGraphNode{
			ID:       fn.Key,
			NodeID:   fn.ID,
			Kind:     fn.Kind,
			Position: positions[fn.Key],
			Data:     data,
		}
	}

	return approval.InstanceFlowGraph{Nodes: nodes, Edges: edges}
}

// fillNodeExecution aggregates the node's execution payload across its visits
// in traversal order — participants, CC recipients, activities, and the
// first-entered / last-concluded time span (FinishedAt stays nil while the
// node is executing).
func fillNodeExecution(data *approval.FlowGraphNodeData, idx *visitIndex, nodeID string) {
	visits := idx.visitsByNode[nodeID]
	if len(visits) == 0 {
		return
	}

	for _, visit := range visits {
		data.Participants = append(data.Participants, buildParticipants(idx.tasksByVisit[visit.ID], idx.finisherLogs)...)
		data.CCRecipients = append(data.CCRecipients, idx.ccByVisit[visit.ID]...)
		data.Activities = append(data.Activities, idx.activitiesByVisit[visit.ID]...)
	}

	data.StartedAt = new(visits[0].CreatedAt)
	data.FinishedAt = visits[len(visits)-1].FinishedAt
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

// deriveNodeProgress reads each node's progress from the visit trail. Visits
// arrive in sequence order, so the latest visit of a node decides its status —
// and NodeVisitStatus values map onto NodeProgressStatus verbatim (an open
// visit is "active"). One overlay: a paused instance (returned, or withdrawn
// while resting there) shows its current node — the resting point waiting on
// the applicant — as active even though no visit is open there yet; nodes
// that do carry visits keep their visit-derived status.
func deriveNodeProgress(bundle *instanceDetailBundle, idToKey map[string]string) map[string]approval.NodeProgressStatus {
	status := make(map[string]approval.NodeProgressStatus, len(bundle.Visits))

	for i := range bundle.Visits {
		if key := idToKey[bundle.Visits[i].NodeID]; key != "" {
			status[key] = approval.NodeProgressStatus(bundle.Visits[i].Status)
		}
	}

	paused := bundle.Instance.Status == approval.InstanceReturned ||
		bundle.Instance.Status == approval.InstanceWithdrawn
	if paused && bundle.Instance.CurrentNodeID != nil {
		if key := idToKey[*bundle.Instance.CurrentNodeID]; key != "" {
			// A canceled visit means a mid-flow withdraw cut the node short —
			// keep that verdict. Every other resting point (a passed historical
			// visit, or none at all after return-to-initiator) renders active.
			if status[key] != approval.NodeProgressCanceled {
				status[key] = approval.NodeProgressActive
			}
		}
	}

	return status
}
