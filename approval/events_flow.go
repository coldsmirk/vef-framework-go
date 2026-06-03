package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// FlowCreatedEvent fires when a new flow definition is created.
type FlowCreatedEvent struct {
	FlowID       string         `json:"flowId"`
	TenantID     string         `json:"tenantId"`
	Code         string         `json:"code"`
	Name         string         `json:"name"`
	CategoryID   string         `json:"categoryId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewFlowCreatedEvent(flowID, tenantID, code, name, categoryID string) *FlowCreatedEvent {
	return &FlowCreatedEvent{
		FlowID:       flowID,
		TenantID:     tenantID,
		Code:         code,
		Name:         name,
		CategoryID:   categoryID,
		OccurredTime: timex.Now(),
	}
}

func (*FlowCreatedEvent) EventType() string { return EventTypeFlowCreated }

// FlowUpdatedEvent fires when a flow's metadata (name, description, admins,
// initiators, etc.) is updated. Version publication has its own event.
type FlowUpdatedEvent struct {
	FlowID       string         `json:"flowId"`
	TenantID     string         `json:"tenantId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewFlowUpdatedEvent(flowID, tenantID string) *FlowUpdatedEvent {
	return &FlowUpdatedEvent{
		FlowID:       flowID,
		TenantID:     tenantID,
		OccurredTime: timex.Now(),
	}
}

func (*FlowUpdatedEvent) EventType() string { return EventTypeFlowUpdated }

// FlowDeployedEvent fires when a flow's schema is deployed as a new draft
// version (before it's published).
type FlowDeployedEvent struct {
	FlowID       string         `json:"flowId"`
	TenantID     string         `json:"tenantId"`
	VersionID    string         `json:"versionId"`
	Version      int            `json:"version"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewFlowDeployedEvent(flowID, tenantID, versionID string, version int) *FlowDeployedEvent {
	return &FlowDeployedEvent{
		FlowID:       flowID,
		TenantID:     tenantID,
		VersionID:    versionID,
		Version:      version,
		OccurredTime: timex.Now(),
	}
}

func (*FlowDeployedEvent) EventType() string { return EventTypeFlowDeployed }

// FlowToggledEvent fires when a flow is activated or deactivated.
type FlowToggledEvent struct {
	FlowID       string         `json:"flowId"`
	TenantID     string         `json:"tenantId"`
	IsActive     bool           `json:"isActive"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewFlowToggledEvent(flowID, tenantID string, isActive bool) *FlowToggledEvent {
	return &FlowToggledEvent{
		FlowID:       flowID,
		TenantID:     tenantID,
		IsActive:     isActive,
		OccurredTime: timex.Now(),
	}
}

func (*FlowToggledEvent) EventType() string { return EventTypeFlowToggled }

// FlowPublishedEvent fired when a flow version is published.
type FlowPublishedEvent struct {
	FlowID       string         `json:"flowId"`
	TenantID     string         `json:"tenantId"`
	VersionID    string         `json:"versionId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewFlowPublishedEvent(flowID, tenantID, versionID string) *FlowPublishedEvent {
	return &FlowPublishedEvent{
		FlowID:       flowID,
		TenantID:     tenantID,
		VersionID:    versionID,
		OccurredTime: timex.Now(),
	}
}

func (*FlowPublishedEvent) EventType() string { return EventTypeFlowPublished }
