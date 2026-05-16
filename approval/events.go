package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// stringPtrOrNil returns nil for empty strings, or a pointer to the string value.
func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// DomainEvent is the contract every approval domain event satisfies.
// EventType matches the framework's event.Event surface so domain
// events can be published through the event Bus without adaptation.
// Business time is carried as OccurredTime in the concrete payload and
// projected onto Envelope.OccurredAt via event.WithOccurredAt at publish time.
//
// Tenant scope: every instance/task/node/cc-level event carries TenantID so
// subscribers can route on tenancy without re-querying. Flow-level events
// (created/updated/etc.) already include TenantID on the payload.
type DomainEvent interface {
	// EventType returns the unique event identifier (e.g., "approval.instance.created").
	EventType() string
}

// ==================== Instance Events ====================

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

func (*InstanceCreatedEvent) EventType() string { return "approval.instance.created" }

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

func (*InstanceCompletedEvent) EventType() string { return "approval.instance.completed" }

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

func (*InstanceWithdrawnEvent) EventType() string { return "approval.instance.withdrawn" }

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

func (*InstanceRolledBackEvent) EventType() string { return "approval.instance.rolled_back" }

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

func (*InstanceReturnedEvent) EventType() string { return "approval.instance.returned" }

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

func (*InstanceResubmittedEvent) EventType() string { return "approval.instance.resubmitted" }

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

func (*InstanceBindingFailedEvent) EventType() string { return "approval.instance.binding_failed" }

// ==================== Node Events ====================

// NodeEnteredEvent fired when instance enters a new node.
type NodeEnteredEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	NodeID       string         `json:"nodeId"`
	NodeName     string         `json:"nodeName"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeEnteredEvent(instanceID, tenantID, nodeID, nodeName string) *NodeEnteredEvent {
	return &NodeEnteredEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		NodeID:       nodeID,
		NodeName:     nodeName,
		OccurredTime: timex.Now(),
	}
}

func (*NodeEnteredEvent) EventType() string { return "approval.node.entered" }

// NodeAutoPassedEvent fired when a node is auto-passed.
type NodeAutoPassedEvent struct {
	InstanceID   string         `json:"instanceId"`
	TenantID     string         `json:"tenantId"`
	NodeID       string         `json:"nodeId"`
	Reason       string         `json:"reason"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeAutoPassedEvent(instanceID, tenantID, nodeID, reason string) *NodeAutoPassedEvent {
	return &NodeAutoPassedEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		NodeID:       nodeID,
		Reason:       reason,
		OccurredTime: timex.Now(),
	}
}

func (*NodeAutoPassedEvent) EventType() string { return "approval.node.auto_passed" }

// ==================== Task Events ====================

// TaskCreatedEvent fired when a new task is created.
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

func (*TaskCreatedEvent) EventType() string { return "approval.task.created" }

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

func (*TaskApprovedEvent) EventType() string { return "approval.task.approved" }

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

func (*TaskHandledEvent) EventType() string { return "approval.task.handled" }

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

func (*TaskRejectedEvent) EventType() string { return "approval.task.rejected" }

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

func (*TaskTransferredEvent) EventType() string { return "approval.task.transferred" }

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

func (*TaskReassignedEvent) EventType() string { return "approval.task.reassigned" }

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

func (*TaskTimedOutEvent) EventType() string { return "approval.task.timed_out" }

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

func (*AssigneesAddedEvent) EventType() string { return "approval.task.assignees_added" }

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

func (*AssigneesRemovedEvent) EventType() string { return "approval.task.assignees_removed" }

// ==================== CC Events ====================

// CCNotifiedEvent fired when users are carbon-copied.
type CCNotifiedEvent struct {
	InstanceID   string            `json:"instanceId"`
	TenantID     string            `json:"tenantId"`
	NodeID       string            `json:"nodeId"`
	CCUserIDs    []string          `json:"ccUserIds"`
	CCUserNames  map[string]string `json:"ccUserNames"`
	IsManual     bool              `json:"isManual"`
	OccurredTime timex.DateTime    `json:"occurredTime"`
}

func NewCCNotifiedEvent(instanceID, tenantID, nodeID string, ccUserIDs []string, ccUserNames map[string]string, isManual bool) *CCNotifiedEvent {
	return &CCNotifiedEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		NodeID:       nodeID,
		CCUserIDs:    ccUserIDs,
		CCUserNames:  ccUserNames,
		IsManual:     isManual,
		OccurredTime: timex.Now(),
	}
}

func (*CCNotifiedEvent) EventType() string { return "approval.cc.notified" }

// ==================== Flow Events ====================

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

func (*FlowCreatedEvent) EventType() string { return "approval.flow.created" }

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

func (*FlowUpdatedEvent) EventType() string { return "approval.flow.updated" }

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

func (*FlowDeployedEvent) EventType() string { return "approval.flow.deployed" }

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

func (*FlowToggledEvent) EventType() string { return "approval.flow.toggled" }

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

func (*FlowPublishedEvent) EventType() string { return "approval.flow.published" }

// ==================== Timeout & Urge Events ====================

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

func (*TaskDeadlineWarningEvent) EventType() string { return "approval.task.deadline_warning" }

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

func (*TaskUrgedEvent) EventType() string { return "approval.task.urged" }
