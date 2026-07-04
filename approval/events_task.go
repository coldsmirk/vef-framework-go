package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// TaskCreatedEvent fires the moment a task row is inserted, not the moment
// it becomes actionable. Under sequential approval, tasks after the first
// start with Status=Waiting and a nil Deadline; subscribers should treat a
// nil Deadline as the cue that the task is queued behind a predecessor and
// must not yet surface a "new pending task" notification. When the
// predecessor finishes and the task transitions to Pending, no new event
// is published — subscribers reading the live task table see the change.
// If this contract proves insufficient, the right extension is to add a
// dedicated TaskActivatedEvent rather than overloading TaskCreatedEvent.
type TaskCreatedEvent struct {
	TaskEventBase

	Assignee UserInfo        `json:"assignee"`
	Deadline *timex.DateTime `json:"deadline,omitempty"`
}

func NewTaskCreatedEvent(instance *Instance, task *Task, node *FlowNode) *TaskCreatedEvent {
	return &TaskCreatedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Assignee:      task.Assignee(),
		Deadline:      task.Deadline,
	}
}

func (*TaskCreatedEvent) EventType() string { return EventTypeTaskCreated }

// TaskApprovedEvent fired when a task is approved.
type TaskApprovedEvent struct {
	TaskEventBase

	Operator UserInfo `json:"operator"`
	Opinion  *string  `json:"opinion,omitempty"`
}

func NewTaskApprovedEvent(instance *Instance, task *Task, node *FlowNode, operator UserInfo, opinion string) *TaskApprovedEvent {
	return &TaskApprovedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Operator:      operator,
		Opinion:       stringPtrOrNil(opinion),
	}
}

func (*TaskApprovedEvent) EventType() string { return EventTypeTaskApproved }

// TaskHandledEvent fired when a handle-type task is completed.
type TaskHandledEvent struct {
	TaskEventBase

	Operator UserInfo `json:"operator"`
	Opinion  *string  `json:"opinion,omitempty"`
}

func NewTaskHandledEvent(instance *Instance, task *Task, node *FlowNode, operator UserInfo, opinion string) *TaskHandledEvent {
	return &TaskHandledEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Operator:      operator,
		Opinion:       stringPtrOrNil(opinion),
	}
}

func (*TaskHandledEvent) EventType() string { return EventTypeTaskHandled }

// TaskRejectedEvent fired when a task is rejected.
type TaskRejectedEvent struct {
	TaskEventBase

	Operator UserInfo `json:"operator"`
	Opinion  *string  `json:"opinion,omitempty"`
}

func NewTaskRejectedEvent(instance *Instance, task *Task, node *FlowNode, operator UserInfo, opinion string) *TaskRejectedEvent {
	return &TaskRejectedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Operator:      operator,
		Opinion:       stringPtrOrNil(opinion),
	}
}

func (*TaskRejectedEvent) EventType() string { return EventTypeTaskRejected }

// TaskCanceledEvent fired when the engine cancels a task that no longer needs
// a decision — its node completed through other votes, or the whole instance
// was withdrawn, rolled back, or terminated. Subscribers use it to retract
// pending to-do entries for the canceled assignee. Reason carries the
// triggering operation.
type TaskCanceledEvent struct {
	TaskEventBase

	Assignee UserInfo `json:"assignee"`
	Reason   string   `json:"reason"`
}

func NewTaskCanceledEvent(instance *Instance, task *Task, node *FlowNode, reason string) *TaskCanceledEvent {
	return &TaskCanceledEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Assignee:      task.Assignee(),
		Reason:        reason,
	}
}

func (*TaskCanceledEvent) EventType() string { return EventTypeTaskCanceled }

// TaskTransferredEvent fired when a task is transferred by its assignee.
type TaskTransferredEvent struct {
	TaskEventBase

	From   UserInfo `json:"from"`
	To     UserInfo `json:"to"`
	Reason *string  `json:"reason,omitempty"`
}

func NewTaskTransferredEvent(instance *Instance, task *Task, node *FlowNode, from, to UserInfo, reason string) *TaskTransferredEvent {
	return &TaskTransferredEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		From:          from,
		To:            to,
		Reason:        stringPtrOrNil(reason),
	}
}

func (*TaskTransferredEvent) EventType() string { return EventTypeTaskTransferred }

// TaskReassignedEvent fired when an admin reassigns a task to a different user.
type TaskReassignedEvent struct {
	TaskEventBase

	From   UserInfo `json:"from"`
	To     UserInfo `json:"to"`
	Reason *string  `json:"reason,omitempty"`
}

func NewTaskReassignedEvent(instance *Instance, task *Task, node *FlowNode, from, to UserInfo, reason string) *TaskReassignedEvent {
	return &TaskReassignedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		From:          from,
		To:            to,
		Reason:        stringPtrOrNil(reason),
	}
}

func (*TaskReassignedEvent) EventType() string { return EventTypeTaskReassigned }

// TaskTimedOutEvent fired when a task times out.
type TaskTimedOutEvent struct {
	TaskEventBase

	Assignee UserInfo       `json:"assignee"`
	Deadline timex.DateTime `json:"deadline"`
}

func NewTaskTimedOutEvent(instance *Instance, task *Task, node *FlowNode) *TaskTimedOutEvent {
	var deadline timex.DateTime
	if task.Deadline != nil {
		deadline = *task.Deadline
	}

	return &TaskTimedOutEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Assignee:      task.Assignee(),
		Deadline:      deadline,
	}
}

func (*TaskTimedOutEvent) EventType() string { return EventTypeTaskTimedOut }

// AssigneesAddedEvent fired when assignees are dynamically added. TaskID is
// the task whose assignee initiated the addition.
type AssigneesAddedEvent struct {
	TaskEventBase

	AddType   AddAssigneeType `json:"addType"`
	Assignees []UserInfo      `json:"assignees"`
}

func NewAssigneesAddedEvent(instance *Instance, task *Task, node *FlowNode, addType AddAssigneeType, assignees []UserInfo) *AssigneesAddedEvent {
	return &AssigneesAddedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		AddType:       addType,
		Assignees:     assignees,
	}
}

func (*AssigneesAddedEvent) EventType() string { return EventTypeAssigneesAdded }

// AssigneesRemovedEvent fired when assignees are dynamically removed. TaskID
// is the removed assignee's task.
type AssigneesRemovedEvent struct {
	TaskEventBase

	Assignees []UserInfo `json:"assignees"`
}

func NewAssigneesRemovedEvent(instance *Instance, task *Task, node *FlowNode, assignees []UserInfo) *AssigneesRemovedEvent {
	return &AssigneesRemovedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Assignees:     assignees,
	}
}

func (*AssigneesRemovedEvent) EventType() string { return EventTypeAssigneesRemoved }
