package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// TaskDeadlineWarningEvent fired when a task is approaching its deadline.
type TaskDeadlineWarningEvent struct {
	TaskID       string         `json:"taskId"`
	TenantID     string         `json:"tenantId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	AssigneeID   string         `json:"assigneeId"`
	AssigneeName string         `json:"assigneeName"`
	Deadline     timex.DateTime `json:"deadline"`
	HoursLeft    int            `json:"hoursLeft"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskDeadlineWarningEvent(taskID, tenantID, instanceID, nodeID, assigneeID, assigneeName string, deadline timex.DateTime, hoursLeft int) *TaskDeadlineWarningEvent {
	return &TaskDeadlineWarningEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		Deadline:     deadline,
		HoursLeft:    hoursLeft,
		OccurredTime: timex.Now(),
	}
}

func (*TaskDeadlineWarningEvent) EventType() string { return EventTypeTaskDeadlineWarning }

// TaskUrgedEvent fired when a task assignee is urged/reminded.
type TaskUrgedEvent struct {
	InstanceID     string         `json:"instanceId"`
	TenantID       string         `json:"tenantId"`
	NodeID         string         `json:"nodeId"`
	TaskID         string         `json:"taskId"`
	UrgerID        string         `json:"urgerId"`
	UrgerName      string         `json:"urgerName"`
	TargetUserID   string         `json:"targetUserId"`
	TargetUserName string         `json:"targetUserName"`
	Message        *string        `json:"message,omitempty"`
	OccurredTime   timex.DateTime `json:"occurredTime"`
}

//nolint:revive // 9 positional args are clearer than a wrapper struct here; callers map flat task fields directly.
func NewTaskUrgedEvent(instanceID, tenantID, nodeID, taskID, urgerID, urgerName, targetUserID, targetUserName, message string) *TaskUrgedEvent {
	return &TaskUrgedEvent{
		InstanceID:     instanceID,
		TenantID:       tenantID,
		NodeID:         nodeID,
		TaskID:         taskID,
		UrgerID:        urgerID,
		UrgerName:      urgerName,
		TargetUserID:   targetUserID,
		TargetUserName: targetUserName,
		Message:        stringPtrOrNil(message),
		OccurredTime:   timex.Now(),
	}
}

func (*TaskUrgedEvent) EventType() string { return EventTypeTaskUrged }
