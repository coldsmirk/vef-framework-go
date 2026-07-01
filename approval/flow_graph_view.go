package approval

import (
	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NodeProgressStatus describes how far an instance has advanced through a flow
// node, for the read-only progress rendering of the flow graph.
type NodeProgressStatus string

const (
	// NodeProgressPending marks a node the instance has not reached yet.
	NodeProgressPending NodeProgressStatus = "pending"
	// NodeProgressCurrent marks a node the instance is currently sitting on.
	NodeProgressCurrent NodeProgressStatus = "current"
	// NodeProgressCompleted marks a node the instance has already passed.
	NodeProgressCompleted NodeProgressStatus = "completed"
)

// InstanceFlowGraph is a React Flow–ready, read-only projection of an instance's
// flow definition annotated with runtime progress. Its Nodes and Edges map
// directly onto React Flow's node/edge shape so a client can render it without
// reshaping — a node's Kind is what the client converts into React Flow's own
// `type` field. It is distinct from the editor-facing shared.FlowGraph (raw
// definition rows without progress): this one is pinned to the instance's
// version and carries per-node progress.
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
	Kind     string            `json:"kind"`
	Position Position          `json:"position"`
	Data     FlowGraphNodeData `json:"data"`
}

// FlowGraphNodeData is the per-node payload a read-only node component renders:
// the node's label, its approval semantics (populated only for approval nodes —
// handle nodes claim-and-do, they do not decide), its progress status, and —
// for reached approval/handle nodes — who acted on it. Enum-derived fields are
// emitted as strings to match the rest of the detail response.
type FlowGraphNodeData struct {
	Name           string                 `json:"name"`
	Status         NodeProgressStatus     `json:"status"`
	ExecutionType  string                 `json:"executionType,omitempty"`
	ApprovalMethod string                 `json:"approvalMethod,omitempty"`
	PassRule       string                 `json:"passRule,omitempty"`
	PassRatio      *decimal.Decimal       `json:"passRatio,omitempty"`
	Participants   []FlowGraphParticipant `json:"participants,omitempty"`
}

// FlowGraphParticipant is one assignee's outcome at an approval/handle node,
// assembled from that node's tasks (status) and the log that finished each task
// (opinion, time).
type FlowGraphParticipant struct {
	UserID     string          `json:"userId"`
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Opinion    *string         `json:"opinion,omitempty"`
	ActionTime *timex.DateTime `json:"actionTime,omitempty"`
}

// FlowGraphEdge is one React Flow edge connecting two nodes by their ids.
type FlowGraphEdge struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	SourceHandle *string `json:"sourceHandle,omitempty"`
}

// UserBrief is a minimal {id, name} reference used in action-log projections
// (added/removed assignees, CC recipients) so a client can render names without
// a second lookup.
type UserBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
