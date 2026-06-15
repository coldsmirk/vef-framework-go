package my

import "github.com/coldsmirk/vef-framework-go/timex"

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
