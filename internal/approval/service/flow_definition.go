package service

import (
	"errors"
	"fmt"

	"github.com/coldsmirk/go-collections"

	streams "github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/approval"
)

var (
	errEmptyNodeID        = errors.New("node ID must not be empty")
	errDuplicateNodeID    = errors.New("duplicate node ID")
	errInvalidNodeKind    = errors.New("invalid node kind")
	errStartNodeCount     = errors.New("flow must have exactly 1 start node")
	errEndNodeCount       = errors.New("flow must have at least 1 end node")
	errEmptyEdgeID        = errors.New("edge ID must not be empty")
	errDuplicateEdgeID    = errors.New("duplicate edge ID")
	errUnknownSourceNode  = errors.New("edge references unknown source node")
	errUnknownTargetNode  = errors.New("edge references unknown target node")
	errStartIncoming      = errors.New("start node must not have incoming edges")
	errStartOutgoing      = errors.New("start node must have exactly 1 outgoing edge")
	errEndOutgoing        = errors.New("end node must not have outgoing edges")
	errEndIncoming        = errors.New("end node must have at least 1 incoming edge")
	errNodeOutgoingCount  = errors.New("node must have exactly 1 outgoing edge")
	errNodeSourceHandle   = errors.New("non-condition node must not have sourceHandle on outgoing edge")
	errGraphCycle         = errors.New("flow graph contains a cycle")
	errNodeUnreachable    = errors.New("node is not reachable from start node")
	errNodeCannotReachEnd = errors.New("node cannot reach end node")
	errNoNodes            = errors.New("flow must have at least one node")
	errCondMinBranches    = errors.New("condition node must have at least 2 branches")
	errCondEmptyBranchID  = errors.New("condition node has a branch with empty ID")
	errCondDupBranchID    = errors.New("condition node has duplicate branch ID")
	errCondDefaultCount   = errors.New("condition node must have exactly 1 default branch")
	errCondMissingHandle  = errors.New("edge must have a sourceHandle")
	errCondUnknownHandle  = errors.New("edge has unknown sourceHandle")
	errCondDupHandle      = errors.New("duplicate outgoing edge for handle")
	errCondBranchNoEdge   = errors.New("branch has no outgoing edge")
)

// validNodeKinds defines the set of valid node kinds for flow validation.
var validNodeKinds = collections.NewHashSetFrom(
	approval.NodeStart,
	approval.NodeEnd,
	approval.NodeApproval,
	approval.NodeHandle,
	approval.NodeCondition,
	approval.NodeCC,
)

// FlowDefinitionService provides flow-level domain operations.
type FlowDefinitionService struct{}

// NewFlowDefinitionService creates a new FlowDefinitionService.
func NewFlowDefinitionService() *FlowDefinitionService {
	return &FlowDefinitionService{}
}

// ValidateFlowDefinition validates the structural integrity of a flow definition.
//
//nolint:gocyclo // validation function inherently requires many checks
func (*FlowDefinitionService) ValidateFlowDefinition(def *approval.FlowDefinition) error {
	if len(def.Nodes) == 0 {
		return errNoNodes
	}

	// --- Phase 1: Node validation ---
	var (
		nodeIDs      = collections.NewHashSet[string]()
		condBranches = make(map[string][]approval.ConditionBranch)

		startCount, endCount int
		startID              string
		endIDs               []string
	)

	for i := range def.Nodes {
		node := &def.Nodes[i]

		if node.ID == "" {
			return errEmptyNodeID
		}

		if !nodeIDs.Add(node.ID) {
			return fmt.Errorf("%w: %q", errDuplicateNodeID, node.ID)
		}

		if !validNodeKinds.Contains(node.Kind) {
			return fmt.Errorf("%w: %q for node %q", errInvalidNodeKind, node.Kind, node.ID)
		}

		switch node.Kind {
		case approval.NodeStart:
			startCount++
			startID = node.ID
		case approval.NodeEnd:
			endCount++

			endIDs = append(endIDs, node.ID)
		case approval.NodeCondition:
			data, err := node.ParseData()
			if err != nil {
				return fmt.Errorf("parse node %q data: %w", node.ID, err)
			}

			condBranches[node.ID] = data.(*approval.ConditionNodeData).Branches
		}
	}

	if startCount != 1 {
		return fmt.Errorf("%w, found %d", errStartNodeCount, startCount)
	}

	if endCount < 1 {
		return fmt.Errorf("%w, found %d", errEndNodeCount, endCount)
	}

	// --- Phase 2: Edge validation & adjacency ---
	var (
		edgeIDs     = collections.NewHashSet[string]()
		outEdges    = make(map[string][]approval.EdgeDefinition, len(def.Nodes))
		inDegree    = make(map[string]int, len(def.Nodes))
		adjacency   = make(map[string][]string, len(def.Nodes))
		reversedAdj = make(map[string][]string, len(def.Nodes))
	)

	for _, edge := range def.Edges {
		if edge.ID == "" {
			return errEmptyEdgeID
		}

		if !edgeIDs.Add(edge.ID) {
			return fmt.Errorf("%w: %q", errDuplicateEdgeID, edge.ID)
		}

		if !nodeIDs.Contains(edge.Source) {
			return fmt.Errorf("%w: edge %q references %q", errUnknownSourceNode, edge.ID, edge.Source)
		}

		if !nodeIDs.Contains(edge.Target) {
			return fmt.Errorf("%w: edge %q references %q", errUnknownTargetNode, edge.ID, edge.Target)
		}

		outEdges[edge.Source] = append(outEdges[edge.Source], edge)
		inDegree[edge.Target]++
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
		reversedAdj[edge.Target] = append(reversedAdj[edge.Target], edge.Source)
	}

	// --- Phase 3: Degree constraints ---
	if inDegree[startID] > 0 {
		return errStartIncoming
	}

	if len(outEdges[startID]) != 1 {
		return fmt.Errorf("%w, found %d", errStartOutgoing, len(outEdges[startID]))
	}

	for _, endID := range endIDs {
		if len(outEdges[endID]) > 0 {
			return fmt.Errorf("%w: %q", errEndOutgoing, endID)
		}

		if inDegree[endID] == 0 {
			return fmt.Errorf("%w: %q", errEndIncoming, endID)
		}
	}

	for _, node := range def.Nodes {
		if node.Kind == approval.NodeStart || node.Kind == approval.NodeEnd {
			continue
		}

		outs := outEdges[node.ID]

		switch node.Kind {
		case approval.NodeCondition:
			if err := validateConditionEdges(node.ID, condBranches[node.ID], outs); err != nil {
				return err
			}
		default:
			if len(outs) != 1 {
				return fmt.Errorf("%w: node %q has %d", errNodeOutgoingCount, node.ID, len(outs))
			}

			if outs[0].SourceHandle != nil {
				return fmt.Errorf("%w: node %q", errNodeSourceHandle, node.ID)
			}
		}
	}

	// --- Phase 4: Topology ---
	nodeIDSlice := streams.MapTo(streams.FromSlice(def.Nodes), func(n approval.NodeDefinition) string {
		return n.ID
	}).Collect()

	if detectCycle(nodeIDSlice, adjacency) {
		return errGraphCycle
	}

	reachable := collectReachable(adjacency, startID)
	if reachable.Size() != nodeIDs.Size() {
		for _, node := range def.Nodes {
			if !reachable.Contains(node.ID) {
				return fmt.Errorf("%w: %q", errNodeUnreachable, node.ID)
			}
		}
	}

	canReachEnd := collectReachable(reversedAdj, endIDs...)
	if canReachEnd.Size() != nodeIDs.Size() {
		for _, node := range def.Nodes {
			if !canReachEnd.Contains(node.ID) {
				return fmt.Errorf("%w: %q", errNodeCannotReachEnd, node.ID)
			}
		}
	}

	return nil
}

