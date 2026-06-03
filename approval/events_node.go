package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// NodeEnteredEvent fired when instance enters a new node.
type NodeEnteredEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	NodeID       string         `json:"nodeId"`
	NodeName     string         `json:"nodeName"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeEnteredEvent(instanceID, tenantID, nodeID, nodeName string) *NodeEnteredEvent {
	return &NodeEnteredEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		NodeID:       nodeID,
		NodeName:     nodeName,
		OccurredTime: timex.Now(),
	}
}

func (*NodeEnteredEvent) EventType() string { return EventTypeNodeEntered }

// NodeAutoPassedEvent fired when a node is auto-passed.
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
