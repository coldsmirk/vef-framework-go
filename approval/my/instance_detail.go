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
	// MyTask is the viewer's own actionable context: the pending task
	// process_task should target plus the node-level configuration the client
	// needs to build the action UI without re-deriving engine semantics. Nil
	// when the viewer holds no pending task on this instance.
	MyTask *ViewerTask `json:"myTask,omitempty"`
}

// ViewerTask is the viewer's pending task within a detail view, with the
// current node's action configuration resolved server-side.
type ViewerTask struct {
	TaskID string `json:"taskId"`
	NodeID string `json:"nodeId"`
	// IsOpinionRequired mirrors the node config: approve/reject must carry a
	// non-empty opinion when set.
	IsOpinionRequired bool `json:"isOpinionRequired"`
	// AddAssigneeTypes lists the positions the node allows for dynamic
	// assignee addition; empty when adding assignees is not allowed.
	AddAssigneeTypes []approval.AddAssigneeType `json:"addAssigneeTypes,omitempty"`
	// RollbackTargets are the valid rollback destinations, resolved from the
	// node's rollback config and the instance's visit trail exactly like the
	// rollback command validates them; empty when rollback is not allowed.
	RollbackTargets []RollbackTarget `json:"rollbackTargets,omitempty"`
}

// RollbackTarget is one valid rollback destination.
type RollbackTarget struct {
	NodeID string `json:"nodeId"`
	Name   string `json:"name"`
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
