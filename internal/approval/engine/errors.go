package engine

import "errors"

var (
	// Engine errors.
	ErrNoMatchingEdge    = errors.New("no matching outgoing edge for node")
	ErrProcessorNotFound = errors.New("node processor not found for node kind")
	ErrMaxNodeDepth      = errors.New("max node processing depth exceeded")

	// Approval node errors.
	ErrAssigneeServiceNotConfigured = errors.New("assignee service is not configured")

	// Condition node errors.
	ErrNoBranches       = errors.New("condition node has no branches")
	ErrNoMatchingBranch = errors.New("no matching branch and no default branch")

	// ErrInvalidTransition signals that a requested state transition is not
	// permitted by the state machine, or that a concurrent writer already
	// advanced the target row off the expected `from` status.
	ErrInvalidTransition = errors.New("invalid state transition")

	// CompiledFlow errors.
	ErrFlowMissingStartNode  = errors.New("flow has no start node")
	ErrFlowMissingTargetNode = errors.New("compiled flow is missing target node")
	ErrFlowNoNodes           = errors.New("compiled flow has no nodes for version")

	// Process result errors.
	errUnknownNodeAction = errors.New("unknown node action")
	errAmbiguousEdges    = errors.New("ambiguous edges")
)
