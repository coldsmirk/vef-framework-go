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
	TaskID       string          `json:"taskId"`
	TenantID     string          `json:"tenantId"`
	InstanceID   string          `json:"instanceId"`
	NodeID       string          `json:"nodeId"`
	AssigneeID   string          `json:"assigneeId"`
	AssigneeName string          `json:"assigneeName"`
	Deadline     *timex.DateTime `json:"deadline,omitempty"`
	OccurredTime timex.DateTime  `json:"occurredTime"`
}

func NewTaskCreatedEvent(taskID, tenantID, instanceID, nodeID, assigneeID, assigneeName string, deadline *timex.DateTime) *TaskCreatedEvent {
	return &TaskCreatedEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		Deadline:     deadline,
		OccurredTime: timex.Now(),
	}
}

func (*TaskCreatedEvent) EventType() string { return EventTypeTaskCreated }

// TaskApprovedEvent fired when a task is approved.
type TaskApprovedEvent struct {
	TaskID       string         `json:"taskId"`
	TenantID     string         `json:"tenantId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskApprovedEvent(taskID, tenantID, instanceID, nodeID, operatorID, opinion string) *TaskApprovedEvent {
	return &TaskApprovedEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      stringPtrOrNil(opinion),
		OccurredTime: timex.Now(),
	}
}

func (*TaskApprovedEvent) EventType() string { return EventTypeTaskApproved }

// TaskHandledEvent fired when a handle-type task is completed.
type TaskHandledEvent struct {
	TaskID       string         `json:"taskId"`
	TenantID     string         `json:"tenantId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskHandledEvent(taskID, tenantID, instanceID, nodeID, operatorID, opinion string) *TaskHandledEvent {
	return &TaskHandledEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      stringPtrOrNil(opinion),
		OccurredTime: timex.Now(),
	}
}

func (*TaskHandledEvent) EventType() string { return EventTypeTaskHandled }

// TaskRejectedEvent fired when a task is rejected.
type TaskRejectedEvent struct {
	TaskID       string         `json:"taskId"`
	TenantID     string         `json:"tenantId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskRejectedEvent(taskID, tenantID, instanceID, nodeID, operatorID, opinion string) *TaskRejectedEvent {
	return &TaskRejectedEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      stringPtrOrNil(opinion),
		OccurredTime: timex.Now(),
	}
}

func (*TaskRejectedEvent) EventType() string { return EventTypeTaskRejected }

// TaskTransferredEvent fired when a task is transferred.
type TaskTransferredEvent struct {
	TaskID       string         `json:"taskId"`
	TenantID     string         `json:"tenantId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	FromUserID   string         `json:"fromUserId"`
	FromUserName string         `json:"fromUserName"`
	ToUserID     string         `json:"toUserId"`
	ToUserName   string         `json:"toUserName"`
	Reason       *string        `json:"reason,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

//nolint:revive // 9 positional args are clearer than a wrapper struct here; callers map flat task fields directly.
func NewTaskTransferredEvent(taskID, tenantID, instanceID, nodeID, fromUserID, fromUserName, toUserID, toUserName, reason string) *TaskTransferredEvent {
	return &TaskTransferredEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		FromUserID:   fromUserID,
		FromUserName: fromUserName,
		ToUserID:     toUserID,
		ToUserName:   toUserName,
		Reason:       stringPtrOrNil(reason),
		OccurredTime: timex.Now(),
	}
}

func (*TaskTransferredEvent) EventType() string { return EventTypeTaskTransferred }

// TaskReassignedEvent fired when an admin reassigns a task to a different user.
type TaskReassignedEvent struct {
	TaskID       string         `json:"taskId"`
	TenantID     string         `json:"tenantId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	FromUserID   string         `json:"fromUserId"`
	FromUserName string         `json:"fromUserName"`
	ToUserID     string         `json:"toUserId"`
	ToUserName   string         `json:"toUserName"`
	Reason       *string        `json:"reason,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

//nolint:revive // 9 positional args are clearer than a wrapper struct here; callers map flat task fields directly.
func NewTaskReassignedEvent(taskID, tenantID, instanceID, nodeID, fromUserID, fromUserName, toUserID, toUserName, reason string) *TaskReassignedEvent {
	return &TaskReassignedEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		FromUserID:   fromUserID,
		FromUserName: fromUserName,
		ToUserID:     toUserID,
		ToUserName:   toUserName,
		Reason:       stringPtrOrNil(reason),
		OccurredTime: timex.Now(),
	}
}

func (*TaskReassignedEvent) EventType() string { return EventTypeTaskReassigned }

// TaskTimedOutEvent fired when a task times out.
type TaskTimedOutEvent struct {
	TaskID       string         `json:"taskId"`
	TenantID     string         `json:"tenantId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	AssigneeID   string         `json:"assigneeId"`
	AssigneeName string         `json:"assigneeName"`
	Deadline     timex.DateTime `json:"deadline"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskTimedOutEvent(taskID, tenantID, instanceID, nodeID, assigneeID, assigneeName string, deadline timex.DateTime) *TaskTimedOutEvent {
	return &TaskTimedOutEvent{
		TaskID:       taskID,
		TenantID:     tenantID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		Deadline:     deadline,
		OccurredTime: timex.Now(),
	}
}

func (*TaskTimedOutEvent) EventType() string { return EventTypeTaskTimedOut }

// AssigneesAddedEvent fired when assignees are dynamically added.
type AssigneesAddedEvent struct {
	InstanceID    string            `json:"instanceId"`
	TenantID      string            `json:"tenantId"`
	NodeID        string            `json:"nodeId"`
	TaskID        string            `json:"taskId"`
	AddType       AddAssigneeType   `json:"addType"`
	AssigneeIDs   []string          `json:"assigneeIds"`
	AssigneeNames map[string]string `json:"assigneeNames"`
	OccurredTime  timex.DateTime    `json:"occurredTime"`
}

func NewAssigneesAddedEvent(instanceID, tenantID, nodeID, taskID string, addType AddAssigneeType, assigneeIDs []string, assigneeNames map[string]string) *AssigneesAddedEvent {
	return &AssigneesAddedEvent{
		InstanceID:    instanceID,
		TenantID:      tenantID,
		NodeID:        nodeID,
		TaskID:        taskID,
		AddType:       addType,
		AssigneeIDs:   assigneeIDs,
		AssigneeNames: assigneeNames,
		OccurredTime:  timex.Now(),
	}
}

func (*AssigneesAddedEvent) EventType() string { return EventTypeAssigneesAdded }

// AssigneesRemovedEvent fired when assignees are dynamically removed.
type AssigneesRemovedEvent struct {
	InstanceID    string            `json:"instanceId"`
	TenantID      string            `json:"tenantId"`
	NodeID        string            `json:"nodeId"`
	TaskID        string            `json:"taskId"`
	AssigneeIDs   []string          `json:"assigneeIds"`
	AssigneeNames map[string]string `json:"assigneeNames"`
	OccurredTime  timex.DateTime    `json:"occurredTime"`
}

func NewAssigneesRemovedEvent(instanceID, tenantID, nodeID, taskID string, assigneeIDs []string, assigneeNames map[string]string) *AssigneesRemovedEvent {
	return &AssigneesRemovedEvent{
		InstanceID:    instanceID,
		TenantID:      tenantID,
		NodeID:        nodeID,
		TaskID:        taskID,
		AssigneeIDs:   assigneeIDs,
		AssigneeNames: assigneeNames,
		OccurredTime:  timex.Now(),
	}
}

func (*AssigneesRemovedEvent) EventType() string { return EventTypeAssigneesRemoved }
