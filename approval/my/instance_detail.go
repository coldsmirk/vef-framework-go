package my

import (
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// InstanceDetail is the self-service detail view for an approval instance.
// Each top-level field is one renderable concern: the instance's runtime
// state, the version-pinned host form-designer document returned verbatim
// (the schema the instance was submitted under — the counterpart of
// FlowGraph, which pins the flow definition), the node-by-node timeline for
// the transit-record view, the progress-annotated flow graph for the
// read-only diagram, and the viewer-specific action set.
type InstanceDetail struct {
	Instance         InstanceInfo               `json:"instance"`
	FormSchema       json.RawMessage            `json:"formSchema,omitempty"`
	Timeline         []approval.TimelineEntry   `json:"timeline"`
	FlowGraph        approval.InstanceFlowGraph `json:"flowGraph"`
	AvailableActions []string                   `json:"availableActions"`
	// FieldPermissions is the viewer-scoped field interactivity projection,
	// materialized for every top-level form field — the client applies it
	// verbatim (no default resolution). Instance.FormData is already stripped
	// of the fields this viewer may not see.
	FieldPermissions map[string]approval.Permission `json:"fieldPermissions,omitempty"`
}

// InstanceInfo holds the instance's runtime state within a detail view.
type InstanceInfo struct {
	InstanceID      string            `json:"instanceId"`
	InstanceNo      string            `json:"instanceNo"`
	Title           string            `json:"title"`
	FlowName        string            `json:"flowName"`
	FlowIcon        *string           `json:"flowIcon,omitempty"`
	Applicant       approval.UserInfo `json:"applicant"`
	Status          string            `json:"status"`
	CurrentNodeID   *string           `json:"currentNodeId,omitempty"`
	CurrentNodeName *string           `json:"currentNodeName,omitempty"`
	BusinessRef     *string           `json:"businessRef,omitempty"`
	FormData        map[string]any    `json:"formData,omitempty"`
	CreatedAt       timex.DateTime    `json:"createdAt"`
	FinishedAt      *timex.DateTime   `json:"finishedAt,omitempty"`
}
