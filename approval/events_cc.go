package approval

// CCNotifiedEvent fired when CC recipients are notified — automatically when
// a node's CC timing triggers, or manually when a participant adds CCs.
type CCNotifiedEvent struct {
	InstanceEventBase

	NodeID     string     `json:"nodeId"`
	NodeName   string     `json:"nodeName"`
	Recipients []UserInfo `json:"recipients"`
	IsManual   bool       `json:"isManual"`
}

func NewCCNotifiedEvent(instance *Instance, node *FlowNode, recipients []UserInfo, isManual bool) *CCNotifiedEvent {
	return &CCNotifiedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		NodeID:            node.ID,
		NodeName:          node.Name,
		Recipients:        recipients,
		IsManual:          isManual,
	}
}

func (*CCNotifiedEvent) EventType() string { return EventTypeCCNotified }
