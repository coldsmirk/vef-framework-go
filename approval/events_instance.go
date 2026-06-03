package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// InstanceCreatedEvent fired when a new instance is created.
type InstanceCreatedEvent struct {
	InstanceID    string         `json:"instanceId"`
	TenantID      string         `json:"tenantId"`
	FlowID        string         `json:"flowId"`
	Title         string         `json:"title"`
	ApplicantID   string         `json:"applicantId"`
	ApplicantName string         `json:"applicantName"`
	OccurredTime  timex.DateTime `json:"occurredTime"`
}

func NewInstanceCreatedEvent(instanceID, tenantID, flowID, title, applicantID, applicantName string) *InstanceCreatedEvent {
	return &InstanceCreatedEvent{
		InstanceID:    instanceID,
		TenantID:      tenantID,
		FlowID:        flowID,
		Title:         title,
		ApplicantID:   applicantID,
		ApplicantName: applicantName,
		OccurredTime:  timex.Now(),
	}
}

func (*InstanceCreatedEvent) EventType() string { return EventTypeInstanceCreated }

// InstanceCompletedEvent fired when instance reaches a final status.
type InstanceCompletedEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	FinalStatus  InstanceStatus `json:"finalStatus"`
	FinishedAt   timex.DateTime `json:"finishedAt"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceCompletedEvent(instanceID, tenantID string, finalStatus InstanceStatus) *InstanceCompletedEvent {
	now := timex.Now()

	return &InstanceCompletedEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		FinalStatus:  finalStatus,
		FinishedAt:   now,
		OccurredTime: now,
	}
}

func (*InstanceCompletedEvent) EventType() string { return EventTypeInstanceCompleted }

// InstanceWithdrawnEvent fired when applicant withdraws the instance.
type InstanceWithdrawnEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceWithdrawnEvent(instanceID, tenantID, operatorID string) *InstanceWithdrawnEvent {
	return &InstanceWithdrawnEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceWithdrawnEvent) EventType() string { return EventTypeInstanceWithdrawn }

// InstanceRolledBackEvent fired when instance is rolled back.
type InstanceRolledBackEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	FromNodeID   string         `json:"fromNodeId"`
	ToNodeID     string         `json:"toNodeId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceRolledBackEvent(instanceID, tenantID, fromNodeID, toNodeID, operatorID string) *InstanceRolledBackEvent {
	return &InstanceRolledBackEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		FromNodeID:   fromNodeID,
		ToNodeID:     toNodeID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceRolledBackEvent) EventType() string { return EventTypeInstanceRolledBack }

// InstanceReturnedEvent fired when instance is returned to the initiator.
type InstanceReturnedEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	FromNodeID   string         `json:"fromNodeId"`
	ToNodeID     string         `json:"toNodeId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceReturnedEvent(instanceID, tenantID, fromNodeID, toNodeID, operatorID string) *InstanceReturnedEvent {
	return &InstanceReturnedEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		FromNodeID:   fromNodeID,
		ToNodeID:     toNodeID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceReturnedEvent) EventType() string { return EventTypeInstanceReturned }

// InstanceResubmittedEvent fired when the initiator resubmits a returned instance.
type InstanceResubmittedEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceResubmittedEvent(instanceID, tenantID, operatorID string) *InstanceResubmittedEvent {
	return &InstanceResubmittedEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceResubmittedEvent) EventType() string { return EventTypeInstanceResubmitted }

// InstanceBindingFailedEvent fires when business binding (writing the final
// status back to the host's business table) fails after the approval itself
// has already committed. Subscribers retry asynchronously; the approval is
// not rolled back. Operators can grep these events for stuck bindings.
type InstanceBindingFailedEvent struct {
	InstanceID    string         `json:"instanceId"`
	TenantID      string         `json:"tenantId"`
	FlowID        string         `json:"flowId"`
	FinalStatus   InstanceStatus `json:"finalStatus"`
	BusinessTable string         `json:"businessTable"`
	ErrorMessage  string         `json:"errorMessage"`
	OccurredTime  timex.DateTime `json:"occurredTime"`
}

func NewInstanceBindingFailedEvent(instanceID, tenantID, flowID string, finalStatus InstanceStatus, businessTable, errorMessage string) *InstanceBindingFailedEvent {
	return &InstanceBindingFailedEvent{
		InstanceID:    instanceID,
		TenantID:      tenantID,
		FlowID:        flowID,
		FinalStatus:   finalStatus,
		BusinessTable: businessTable,
		ErrorMessage:  errorMessage,
		OccurredTime:  timex.Now(),
	}
}

func (*InstanceBindingFailedEvent) EventType() string { return EventTypeInstanceBindingFailed }
