package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// TaskCreatedEvent fires the moment a task row is inserted, not the moment it
// becomes actionable. Under sequential approval every assignee's row is
// inserted up front, so this event says "the task exists", not "someone should
// act on it" — a subscriber that notifies on it would page approvers whose turn
// has not come. Use it for mirroring the task table, audit, and metrics.
//
// To notify the person who should act now, subscribe to TaskActivatedEvent.
type TaskCreatedEvent struct {
	TaskEventBase

	Assignee UserInfo        `json:"assignee"`
	Deadline *timex.DateTime `json:"deadline,omitempty"`
	Status   TaskStatus      `json:"status"`
}

func NewTaskCreatedEvent(instance *Instance, task *Task, node *FlowNode) *TaskCreatedEvent {
	return &TaskCreatedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Assignee:      task.Assignee(),
		Deadline:      task.Deadline,
		Status:        task.Status,
	}
}

func (*TaskCreatedEvent) EventType() string { return EventTypeTaskCreated }

// TaskActivatedEvent fires when a task becomes actionable by a specific person:
// it is the single answer to "whose turn is it now", and the event a to-do
// notification should subscribe to.
//
// It covers every way a task starts waiting on someone — created already
// Pending (a parallel node's assignees, a sequential node's first approver),
// promoted from Waiting when its predecessor finishes, handed to a new person
// by transfer or timeout auto-transfer, or reassigned to a different assignee.
// Subscribing to it alone is sufficient; there is no need to enumerate the
// actions that can produce a pending task.
//
// Deliberately not derivable from TaskCreatedEvent: a task's Deadline is nil
// whenever its node configures no timeout, so a nil Deadline cannot tell a
// queued task from an immediately actionable one.
type TaskActivatedEvent struct {
	TaskEventBase

	Assignee UserInfo        `json:"assignee"`
	Deadline *timex.DateTime `json:"deadline,omitempty"`
	// Reason names what made the task actionable, so a subscriber can vary the
	// wording (a first assignment reads differently from a transfer).
	Reason TaskActivationReason `json:"reason"`
}

// TaskActivationReason classifies why a task became actionable.
type TaskActivationReason string

const (
	// TaskActivationAssigned is a task that was actionable the moment it was
	// created — a parallel node's assignees, or a sequential node's first.
	TaskActivationAssigned TaskActivationReason = "assigned"
	// TaskActivationQueueAdvanced is a task promoted from Waiting because the
	// task it was queued behind finished.
	TaskActivationQueueAdvanced TaskActivationReason = "queue_advanced"
	// TaskActivationTransferred is a task handed to a new assignee by a manual
	// transfer or by the timeout scanner's auto-transfer.
	TaskActivationTransferred TaskActivationReason = "transferred"
	// TaskActivationReassigned is an existing pending task pointed at a
	// different assignee by an administrator.
	TaskActivationReassigned TaskActivationReason = "reassigned"
)

func NewTaskActivatedEvent(instance *Instance, task *Task, node *FlowNode, reason TaskActivationReason) *TaskActivatedEvent {
	return &TaskActivatedEvent{
		TaskEventBase: NewTaskEventBase(instance, task, node),
		Assignee:      task.Assignee(),
		Deadline:      task.Deadline,
		Reason:        reason,
	}
}

func (*TaskActivatedEvent) EventType() string { return EventTypeTaskActivated }

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
