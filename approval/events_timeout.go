package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// TaskDeadlineWarningEvent fired ahead of a task's deadline so subscribers
// can nudge the assignee before the timeout action kicks in.
type TaskDeadlineWarningEvent struct {
	TaskEventBase

	Assignee  UserInfo       `json:"assignee"`
	Deadline  timex.DateTime `json:"deadline"`
	HoursLeft int            `json:"hoursLeft"`
}

func NewTaskDeadlineWarningEvent(instance *Instance, task *Task, node *FlowNode, hoursLeft int) *TaskDeadlineWarningEvent {
	var deadline timex.DateTime
	if task.Deadline != nil {
		deadline = *task.Deadline
	}

	return &TaskDeadlineWarningEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Assignee:      task.Assignee(),
		Deadline:      deadline,
		HoursLeft:     hoursLeft,
	}
}

func (*TaskDeadlineWarningEvent) EventType() string { return EventTypeTaskDeadlineWarning }

// TaskUrgedEvent fired when a participant urges the pending assignee.
type TaskUrgedEvent struct {
	TaskEventBase

	Urger   UserInfo `json:"urger"`
	Target  UserInfo `json:"target"`
	Message *string  `json:"message,omitempty"`
}

func NewTaskUrgedEvent(instance *Instance, task *Task, node *FlowNode, urger UserInfo, message string) *TaskUrgedEvent {
	return &TaskUrgedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Urger:         urger,
		Target:        task.Assignee(),
		Message:       stringPtrOrNil(message),
	}
}

func (*TaskUrgedEvent) EventType() string { return EventTypeTaskUrged }
