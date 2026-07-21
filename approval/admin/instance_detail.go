package admin

import (
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// InstanceDetail represents the full admin detail view of an approval
// instance. Each top-level field is one renderable concern: the instance's
// runtime state, the version-pinned host form-designer document returned
// verbatim (the counterpart of FlowGraph, which pins the flow definition),
// the node-by-node timeline, and the progress-annotated flow graph. The raw
// audit trail stays available through the paginated admin action-log query.
type InstanceDetail struct {
	Instance   InstanceDetailInfo         `json:"instance"`
	FormSchema json.RawMessage            `json:"formSchema,omitempty"`
	Timeline   []approval.TimelineEntry   `json:"timeline"`
	FlowGraph  approval.InstanceFlowGraph `json:"flowGraph"`
}

// InstanceDetailInfo carries the instance's runtime state within an admin
// detail view.
type InstanceDetailInfo struct {
	InstanceID    string `json:"instanceId"`
	InstanceNo    string `json:"instanceNo"`
	Title         string `json:"title"`
	TenantID      string `json:"tenantId"`
	FlowID        string `json:"flowId"`
	FlowCode      string `json:"flowCode"`
	FlowName      string `json:"flowName"`
	FlowVersionID string `json:"flowVersionId"`
	// Labels are the flow's host-owned selection metadata, read from the
	// mutable flow at query time (like FlowName — display identity, not a
	// version-pinned snapshot).
	Labels          map[string]string `json:"labels,omitempty"`
	Applicant       approval.UserInfo `json:"applicant"`
	Status          string            `json:"status"`
	CurrentNodeID   *string           `json:"currentNodeId,omitempty"`
	CurrentNodeName *string           `json:"currentNodeName,omitempty"`
	BusinessRef     *string           `json:"businessRef,omitempty"`
	FormData        map[string]any    `json:"formData,omitempty"`
	CreatedAt       timex.DateTime    `json:"createdAt"`
	FinishedAt      *timex.DateTime   `json:"finishedAt,omitempty"`
}

// ActionLog represents an action log entry in the admin audit view. Person
// references are uniform UserInfo snapshots captured at action time.
type ActionLog struct {
	LogID            string              `json:"logId"`
	Action           string              `json:"action"`
	NodeID           *string             `json:"nodeId,omitempty"`
	TaskID           *string             `json:"taskId,omitempty"`
	Operator         approval.UserInfo   `json:"operator"`
	TransferTo       *approval.UserInfo  `json:"transferTo,omitempty"`
	RollbackToNodeID *string             `json:"rollbackToNodeId,omitempty"`
	AddedAssignees   []approval.UserInfo `json:"addedAssignees,omitempty"`
	RemovedAssignees []approval.UserInfo `json:"removedAssignees,omitempty"`
	CCUsers          []approval.UserInfo `json:"ccUsers,omitempty"`
	Opinion          *string             `json:"opinion,omitempty"`
	Attachments      []string            `json:"attachments,omitempty"`
	CreatedAt        timex.DateTime      `json:"createdAt"`
}