// validateConditionEdges validates that a condition node's outgoing edges match its branches exactly.
func validateConditionEdges(nodeID string, branches []approval.ConditionBranch, outs []approval.EdgeDefinition) error {
	if len(branches) < 2 {
		return fmt.Errorf("%w: node %q has %d", errCondMinBranches, nodeID, len(branches))
	}

	branchIDs := collections.NewHashSet[string]()

	var defaultCount int

	for _, branch := range branches {
		if branch.ID == "" {
			return fmt.Errorf("%w: node %q", errCondEmptyBranchID, nodeID)
		}

		if !branchIDs.Add(branch.ID) {
			return fmt.Errorf("%w: node %q branch %q", errCondDupBranchID, nodeID, branch.ID)
		}

		if branch.IsDefault {
			defaultCount++
		}
	}

	if defaultCount != 1 {
		return fmt.Errorf("%w: node %q has %d", errCondDefaultCount, nodeID, defaultCount)
	}

	edgeHandles := collections.NewHashSet[string]()

	for _, edge := range outs {
		if edge.SourceHandle == nil {
			return fmt.Errorf("%w: node %q edge %q", errCondMissingHandle, nodeID, edge.ID)
		}

		handle := *edge.SourceHandle
		if !branchIDs.Contains(handle) {
			return fmt.Errorf("%w: node %q edge %q handle %q", errCondUnknownHandle, nodeID, edge.ID, handle)
		}

		if !edgeHandles.Add(handle) {
			return fmt.Errorf("%w: node %q handle %q", errCondDupHandle, nodeID, handle)
		}
	}

	if edgeHandles.Size() != branchIDs.Size() {
		for _, branch := range branches {
			if !edgeHandles.Contains(branch.ID) {
				return fmt.Errorf("%w: node %q branch %q", errCondBranchNoEdge, nodeID, branch.ID)
			}
		}
	}

	return nil
}

// detectCycle returns true if the directed graph contains a cycle (DFS coloring).
func detectCycle(nodes []string, adjacency map[string][]string) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	var (
		color = make(map[string]int, len(nodes))
		visit func(string) bool
	)

	visit = func(node string) bool {
		color[node] = gray

		for _, next := range adjacency[node] {
			if color[next] == gray || (color[next] == white && visit(next)) {
				return true
			}
		}

		color[node] = black

		return false
	}

	for _, node := range nodes {
		if color[node] == white && visit(node) {
			return true
		}
	}

	return false
}

// collectReachable returns the set of nodes reachable from any of the start nodes via BFS.
func collectReachable(adjacency map[string][]string, starts ...string) collections.Set[string] {
	visited := collections.NewHashSet[string]()
	queue := make([]string, len(starts))

	for i, start := range starts {
		visited.Add(start)
		queue[i] = start
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range adjacency[current] {
			if visited.Add(next) {
				queue = append(queue, next)
			}
		}
	}

	return visited
}
