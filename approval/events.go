package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// DomainEvent is the base interface for all approval domain events.
type DomainEvent interface {
	// EventName returns the unique event identifier (e.g., "approval.instance.created").
	EventName() string
	// OccurredAt returns the timestamp when the event occurred.
	OccurredAt() timex.DateTime
}

// EventDispatcher dispatches outbox events to external systems.
// Default implementation forwards to event.Bus.
type EventDispatcher interface {
	// Dispatch sends an outbox event record to the external event system.
	Dispatch(ctx context.Context, record EventOutbox) error
}

// ==================== Instance Events ====================

// InstanceCreatedEvent fired when a new instance is created.
type InstanceCreatedEvent struct {
	InstanceID    string         `json:"instanceId"`
	FlowID        string         `json:"flowId"`
	Title         string         `json:"title"`
	ApplicantID   string         `json:"applicantId"`
	ApplicantName string         `json:"applicantName"`
	OccurredTime  timex.DateTime `json:"occurredTime"`
}

func NewInstanceCreatedEvent(instanceID, flowID, title, applicantID, applicantName string) *InstanceCreatedEvent {
	return &InstanceCreatedEvent{
		InstanceID:    instanceID,
		FlowID:        flowID,
		Title:         title,
		ApplicantID:   applicantID,
		ApplicantName: applicantName,
		OccurredTime:  timex.Now(),
	}
}

