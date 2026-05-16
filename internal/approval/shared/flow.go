package shared

import "github.com/coldsmirk/vef-framework-go/approval"

// FlowGraph contains the complete flow graph for a version.
type FlowGraph struct {
	Flow    *approval.Flow        `json:"flow"`
	Version *approval.FlowVersion `json:"version"`
	Nodes   []approval.FlowNode   `json:"nodes"`
	Edges   []approval.FlowEdge   `json:"edges"`
}
