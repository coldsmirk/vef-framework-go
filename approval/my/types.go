package my

import "github.com/coldsmirk/vef-framework-go/timex"

// AvailableFlow describes a flow the current user is allowed to initiate.
type AvailableFlow struct {
	FlowID       string  `json:"flowId"`
	FlowCode     string  `json:"flowCode"`
	FlowName     string  `json:"flowName"`
	FlowIcon     *string `json:"flowIcon,omitempty"`
	Description  *string `json:"description,omitempty"`
	CategoryID   string  `json:"categoryId"`
	CategoryName string  `json:"categoryName"`
}

// InitiatedInstance represents an approval instance submitted by the current user.
type InitiatedInstance struct {
	InstanceID      string          `json:"instanceId"`
	InstanceNo      string          `json:"instanceNo"`
	Title           string          `json:"title"`
	FlowName        string          `json:"flowName"`
	FlowIcon        *string         `json:"flowIcon,omitempty"`
	Status          string          `json:"status"`
	CurrentNodeName *string         `json:"currentNodeName,omitempty"`
	CreatedAt       timex.DateTime  `json:"createdAt"`
	FinishedAt      *timex.DateTime `json:"finishedAt,omitempty"`
}

// PendingTask represents a task that is awaiting the current user's action.
type PendingTask struct {
	TaskID        string          `json:"taskId"`
	InstanceID    string          `json:"instanceId"`
	InstanceTitle string          `json:"instanceTitle"`
	InstanceNo    string          `json:"instanceNo"`
	FlowName      string          `json:"flowName"`
	FlowIcon      *string         `json:"flowIcon,omitempty"`
	ApplicantName string          `json:"applicantName"`
	NodeName      string          `json:"nodeName"`
	CreatedAt     timex.DateTime  `json:"createdAt"`
	Deadline      *timex.DateTime `json:"deadline,omitempty"`
	IsTimeout     bool            `json:"isTimeout"`
}

// CompletedTask represents a task the current user has already processed.
type CompletedTask struct {
	TaskID        string          `json:"taskId"`
	InstanceID    string          `json:"instanceId"`
	InstanceTitle string          `json:"instanceTitle"`
	InstanceNo    string          `json:"instanceNo"`
	FlowName      string          `json:"flowName"`
	FlowIcon      *string         `json:"flowIcon,omitempty"`
	ApplicantName string          `json:"applicantName"`
	NodeName      string          `json:"nodeName"`
	Status        string          `json:"status"`
	FinishedAt    *timex.DateTime `json:"finishedAt,omitempty"`
}

// CCRecord represents a CC notification addressed to the current user.
type CCRecord struct {
	CCRecordID    string         `json:"ccRecordId"`
	InstanceID    string         `json:"instanceId"`
	InstanceTitle string         `json:"instanceTitle"`
	InstanceNo    string         `json:"instanceNo"`
	FlowName      string         `json:"flowName"`
	FlowIcon      *string        `json:"flowIcon,omitempty"`
	ApplicantName string         `json:"applicantName"`
	NodeName      *string        `json:"nodeName,omitempty"`
	IsRead        bool           `json:"isRead"`
	CreatedAt     timex.DateTime `json:"createdAt"`
}

// PendingCounts holds the counts of pending actions for the current user.
type PendingCounts struct {
	PendingTaskCount int `json:"pendingTaskCount"`
	UnreadCCCount    int `json:"unreadCcCount"`
}

// InstanceDetail is the self-service detail view for an approval instance.
type InstanceDetail struct {
	Instance         InstanceInfo    `json:"instance"`
	Tasks            []TaskInfo      `json:"tasks"`
	ActionLogs       []ActionLogInfo `json:"actionLogs"`
	FlowNodes        []FlowNodeInfo  `json:"flowNodes"`
	AvailableActions []string        `json:"availableActions"`
}

// InstanceInfo holds the core instance information within a detail view.
type InstanceInfo struct {
	InstanceID       string          `json:"instanceId"`
	InstanceNo       string          `json:"instanceNo"`
	Title            string          `json:"title"`
	FlowName         string          `json:"flowName"`
	FlowIcon         *string         `json:"flowIcon,omitempty"`
	ApplicantID      string          `json:"applicantId"`
	ApplicantName    string          `json:"applicantName"`
	Status           string          `json:"status"`
	CurrentNodeName  *string         `json:"currentNodeName,omitempty"`
	BusinessRecordID *string         `json:"businessRecordId,omitempty"`
	FormData         map[string]any  `json:"formData,omitempty"`
	CreatedAt        timex.DateTime  `json:"createdAt"`
	FinishedAt       *timex.DateTime `json:"finishedAt,omitempty"`
}

// TaskInfo holds task information within a detail view.
type TaskInfo struct {
	TaskID       string          `json:"taskId"`
	NodeName     string          `json:"nodeName"`
	AssigneeID   string          `json:"assigneeId"`
	AssigneeName string          `json:"assigneeName"`
	Status       string          `json:"status"`
	SortOrder    int             `json:"sortOrder"`
	CreatedAt    timex.DateTime  `json:"createdAt"`
	FinishedAt   *timex.DateTime `json:"finishedAt,omitempty"`
}

// ActionLogInfo holds an action log entry within a detail view.
type ActionLogInfo struct {
	Action       string         `json:"action"`
	OperatorName string         `json:"operatorName"`
	Opinion      *string        `json:"opinion,omitempty"`
	CreatedAt    timex.DateTime `json:"createdAt"`
}

// FlowNodeInfo holds a flow node entry within a detail view.
type FlowNodeInfo struct {
	NodeID string `json:"nodeId"`
	Key    string `json:"key"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
}
