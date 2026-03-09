package admin

import "github.com/coldsmirk/vef-framework-go/timex"

// Instance represents an approval instance in the admin view.
type Instance struct {
	InstanceID      string          `json:"instanceId"`
	InstanceNo      string          `json:"instanceNo"`
	Title           string          `json:"title"`
	TenantID        string          `json:"tenantId"`
	FlowID          string          `json:"flowId"`
	FlowName        string          `json:"flowName"`
	ApplicantID     string          `json:"applicantId"`
	ApplicantName   string          `json:"applicantName"`
	Status          string          `json:"status"`
	CurrentNodeName *string         `json:"currentNodeName,omitempty"`
	CreatedAt       timex.DateTime  `json:"createdAt"`
	FinishedAt      *timex.DateTime `json:"finishedAt,omitempty"`
}

// Task represents an approval task in the admin view.
type Task struct {
	TaskID        string          `json:"taskId"`
	InstanceID    string          `json:"instanceId"`
	InstanceTitle string          `json:"instanceTitle"`
	FlowName      string          `json:"flowName"`
	NodeName      string          `json:"nodeName"`
	AssigneeID    string          `json:"assigneeId"`
	AssigneeName  string          `json:"assigneeName"`
	Status        string          `json:"status"`
	CreatedAt     timex.DateTime  `json:"createdAt"`
	Deadline      *timex.DateTime `json:"deadline,omitempty"`
	FinishedAt    *timex.DateTime `json:"finishedAt,omitempty"`
}

// InstanceDetail represents the full admin detail view of an approval instance.
type InstanceDetail struct {
	Instance   InstanceDetailInfo `json:"instance"`
	Tasks      []TaskDetailInfo   `json:"tasks"`
	ActionLogs []ActionLog        `json:"actionLogs"`
	FlowNodes  []FlowNodeInfo     `json:"flowNodes"`
}

// InstanceDetailInfo carries the instance portion of an admin detail view.
type InstanceDetailInfo struct {
	InstanceID       string          `json:"instanceId"`
	InstanceNo       string          `json:"instanceNo"`
	Title            string          `json:"title"`
	TenantID         string          `json:"tenantId"`
	FlowID           string          `json:"flowId"`
	FlowName         string          `json:"flowName"`
	FlowVersionID    string          `json:"flowVersionId"`
	ApplicantID      string          `json:"applicantId"`
	ApplicantName    string          `json:"applicantName"`
	Status           string          `json:"status"`
	CurrentNodeName  *string         `json:"currentNodeName,omitempty"`
	BusinessRecordID *string         `json:"businessRecordId,omitempty"`
	FormData         map[string]any  `json:"formData,omitempty"`
	CreatedAt        timex.DateTime  `json:"createdAt"`
	FinishedAt       *timex.DateTime `json:"finishedAt,omitempty"`
}

// TaskDetailInfo represents a task entry within the admin instance detail view.
type TaskDetailInfo struct {
	TaskID        string          `json:"taskId"`
	NodeID        string          `json:"nodeId"`
	NodeName      string          `json:"nodeName"`
	AssigneeID    string          `json:"assigneeId"`
	AssigneeName  string          `json:"assigneeName"`
	DelegatorID   *string         `json:"delegatorId,omitempty"`
	DelegatorName *string         `json:"delegatorName,omitempty"`
	Status        string          `json:"status"`
	SortOrder     int             `json:"sortOrder"`
	Deadline      *timex.DateTime `json:"deadline,omitempty"`
	IsTimeout     bool            `json:"isTimeout"`
	CreatedAt     timex.DateTime  `json:"createdAt"`
	FinishedAt    *timex.DateTime `json:"finishedAt,omitempty"`
}

// ActionLog represents an action log entry in the admin view.
type ActionLog struct {
	LogID            string         `json:"logId"`
	Action           string         `json:"action"`
	OperatorID       string         `json:"operatorId"`
	OperatorName     string         `json:"operatorName"`
	OperatorDeptName *string        `json:"operatorDeptName,omitempty"`
	TransferToID     *string        `json:"transferToId,omitempty"`
	TransferToName   *string        `json:"transferToName,omitempty"`
	Opinion          *string        `json:"opinion,omitempty"`
	CreatedAt        timex.DateTime `json:"createdAt"`
}

// FlowNodeInfo represents a flow node in the admin instance detail view.
type FlowNodeInfo struct {
	NodeID        string `json:"nodeId"`
	Key           string `json:"key"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	ExecutionType string `json:"executionType"`
}
