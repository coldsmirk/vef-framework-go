package service

import "errors"

// Flow-definition validation sentinels. These are internal errors wrapped
// into shared.ErrInvalidFlowDesign at the command layer and never surfaced raw.
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
	errUnexpectedCondData = errors.New("unexpected condition node data type")
	errInvalidCCKind      = errors.New("invalid cc kind")
)
