package approval

// FlowCreatedEvent fires when a new flow definition is created.
type FlowCreatedEvent struct {
	FlowEventBase

	CategoryID string `json:"categoryId"`
}

func NewFlowCreatedEvent(flow *Flow) *FlowCreatedEvent {
	return &FlowCreatedEvent{
		FlowEventBase: NewFlowEventBase(flow),
		CategoryID:    flow.CategoryID,
	}
}

func (*FlowCreatedEvent) EventType() string { return EventTypeFlowCreated }

// FlowUpdatedEvent fires when a flow definition's metadata is updated.
type FlowUpdatedEvent struct {
	FlowEventBase
}

func NewFlowUpdatedEvent(flow *Flow) *FlowUpdatedEvent {
	return &FlowUpdatedEvent{FlowEventBase: NewFlowEventBase(flow)}
}

func (*FlowUpdatedEvent) EventType() string { return EventTypeFlowUpdated }

// FlowDeployedEvent fires when a flow version is deployed.
type FlowDeployedEvent struct {
	FlowEventBase

	VersionID string `json:"versionId"`
	Version   int    `json:"version"`
}

func NewFlowDeployedEvent(flow *Flow, versionID string, version int) *FlowDeployedEvent {
	return &FlowDeployedEvent{
		FlowEventBase: NewFlowEventBase(flow),
		VersionID:     versionID,
		Version:       version,
	}
}

func (*FlowDeployedEvent) EventType() string { return EventTypeFlowDeployed }

// FlowToggledEvent fires when a flow is activated or deactivated.
type FlowToggledEvent struct {
	FlowEventBase

	IsActive bool `json:"isActive"`
}

func NewFlowToggledEvent(flow *Flow, isActive bool) *FlowToggledEvent {
	return &FlowToggledEvent{
		FlowEventBase: NewFlowEventBase(flow),
		IsActive:      isActive,
	}
}

func (*FlowToggledEvent) EventType() string { return EventTypeFlowToggled }

// FlowPublishedEvent fires when a flow version is published.
type FlowPublishedEvent struct {
	FlowEventBase

	VersionID string `json:"versionId"`
}

func NewFlowPublishedEvent(flow *Flow, versionID string) *FlowPublishedEvent {
	return &FlowPublishedEvent{
		FlowEventBase: NewFlowEventBase(flow),
		VersionID:     versionID,
	}
}

func (*FlowPublishedEvent) EventType() string { return EventTypeFlowPublished }
