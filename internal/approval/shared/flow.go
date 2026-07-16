package shared

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// FlowGraph contains the complete flow graph for a version.
type FlowGraph struct {
	Flow    *approval.Flow        `json:"flow"`
	Version *approval.FlowVersion `json:"version"`
	Nodes   []approval.FlowNode   `json:"nodes"`
	Edges   []approval.FlowEdge   `json:"edges"`
}

// FlowVersionSummary is the version-list projection of a FlowVersion:
// identity and lifecycle metadata without the definition payloads
// (FlowSchema / FormSchema / FormFields), which a version list never renders.
// A single version's full definition is fetched through get_graph with an
// explicit version id.
type FlowVersionSummary struct {
	ID          string                 `json:"id"`
	FlowID      string                 `json:"flowId"`
	Version     int                    `json:"version"`
	Status      approval.VersionStatus `json:"status"`
	Description *string                `json:"description,omitempty"`
	StorageMode approval.StorageMode   `json:"storageMode"`
	PublishedAt *timex.DateTime        `json:"publishedAt,omitempty"`
	PublishedBy *string                `json:"publishedBy,omitempty"`
	CreatedAt   timex.DateTime         `json:"createdAt"`
	CreatedBy   string                 `json:"createdBy"`
}
