package approval

// AnnotationPublishedEvent groups the drafts made visible by one decision or resubmission.
type AnnotationPublishedEvent struct {
	InstanceEventBase
	TaskID        *string  `json:"taskId,omitempty"`
	NodeID        string   `json:"nodeId"`
	Author        UserInfo `json:"author"`
	AnnotationIDs []string `json:"annotationIds"`
}

func (*AnnotationPublishedEvent) EventType() string { return EventTypeAnnotationPublished }
