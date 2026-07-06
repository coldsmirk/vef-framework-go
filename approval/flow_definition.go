package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	// ErrUnknownNodeKind reports a node definition whose kind is not one of
	// the NodeKind enum values.
	ErrUnknownNodeKind = errors.New("unknown node kind")

	// ErrNodeDataUnmarshal reports node data JSON that failed to decode into
	// the kind's typed payload.
	ErrNodeDataUnmarshal = errors.New("node data unmarshal failed")
)

// FlowDefinition represents the structure of a flow definition JSON (React Flow compatible).
type FlowDefinition struct {
	Nodes []NodeDefinition `json:"nodes"`
	Edges []EdgeDefinition `json:"edges"`
}

// Position represents the visual position of a node on the canvas.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeDefinition represents a node in the flow definition. The node kind
// travels in `kind`, deliberately not React Flow's `type`: in React Flow,
// `type` selects the rendering component and belongs to whichever client
// renders the graph, so the persisted contract carries the business
// discriminator and stays decoupled from rendering concerns.
type NodeDefinition struct {
	ID       string          `json:"id"`
	Kind     NodeKind        `json:"kind"`
	Position Position        `json:"position"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// ParseData parses Data into the appropriate typed struct based on Kind.
func (nd *NodeDefinition) ParseData() (NodeData, error) {
	var target NodeData

	switch nd.Kind {
	case NodeStart:
		target = &StartNodeData{}
	case NodeEnd:
		target = &EndNodeData{}
	case NodeApproval:
		target = &ApprovalNodeData{}
	case NodeHandle:
		target = &HandleNodeData{}
	case NodeCC:
		target = &CCNodeData{}
	case NodeCondition:
		target = &ConditionNodeData{}
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownNodeKind, nd.Kind)
	}

	if len(nd.Data) > 0 {
		if err := json.Unmarshal(nd.Data, target); err != nil {
			return nil, fmt.Errorf("%w for %q: %w", ErrNodeDataUnmarshal, nd.Kind, err)
		}
	}

	return target, nil
}

// EdgeDefinition represents a connection between nodes.
type EdgeDefinition struct {
	ID           string         `json:"id"`
	Source       string         `json:"source"`
	Target       string         `json:"target"`
	SourceHandle *string        `json:"sourceHandle,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
}

// InstanceNoGenerator generates unique instance numbers for flow instances.
type InstanceNoGenerator interface {
	// Generate creates a unique instance number for a flow identified by flowCode.
	Generate(ctx context.Context, flowCode string) (string, error)
}
