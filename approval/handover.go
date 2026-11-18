package approval

import "context"

// Handover identifies the successor of a user who no longer receives approval work.
type Handover struct {
	// Successor is nil when the user left without a successor.
	Successor *UserInfo
	// Reason is the display label recorded beside the predecessor.
	Reason string
}

// HandoverContext scopes a handover lookup to the work being assigned.
type HandoverContext struct {
	TenantID string
	Instance *Instance
	Node     *FlowNode
}

// HandoverResolver supplies the host's approval handover records.
type HandoverResolver interface {
	// ResolveHandovers returns entries for users who handed over their approval work.
	// Users absent from the result continue to receive work.
	ResolveHandovers(ctx context.Context, hc *HandoverContext, userIDs []string) (map[string]Handover, error)
}

// ApplyHandoversInput selects whose work to move and identifies the operator.
type ApplyHandoversInput struct {
	UserIDs  []string
	Operator UserInfo
	Caller   CallerContext
}

// HandoverReport lists moved work, work without successors, and instances awaiting the applicant.
type HandoverReport struct {
	Transferred       []HandoverItem `json:"transferred"`
	Unassigned        []HandoverItem `json:"unassigned"`
	AwaitingApplicant []string       `json:"awaitingApplicant"`
}

// HandoverItem identifies a task or read-confirmation receipt and its people.
type HandoverItem struct {
	InstanceID  string    `json:"instanceId"`
	TaskID      *string   `json:"taskId,omitempty"`
	CCRecordID  *string   `json:"ccRecordId,omitempty"`
	Predecessor UserInfo  `json:"predecessor"`
	Successor   *UserInfo `json:"successor,omitempty"`
}
