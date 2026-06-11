package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// NodeAutoPassedEvent fired when a node passes without any human decision:
// auto-pass execution type, empty-assignee auto-pass, or same-applicant
// auto-pass. Reason carries which rule produced the pass so audit timelines
// can render the skipped step.
type NodeAutoPassedEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	NodeID       string         `json:"nodeId"`
	Reason       string         `json:"reason"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeAutoPassedEvent(instanceID, tenantID, nodeID, reason string) *NodeAutoPassedEvent {
	return &NodeAutoPassedEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		NodeID:       nodeID,
		Reason:       reason,
		OccurredTime: timex.Now(),
	}
}

func (*NodeAutoPassedEvent) EventType() string { return EventTypeNodeAutoPassed }
