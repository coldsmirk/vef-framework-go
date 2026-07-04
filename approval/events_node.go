package approval

// NodeAutoPassedEvent fired when a node passes without any human decision:
// auto-pass execution type, empty-assignee auto-pass, or same-applicant
// auto-pass. Reason carries which rule produced the pass so audit timelines
// can render the skipped step.
type NodeAutoPassedEvent struct {
	InstanceEventBase

	NodeID   string `json:"nodeId"`
	NodeName string `json:"nodeName"`
	Reason   string `json:"reason"`
}

func NewNodeAutoPassedEvent(instance *Instance, node *FlowNode, reason string) *NodeAutoPassedEvent {
	return &NodeAutoPassedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		NodeID:            node.ID,
		NodeName:          node.Name,
		Reason:            reason,
	}
}

func (*NodeAutoPassedEvent) EventType() string { return EventTypeNodeAutoPassed }
