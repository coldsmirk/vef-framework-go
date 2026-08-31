package service

import (
	"fmt"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
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
type FlowDefinitionService struct {
	// aggregateKinds is the set of aggregate kinds with a registered
	// approval.Aggregator. Deploy validation accepts exactly this set, so a
	// host-registered aggregate (vef.ProvideApprovalAggregator) becomes
	// deployable with no framework changes — the open-closed contract the
	// Aggregator interface promises.
	aggregateKinds collections.Set[approval.AggregateKind]
	// assigneeKinds and ccKinds map each registered kind to the designer input
	// it requires. They are the same open-closed contract applied to the two
	// node-level principal vocabularies: deploy accepts exactly the kinds with
	// a registered resolver, and enforces that kind's declared input, so a host
	// kind is validated like a built-in without a line of framework code.
	assigneeKinds map[approval.AssigneeKind]approval.SelectionMode
	ccKinds       map[approval.CCKind]approval.SelectionMode
}

// FlowDefinitionOption configures a FlowDefinitionService.
type FlowDefinitionOption func(*FlowDefinitionService)

// WithAggregateKinds sets the aggregate kinds deploy validation accepts.
func WithAggregateKinds(kinds ...approval.AggregateKind) FlowDefinitionOption {
	return func(s *FlowDefinitionService) {
		s.aggregateKinds = collections.NewHashSetFrom(kinds...)
	}
}

// WithAssigneeKinds sets the assignee kinds deploy validation accepts,
// together with the input each requires.
func WithAssigneeKinds(descriptors ...approval.KindDescriptor[approval.AssigneeKind]) FlowDefinitionOption {
	return func(s *FlowDefinitionService) {
		s.assigneeKinds = shared.SelectionIndex(descriptors)
	}
}

// WithCCKinds sets the CC kinds deploy validation accepts, together with the
// input each requires.
func WithCCKinds(descriptors ...approval.KindDescriptor[approval.CCKind]) FlowDefinitionOption {
	return func(s *FlowDefinitionService) {
		s.ccKinds = shared.SelectionIndex(descriptors)
	}
}

// NewFlowDefinitionService creates a new FlowDefinitionService. Every kind
// vocabulary defaults to the framework built-ins, so a test constructs the
// service with no arguments; the production module passes the full
// boot-registered sets.
func NewFlowDefinitionService(opts ...FlowDefinitionOption) *FlowDefinitionService {
	svc := &FlowDefinitionService{
		aggregateKinds: collections.NewHashSetFrom(approval.AggregateSum, approval.AggregateCount, approval.AggregateAvg),
		assigneeKinds:  shared.SelectionIndex(strategy.BuiltinAssigneeKinds()),
		ccKinds:        shared.SelectionIndex(strategy.BuiltinCCKinds()),
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc
}

// nodeScan holds the Phase 1 node-validation outputs that the later edge,
// degree, and topology phases consume. Carrying them in one struct (rather
// than a wide return list) keeps each phase helper within revive's
// function-result-limit while still flowing the cross-phase state through.
type nodeScan struct {
	nodeIDs      collections.Set[string]
	taskNodeIDs  collections.Set[string]
	parsed       map[string]approval.NodeData
	condBranches map[string][]approval.ConditionBranch
	startID      string
	endIDs       []string
}

// edgeScan holds the Phase 2 edge-validation outputs (adjacency + degree maps)
// that the degree-constraint and topology phases consume.
type edgeScan struct {
	outEdges    map[string][]approval.EdgeDefinition
	inDegree    map[string]int
	adjacency   map[string][]string
	reversedAdj map[string][]string
}

// ValidateFlowDefinition validates a flow graph and returns the parsed node
// data (which deploy persists verbatim — it is never re-parsed). It runs four
// phases in sequence — node validation, edge/adjacency, degree constraints,
// topology — each extracted into a focused helper; later phases consume the
// scan state earlier ones produce.
func (s *FlowDefinitionService) ValidateFlowDefinition(def *approval.FlowDefinition) (map[string]approval.NodeData, error) {
	if len(def.Nodes) == 0 {
		return nil, errNoNodes
	}

	nodes, err := s.validateNodes(def)
	if err != nil {
		return nil, err
	}

	edges, err := validateEdges(def, nodes.nodeIDs)
	if err != nil {
		return nil, err
	}

	if err := validateDegreeConstraints(def, nodes, edges); err != nil {
		return nil, err
	}

	if err := validateTopology(def, nodes, edges); err != nil {
		return nil, err
	}

	return nodes.parsed, nil
}

// validateNodes performs Phase 1: per-node ID/kind/data validation, start/end
// counts, and the cross-node rollback-target reference check.
func (s *FlowDefinitionService) validateNodes(def *approval.FlowDefinition) (*nodeScan, error) {
	scan := &nodeScan{
		nodeIDs:      collections.NewHashSet[string](),
		taskNodeIDs:  collections.NewHashSet[string](),
		parsed:       make(map[string]approval.NodeData, len(def.Nodes)),
		condBranches: make(map[string][]approval.ConditionBranch),
	}

	rollbackRefs := make(map[string][]string)

	var startCount, endCount int

	for i := range def.Nodes {
		node := &def.Nodes[i]

		if node.ID == "" {
			return nil, errEmptyNodeID
		}

		if !scan.nodeIDs.Add(node.ID) {
			return nil, fmt.Errorf("%w: %q", errDuplicateNodeID, node.ID)
		}

		if !validNodeKinds.Contains(node.Kind) {
			return nil, fmt.Errorf("%w: %q for node %q", errInvalidNodeKind, node.Kind, node.ID)
		}

		data, err := node.ParseData()
		if err != nil {
			return nil, fmt.Errorf("parse node %q data: %w", node.ID, err)
		}

		if err := s.validateNodeConfig(node.ID, data); err != nil {
			return nil, err
		}

		scan.parsed[node.ID] = data

		switch node.Kind {
		case approval.NodeStart:
			startCount++
			scan.startID = node.ID
		case approval.NodeEnd:
			endCount++

			scan.endIDs = append(scan.endIDs, node.ID)
		case approval.NodeCondition:
			cnd, ok := data.(*approval.ConditionNodeData)
			if !ok {
				return nil, fmt.Errorf("node %q: %w", node.ID, errUnexpectedCondData)
			}

			scan.condBranches[node.ID] = cnd.Branches

		case approval.NodeApproval, approval.NodeHandle:
			scan.taskNodeIDs.Add(node.ID)

			if ad, ok := data.(*approval.ApprovalNodeData); ok && len(ad.RollbackTargetKeys) > 0 {
				rollbackRefs[node.ID] = ad.RollbackTargetKeys
			}
		}
	}

	if startCount != 1 {
		return nil, fmt.Errorf("%w, found %d", errStartNodeCount, startCount)
	}

	if endCount < 1 {
		return nil, fmt.Errorf("%w, found %d", errEndNodeCount, endCount)
	}

	if err := validateRollbackRefs(rollbackRefs, scan.taskNodeIDs); err != nil {
		return nil, err
	}

	return scan, nil
}

// validateRollbackRefs resolves cross-node rollback target keys — possible
// only once every node ID is known. Targets must be task nodes (the designer
// offers exactly approval / handle candidates) and never the node itself.
func validateRollbackRefs(rollbackRefs map[string][]string, taskNodeIDs collections.Set[string]) error {
	for nodeID, targetKeys := range rollbackRefs {
		for _, key := range targetKeys {
			if key == nodeID {
				return fmt.Errorf("%w: node %q", errRollbackTargetSelf, nodeID)
			}

			if !taskNodeIDs.Contains(key) {
				return fmt.Errorf("%w: %q in node %q", errRollbackTargetUnknown, key, nodeID)
			}
		}
	}

	return nil
}

// validateEdges performs Phase 2: per-edge ID/endpoint validation, building the
// adjacency and degree maps the later phases consume.
func validateEdges(def *approval.FlowDefinition, nodeIDs collections.Set[string]) (*edgeScan, error) {
	edgeIDs := collections.NewHashSet[string]()
	scan := &edgeScan{
		outEdges:    make(map[string][]approval.EdgeDefinition, len(def.Nodes)),
		inDegree:    make(map[string]int, len(def.Nodes)),
		adjacency:   make(map[string][]string, len(def.Nodes)),
		reversedAdj: make(map[string][]string, len(def.Nodes)),
	}

	for _, edge := range def.Edges {
		if edge.ID == "" {
			return nil, errEmptyEdgeID
		}

		if !edgeIDs.Add(edge.ID) {
			return nil, fmt.Errorf("%w: %q", errDuplicateEdgeID, edge.ID)
		}

		if !nodeIDs.Contains(edge.Source) {
			return nil, fmt.Errorf("%w: edge %q references %q", errUnknownSourceNode, edge.ID, edge.Source)
		}

		if !nodeIDs.Contains(edge.Target) {
			return nil, fmt.Errorf("%w: edge %q references %q", errUnknownTargetNode, edge.ID, edge.Target)
		}

		scan.outEdges[edge.Source] = append(scan.outEdges[edge.Source], edge)
		scan.inDegree[edge.Target]++
		scan.adjacency[edge.Source] = append(scan.adjacency[edge.Source], edge.Target)
		scan.reversedAdj[edge.Target] = append(scan.reversedAdj[edge.Target], edge.Source)
	}

	return scan, nil
}

// validateDegreeConstraints performs Phase 3: start/end in/out-degree rules and
// the per-node outgoing-edge constraints (condition branch coverage, single
// unguarded out-edge for everything else).
func validateDegreeConstraints(def *approval.FlowDefinition, nodes *nodeScan, edges *edgeScan) error {
	if edges.inDegree[nodes.startID] > 0 {
		return errStartIncoming
	}

	if len(edges.outEdges[nodes.startID]) != 1 {
		return fmt.Errorf("%w, found %d", errStartOutgoing, len(edges.outEdges[nodes.startID]))
	}

	for _, endID := range nodes.endIDs {
		if len(edges.outEdges[endID]) > 0 {
			return fmt.Errorf("%w: %q", errEndOutgoing, endID)
		}

		if edges.inDegree[endID] == 0 {
			return fmt.Errorf("%w: %q", errEndIncoming, endID)
		}
	}

	for _, node := range def.Nodes {
		if node.Kind == approval.NodeStart || node.Kind == approval.NodeEnd {
			continue
		}

		outs := edges.outEdges[node.ID]

		switch node.Kind {
		case approval.NodeCondition:
			if err := validateConditionEdges(node.ID, nodes.condBranches[node.ID], outs); err != nil {
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

	return nil
}

// validateTopology performs Phase 4: cycle detection plus forward (from start)
// and backward (to an end) reachability over the whole node set.
func validateTopology(def *approval.FlowDefinition, nodes *nodeScan, edges *edgeScan) error {
	nodeIDSlice := make([]string, 0, len(def.Nodes))
	for i := range def.Nodes {
		nodeIDSlice = append(nodeIDSlice, def.Nodes[i].ID)
	}

	if detectCycle(nodeIDSlice, edges.adjacency) {
		return errGraphCycle
	}

	reachable := collectReachable(edges.adjacency, nodes.startID)
	if reachable.Size() != nodes.nodeIDs.Size() {
		for _, node := range def.Nodes {
			if !reachable.Contains(node.ID) {
				return fmt.Errorf("%w: %q", errNodeUnreachable, node.ID)
			}
		}
	}

	canReachEnd := collectReachable(edges.reversedAdj, nodes.endIDs...)
	if canReachEnd.Size() != nodes.nodeIDs.Size() {
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
