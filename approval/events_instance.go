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

	// Reason is the administrator's stated reason when FinalStatus is
	// terminated; nil for approved / rejected completions, whose deciding
	// opinions live on the task events.
	Reason *string `json:"reason,omitempty"`
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

	// Reason is the applicant's stated withdrawal reason, when provided.
	Reason *string `json:"reason,omitempty"`
}

func NewInstanceWithdrawnEvent(instance *Instance, operator UserInfo, reason *string) *InstanceWithdrawnEvent {
	return &InstanceWithdrawnEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		Operator:          operator,
		Reason:            reason,
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

	// Opinion is the rollback opinion provided by the operator, if any.
	Opinion *string `json:"opinion,omitempty"`
}

func NewInstanceRolledBackEvent(instance *Instance, fromNode, toNode *FlowNode, operator UserInfo, opinion *string) *InstanceRolledBackEvent {
	return &InstanceRolledBackEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		FromNodeID:        fromNode.ID,
		FromNodeName:      fromNode.Name,
		ToNodeID:          toNode.ID,
		ToNodeName:        toNode.Name,
		Operator:          operator,
		Opinion:           opinion,
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

	// Opinion is the rollback opinion provided by the operator, if any.
	Opinion *string `json:"opinion,omitempty"`
}

func NewInstanceReturnedEvent(instance *Instance, fromNode, toNode *FlowNode, operator UserInfo, opinion *string) *InstanceReturnedEvent {
	return &InstanceReturnedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		FromNodeID:        fromNode.ID,
		FromNodeName:      fromNode.Name,
		ToNodeID:          toNode.ID,
		ToNodeName:        toNode.Name,
		Operator:          operator,
		Opinion:           opinion,
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

// InstanceBindingFailedEvent fires when the engine-owned business write-back
// fails after the driving approval action has already committed. Subscribers
// retry asynchronously; the approval is not rolled back. Operators can grep
// these events for stuck bindings. The started trigger never appears here —
// its write-back runs inside the start transaction and a failure rolls back
// the initiation instead of firing this event.
type InstanceBindingFailedEvent struct {
	InstanceEventBase

	// Trigger is the lifecycle moment whose write-back failed (completed /
	// returned / withdrawn / resubmitted).
	Trigger BindingTrigger `json:"trigger"`
	// Status is the instance status the write-back attempted to persist into
	// the business status column at that moment.
	Status        InstanceStatus `json:"status"`
	BusinessTable string         `json:"businessTable"`
	ErrorMessage  string         `json:"errorMessage"`
}

func NewInstanceBindingFailedEvent(instance *Instance, trigger BindingTrigger, status InstanceStatus, businessTable, errorMessage string) *InstanceBindingFailedEvent {
	return &InstanceBindingFailedEvent{
		InstanceEventBase: NewInstanceEventBase(instance),
		Trigger:           trigger,
		Status:            status,
		BusinessTable:     businessTable,
		ErrorMessage:      errorMessage,
	}
}

func (*InstanceBindingFailedEvent) EventType() string { return EventTypeInstanceBindingFailed }
