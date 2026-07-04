package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// InstanceCreatedEvent fired when a new instance is created.
type InstanceCreatedEvent struct {
	InstanceEventBase
}

func NewInstanceCreatedEvent(instance *Instance) *InstanceCreatedEvent {
	return &InstanceCreatedEvent{InstanceEventBase: NewInstanceEventBase(instance)}
}

func (*InstanceCreatedEvent) EventType() string { return EventTypeInstanceCreated }

// InstanceCompletedEvent fired when instance reaches a final status.
type InstanceCompletedEvent struct {
	InstanceEventBase

	// FinalStatus is always one of the IsFinal() statuses — approved,
	// rejected, or terminated. Paused states (returned / withdrawn) never
	// fire this event.
	FinalStatus InstanceStatus `json:"finalStatus"`
	FinishedAt  timex.DateTime `json:"finishedAt"`
}

// NewInstanceCompletedEvent builds the completion event. FinishedAt is taken
// from the instance (every completion path stamps it before publishing) and
// falls back to now for defensive completeness.
func NewInstanceCompletedEvent(instance *Instance, finalStatus InstanceStatus) *InstanceCompletedEvent {
	finishedAt := timex.Now()
	if instance.FinishedAt != nil {
		finishedAt = *instance.FinishedAt
	}

	return &InstanceCompletedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		FinalStatus:       finalStatus,
		FinishedAt:        finishedAt,
	}
}

func (*InstanceCompletedEvent) EventType() string { return EventTypeInstanceCompleted }

// InstanceWithdrawnEvent fired when applicant withdraws the instance.
type InstanceWithdrawnEvent struct {
	InstanceEventBase

	Operator UserInfo `json:"operator"`
}

func NewInstanceWithdrawnEvent(instance *Instance, operator UserInfo) *InstanceWithdrawnEvent {
	return &InstanceWithdrawnEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		Operator:          operator,
	}
}

func (*InstanceWithdrawnEvent) EventType() string { return EventTypeInstanceWithdrawn }

// InstanceRolledBackEvent fired when a task decision sends the flow back to
// an intermediate node (the instance keeps running). Rolling back to the
// start node fires InstanceReturnedEvent instead.
type InstanceRolledBackEvent struct {
	InstanceEventBase

	FromNodeID   string   `json:"fromNodeId"`
	FromNodeName string   `json:"fromNodeName"`
	ToNodeID     string   `json:"toNodeId"`
	ToNodeName   string   `json:"toNodeName"`
	Operator     UserInfo `json:"operator"`
}

func NewInstanceRolledBackEvent(instance *Instance, fromNode, toNode *FlowNode, operator UserInfo) *InstanceRolledBackEvent {
	return &InstanceRolledBackEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		FromNodeID:        fromNode.ID,
		FromNodeName:      fromNode.Name,
		ToNodeID:          toNode.ID,
		ToNodeName:        toNode.Name,
		Operator:          operator,
	}
}

func (*InstanceRolledBackEvent) EventType() string { return EventTypeInstanceRolledBack }

// InstanceReturnedEvent fired when the flow is sent back to the initiator:
// the instance pauses as returned until the applicant resubmits or abandons.
type InstanceReturnedEvent struct {
	InstanceEventBase

	FromNodeID   string   `json:"fromNodeId"`
	FromNodeName string   `json:"fromNodeName"`
	ToNodeID     string   `json:"toNodeId"`
	ToNodeName   string   `json:"toNodeName"`
	Operator     UserInfo `json:"operator"`
}

func NewInstanceReturnedEvent(instance *Instance, fromNode, toNode *FlowNode, operator UserInfo) *InstanceReturnedEvent {
	return &InstanceReturnedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		FromNodeID:        fromNode.ID,
		FromNodeName:      fromNode.Name,
		ToNodeID:          toNode.ID,
		ToNodeName:        toNode.Name,
		Operator:          operator,
	}
}

func (*InstanceReturnedEvent) EventType() string { return EventTypeInstanceReturned }

// InstanceResubmittedEvent fired when the initiator resubmits a returned instance.
type InstanceResubmittedEvent struct {
	InstanceEventBase

	Operator UserInfo `json:"operator"`
}

func NewInstanceResubmittedEvent(instance *Instance, operator UserInfo) *InstanceResubmittedEvent {
	return &InstanceResubmittedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		Operator:          operator,
	}
}

func (*InstanceResubmittedEvent) EventType() string { return EventTypeInstanceResubmitted }

// InstanceBindingFailedEvent fires when business binding (writing the final
// status back to the host's business table) fails after the approval itself
// has already committed. Subscribers retry asynchronously; the approval is
// not rolled back. Operators can grep these events for stuck bindings.
type InstanceBindingFailedEvent struct {
	InstanceEventBase

	// FinalStatus is the terminal status the write-back attempted to persist,
	// always one of the IsFinal() statuses (approved / rejected / terminated).
	FinalStatus   InstanceStatus `json:"finalStatus"`
	BusinessTable string         `json:"businessTable"`
	ErrorMessage  string         `json:"errorMessage"`
}

func NewInstanceBindingFailedEvent(instance *Instance, finalStatus InstanceStatus, businessTable, errorMessage string) *InstanceBindingFailedEvent {
	return &InstanceBindingFailedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		FinalStatus:       finalStatus,
		BusinessTable:     businessTable,
		ErrorMessage:      errorMessage,
	}
}

func (*InstanceBindingFailedEvent) EventType() string { return EventTypeInstanceBindingFailed }
