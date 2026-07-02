package approval

import (
	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NodeProgressStatus describes how far an instance has advanced through a flow
// node, for the read-only progress rendering of the flow graph. Except for
// pending (a node with no visit yet), the values mirror NodeVisitStatus — a
// reached node reports the outcome of its latest visit, so the graph and the
// timeline share one status vocabulary.
type NodeProgressStatus string

const (
	// NodeProgressPending marks a node the instance has not reached yet.
	NodeProgressPending NodeProgressStatus = "pending"
	// NodeProgressActive marks a node the instance is currently sitting on.
	NodeProgressActive NodeProgressStatus = "active"
	// NodeProgressPassed marks a node the instance has already passed.
	NodeProgressPassed NodeProgressStatus = "passed"
	// NodeProgressRejected marks the node whose decision rejected the instance.
	NodeProgressRejected NodeProgressStatus = "rejected"
	// NodeProgressReturned marks a node from which the flow was sent back.
	NodeProgressReturned NodeProgressStatus = "returned"
	// NodeProgressCanceled marks a node whose visit was cut short by a
	// withdraw or terminate.
	NodeProgressCanceled NodeProgressStatus = "canceled"
)

// InstanceFlowGraph is a React Flow–ready, read-only projection of an instance's
// flow definition annotated with runtime progress. Its Nodes and Edges map
// directly onto React Flow's node/edge shape so a client can render it without
// reshaping — except the node kind, which stays in `kind` (mirroring
// NodeDefinition's wire format): React Flow's `type` selects the rendering
// component and belongs to the client, so the graph carries the business
// discriminator instead. It is distinct from the editor-facing
// shared.FlowGraph (raw definition rows without progress): this one is pinned
// to the instance's version and carries per-node progress.
type InstanceFlowGraph struct {
	Nodes []FlowGraphNode `json:"nodes"`
	Edges []FlowGraphEdge `json:"edges"`
}

// FlowGraphNode is one React Flow node. ID is the React Flow identity — the
// design-time node key that Position and Edges reference. NodeID is the node's
// persistent flow-node id: the value action-log nodeId / rollbackToNodeId carry
// and the process_task rollback API expects as targetNodeId, so a client can map
// those references onto this graph and drive a rollback without a second lookup.
type FlowGraphNode struct {
	ID       string            `json:"id"`
	NodeID   string            `json:"nodeId"`
	Kind     NodeKind          `json:"kind"`
	Position Position          `json:"position"`
	Data     FlowGraphNodeData `json:"data"`
}

// FlowGraphNodeData is the payload inside a React Flow node's `data` extension
// point — the only part of the graph that is ours to shape; the surrounding
// node/edge structure stays React Flow's contract verbatim. It mirrors the
// timeline entry of the same node — the node's label, its approval semantics
// (populated only for approval nodes — handle nodes claim-and-do, they do not
// decide), its progress status, and everything that happened there:
// participants, CC recipients, and side-action activities (including the
// submit/resubmit on the start node), each aggregated across the node's visits
// in traversal order so a rollback → redo history stays round-by-round.
// StartedAt/FinishedAt span from the first entry into the node to its latest
// conclusion (FinishedAt is nil while the node is executing). Enum-derived
// fields are emitted as strings to match the rest of the detail response.
type FlowGraphNodeData struct {
	Name           string             `json:"name"`
	Status         NodeProgressStatus `json:"status"`
	ExecutionType  string             `json:"executionType,omitempty"`
	ApprovalMethod string             `json:"approvalMethod,omitempty"`
	PassRule       string             `json:"passRule,omitempty"`
	PassRatio      *decimal.Decimal   `json:"passRatio,omitempty"`
	Participants   []NodeParticipant  `json:"participants,omitempty"`
	CCRecipients   []CCRecipient      `json:"ccRecipients,omitempty"`
	Activities     []Activity         `json:"activities,omitempty"`
	StartedAt      *timex.DateTime    `json:"startedAt,omitempty"`
	FinishedAt     *timex.DateTime    `json:"finishedAt,omitempty"`
}

// FlowGraphEdge is one React Flow edge connecting two nodes by their ids.
type FlowGraphEdge struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	SourceHandle *string `json:"sourceHandle,omitempty"`
}