func (*InstanceCreatedEvent) EventName() string            { return "approval.instance.created" }
func (e *InstanceCreatedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceCompletedEvent fired when instance reaches a final status.
type InstanceCompletedEvent struct {
	InstanceID   string         `json:"instanceId"`
	FinalStatus  InstanceStatus `json:"finalStatus"`
	FinishedAt   timex.DateTime `json:"finishedAt"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceCompletedEvent(instanceID string, finalStatus InstanceStatus) *InstanceCompletedEvent {
	now := timex.Now()

	return &InstanceCompletedEvent{
		InstanceID:   instanceID,
		FinalStatus:  finalStatus,
		FinishedAt:   now,
		OccurredTime: now,
	}
}

func (*InstanceCompletedEvent) EventName() string            { return "approval.instance.completed" }
func (e *InstanceCompletedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceWithdrawnEvent fired when applicant withdraws the instance.
type InstanceWithdrawnEvent struct {
	InstanceID   string         `json:"instanceId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceWithdrawnEvent(instanceID, operatorID string) *InstanceWithdrawnEvent {
	return &InstanceWithdrawnEvent{
		InstanceID:   instanceID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceWithdrawnEvent) EventName() string            { return "approval.instance.withdrawn" }
func (e *InstanceWithdrawnEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceRolledBackEvent fired when instance is rolled back.
type InstanceRolledBackEvent struct {
	InstanceID   string         `json:"instanceId"`
	FromNodeID   string         `json:"fromNodeId"`
	ToNodeID     string         `json:"toNodeId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceRolledBackEvent(instanceID, fromNodeID, toNodeID, operatorID string) *InstanceRolledBackEvent {
	return &InstanceRolledBackEvent{
		InstanceID:   instanceID,
		FromNodeID:   fromNodeID,
		ToNodeID:     toNodeID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceRolledBackEvent) EventName() string            { return "approval.instance.rolled_back" }
func (e *InstanceRolledBackEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceReturnedEvent fired when instance is returned to the initiator.
type InstanceReturnedEvent struct {
	InstanceID   string         `json:"instanceId"`
	FromNodeID   string         `json:"fromNodeId"`
	ToNodeID     string         `json:"toNodeId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceReturnedEvent(instanceID, fromNodeID, toNodeID, operatorID string) *InstanceReturnedEvent {
	return &InstanceReturnedEvent{
		InstanceID:   instanceID,
		FromNodeID:   fromNodeID,
		ToNodeID:     toNodeID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceReturnedEvent) EventName() string            { return "approval.instance.returned" }
func (e *InstanceReturnedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceResubmittedEvent fired when the initiator resubmits a returned instance.
type InstanceResubmittedEvent struct {
	InstanceID   string         `json:"instanceId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceResubmittedEvent(instanceID, operatorID string) *InstanceResubmittedEvent {
	return &InstanceResubmittedEvent{
		InstanceID:   instanceID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (*InstanceResubmittedEvent) EventName() string            { return "approval.instance.resubmitted" }
func (e *InstanceResubmittedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Node Events ====================

// NodeEnteredEvent fired when instance enters a new node.
type NodeEnteredEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	NodeName     string         `json:"nodeName"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeEnteredEvent(instanceID, nodeID, nodeName string) *NodeEnteredEvent {
	return &NodeEnteredEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		NodeName:     nodeName,
		OccurredTime: timex.Now(),
	}
}

func (*NodeEnteredEvent) EventName() string            { return "approval.node.entered" }
func (e *NodeEnteredEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// NodeAutoPassedEvent fired when a node is auto-passed.
type NodeAutoPassedEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	Reason       string         `json:"reason"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeAutoPassedEvent(instanceID, nodeID, reason string) *NodeAutoPassedEvent {
	return &NodeAutoPassedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		Reason:       reason,
		OccurredTime: timex.Now(),
	}
}

func (*NodeAutoPassedEvent) EventName() string            { return "approval.node.auto_passed" }
func (e *NodeAutoPassedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ParallelJoinedEvent fired when parallel branches are joined.
type ParallelJoinedEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewParallelJoinedEvent(instanceID, nodeID string) *ParallelJoinedEvent {
	return &ParallelJoinedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OccurredTime: timex.Now(),
	}
}

func (*ParallelJoinedEvent) EventName() string            { return "approval.node.parallel_joined" }
func (e *ParallelJoinedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Task Events ====================

// TaskCreatedEvent fired when a new task is created.
type TaskCreatedEvent struct {
	TaskID       string          `json:"taskId"`
	InstanceID   string          `json:"instanceId"`
	NodeID       string          `json:"nodeId"`
	AssigneeID   string          `json:"assigneeId"`
	AssigneeName string          `json:"assigneeName"`
	Deadline     *timex.DateTime `json:"deadline,omitempty"`
	OccurredTime timex.DateTime  `json:"occurredTime"`
}

func NewTaskCreatedEvent(taskID, instanceID, nodeID, assigneeID, assigneeName string, deadline *timex.DateTime) *TaskCreatedEvent {
	return &TaskCreatedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		Deadline:     deadline,
		OccurredTime: timex.Now(),
	}
}

func (*TaskCreatedEvent) EventName() string            { return "approval.task.created" }
func (e *TaskCreatedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskApprovedEvent fired when a task is approved.
type TaskApprovedEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskApprovedEvent(taskID, instanceID, nodeID, operatorID, opinion string) *TaskApprovedEvent {
	return &TaskApprovedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      new(opinion),
		OccurredTime: timex.Now(),
	}
}

func (*TaskApprovedEvent) EventName() string            { return "approval.task.approved" }
func (e *TaskApprovedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskHandledEvent fired when a handle-type task is completed.
type TaskHandledEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskHandledEvent(taskID, instanceID, nodeID, operatorID, opinion string) *TaskHandledEvent {
	return &TaskHandledEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      new(opinion),
		OccurredTime: timex.Now(),
	}
}

func (*TaskHandledEvent) EventName() string            { return "approval.task.handled" }
func (e *TaskHandledEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskRejectedEvent fired when a task is rejected.
type TaskRejectedEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskRejectedEvent(taskID, instanceID, nodeID, operatorID, opinion string) *TaskRejectedEvent {
	return &TaskRejectedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      new(opinion),
		OccurredTime: timex.Now(),
	}
}

func (*TaskRejectedEvent) EventName() string            { return "approval.task.rejected" }
func (e *TaskRejectedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskTransferredEvent fired when a task is transferred.
type TaskTransferredEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	FromUserID   string         `json:"fromUserId"`
	FromUserName string         `json:"fromUserName"`
	ToUserID     string         `json:"toUserId"`
	ToUserName   string         `json:"toUserName"`
	Reason       *string        `json:"reason,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskTransferredEvent(taskID, instanceID, nodeID, fromUserID, fromUserName, toUserID, toUserName, reason string) *TaskTransferredEvent {
	return &TaskTransferredEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		FromUserID:   fromUserID,
		FromUserName: fromUserName,
		ToUserID:     toUserID,
		ToUserName:   toUserName,
		Reason:       new(reason),
		OccurredTime: timex.Now(),
	}
}

func (*TaskTransferredEvent) EventName() string            { return "approval.task.transferred" }
func (e *TaskTransferredEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskReassignedEvent fired when an admin reassigns a task to a different user.
type TaskReassignedEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	FromUserID   string         `json:"fromUserId"`
	FromUserName string         `json:"fromUserName"`
	ToUserID     string         `json:"toUserId"`
	ToUserName   string         `json:"toUserName"`
	Reason       *string        `json:"reason,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskReassignedEvent(taskID, instanceID, nodeID, fromUserID, fromUserName, toUserID, toUserName, reason string) *TaskReassignedEvent {
	return &TaskReassignedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		FromUserID:   fromUserID,
		FromUserName: fromUserName,
		ToUserID:     toUserID,
		ToUserName:   toUserName,
		Reason:       new(reason),
		OccurredTime: timex.Now(),
	}
}

func (*TaskReassignedEvent) EventName() string            { return "approval.task.reassigned" }
func (e *TaskReassignedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskTimeoutEvent fired when a task times out.
type TaskTimeoutEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	AssigneeID   string         `json:"assigneeId"`
	AssigneeName string         `json:"assigneeName"`
	Deadline     timex.DateTime `json:"deadline"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskTimeoutEvent(taskID, instanceID, nodeID, assigneeID, assigneeName string, deadline timex.DateTime) *TaskTimeoutEvent {
	return &TaskTimeoutEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		Deadline:     deadline,
		OccurredTime: timex.Now(),
	}
}

func (*TaskTimeoutEvent) EventName() string            { return "approval.task.timeout" }
func (e *TaskTimeoutEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// AssigneesAddedEvent fired when assignees are dynamically added.
type AssigneesAddedEvent struct {
	InstanceID    string            `json:"instanceId"`
	NodeID        string            `json:"nodeId"`
	TaskID        string            `json:"taskId"`
	AddType       AddAssigneeType   `json:"addType"`
	AssigneeIDs   []string          `json:"assigneeIds"`
	AssigneeNames map[string]string `json:"assigneeNames"`
	OccurredTime  timex.DateTime    `json:"occurredTime"`
}

func NewAssigneesAddedEvent(instanceID, nodeID, taskID string, addType AddAssigneeType, assigneeIDs []string, assigneeNames map[string]string) *AssigneesAddedEvent {
	return &AssigneesAddedEvent{
		InstanceID:    instanceID,
		NodeID:        nodeID,
		TaskID:        taskID,
		AddType:       addType,
		AssigneeIDs:   assigneeIDs,
		AssigneeNames: assigneeNames,
		OccurredTime:  timex.Now(),
	}
}

func (*AssigneesAddedEvent) EventName() string            { return "approval.task.assignees_added" }
func (e *AssigneesAddedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// AssigneesRemovedEvent fired when assignees are dynamically removed.
type AssigneesRemovedEvent struct {
	InstanceID    string            `json:"instanceId"`
	NodeID        string            `json:"nodeId"`
	TaskID        string            `json:"taskId"`
	AssigneeIDs   []string          `json:"assigneeIds"`
	AssigneeNames map[string]string `json:"assigneeNames"`
	OccurredTime  timex.DateTime    `json:"occurredTime"`
}

func NewAssigneesRemovedEvent(instanceID, nodeID, taskID string, assigneeIDs []string, assigneeNames map[string]string) *AssigneesRemovedEvent {
	return &AssigneesRemovedEvent{
		InstanceID:    instanceID,
		NodeID:        nodeID,
		TaskID:        taskID,
		AssigneeIDs:   assigneeIDs,
		AssigneeNames: assigneeNames,
		OccurredTime:  timex.Now(),
	}
}

func (*AssigneesRemovedEvent) EventName() string            { return "approval.task.assignees_removed" }
func (e *AssigneesRemovedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== CC Events ====================

// CCNotifiedEvent fired when users are carbon-copied.
type CCNotifiedEvent struct {
	InstanceID   string            `json:"instanceId"`
	NodeID       string            `json:"nodeId"`
	CCUserIDs    []string          `json:"ccUserIds"`
	CCUserNames  map[string]string `json:"ccUserNames"`
	IsManual     bool              `json:"isManual"`
	OccurredTime timex.DateTime    `json:"occurredTime"`
}

func NewCCNotifiedEvent(instanceID, nodeID string, ccUserIDs []string, ccUserNames map[string]string, isManual bool) *CCNotifiedEvent {
	return &CCNotifiedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		CCUserIDs:    ccUserIDs,
		CCUserNames:  ccUserNames,
		IsManual:     isManual,
		OccurredTime: timex.Now(),
	}
}

func (*CCNotifiedEvent) EventName() string            { return "approval.cc.notified" }
func (e *CCNotifiedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Flow Events ====================

// FlowPublishedEvent fired when a flow version is published.
type FlowPublishedEvent struct {
	FlowID       string         `json:"flowId"`
	VersionID    string         `json:"versionId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewFlowPublishedEvent(flowID, versionID string) *FlowPublishedEvent {
	return &FlowPublishedEvent{
		FlowID:       flowID,
		VersionID:    versionID,
		OccurredTime: timex.Now(),
	}
}

func (*FlowPublishedEvent) EventName() string            { return "approval.flow.published" }
func (e *FlowPublishedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Timeout & Urge Events ====================

// TaskDeadlineWarningEvent fired when a task is approaching its deadline.
type TaskDeadlineWarningEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	AssigneeID   string         `json:"assigneeId"`
	AssigneeName string         `json:"assigneeName"`
	Deadline     timex.DateTime `json:"deadline"`
	HoursLeft    int            `json:"hoursLeft"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskDeadlineWarningEvent(taskID, instanceID, nodeID, assigneeID, assigneeName string, deadline timex.DateTime, hoursLeft int) *TaskDeadlineWarningEvent {
	return &TaskDeadlineWarningEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		Deadline:     deadline,
		HoursLeft:    hoursLeft,
		OccurredTime: timex.Now(),
	}
}

func (*TaskDeadlineWarningEvent) EventName() string            { return "approval.task.deadline_warning" }
func (e *TaskDeadlineWarningEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskUrgedEvent fired when a task assignee is urged/reminded.
type TaskUrgedEvent struct {
	InstanceID     string         `json:"instanceId"`
	NodeID         string         `json:"nodeId"`
	TaskID         string         `json:"taskId"`
	UrgerID        string         `json:"urgerId"`
	UrgerName      string         `json:"urgerName"`
	TargetUserID   string         `json:"targetUserId"`
	TargetUserName string         `json:"targetUserName"`
	Message        *string        `json:"message,omitempty"`
	OccurredTime   timex.DateTime `json:"occurredTime"`
}

func NewTaskUrgedEvent(instanceID, nodeID, taskID, urgerID, urgerName, targetUserID, targetUserName, message string) *TaskUrgedEvent {
	return &TaskUrgedEvent{
		InstanceID:     instanceID,
		NodeID:         nodeID,
		TaskID:         taskID,
		UrgerID:        urgerID,
		UrgerName:      urgerName,
		TargetUserID:   targetUserID,
		TargetUserName: targetUserName,
		Message:        new(message),
		OccurredTime:   timex.Now(),
	}
}

func (*TaskUrgedEvent) EventName() string            { return "approval.task.urged" }
func (e *TaskUrgedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }
