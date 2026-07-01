package my

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// InstanceDetail is the self-service detail view for an approval instance.
type InstanceDetail struct {
	Instance         InstanceInfo               `json:"instance"`
	Tasks            []TaskInfo                 `json:"tasks"`
	ActionLogs       []ActionLogInfo            `json:"actionLogs"`
	FlowGraph        approval.InstanceFlowGraph `json:"flowGraph"`
	AvailableActions []string                   `json:"availableActions"`
}

// InstanceInfo holds the core instance information within a detail view.
type InstanceInfo struct {
	InstanceID              string                   `json:"instanceId"`
	InstanceNo              string                   `json:"instanceNo"`
	Title                   string                   `json:"title"`
	FlowName                string                   `json:"flowName"`
	FlowIcon                *string                  `json:"flowIcon,omitempty"`
	ApplicantID             string                   `json:"applicantId"`
	ApplicantName           string                   `json:"applicantName"`
	ApplicantDepartmentName *string                  `json:"applicantDepartmentName,omitempty"`
	Status                  string                   `json:"status"`
	CurrentNodeID           *string                  `json:"currentNodeId,omitempty"`
	CurrentNodeName         *string                  `json:"currentNodeName,omitempty"`
	BusinessRecordID        *string                  `json:"businessRecordId,omitempty"`
	FormData                map[string]any           `json:"formData,omitempty"`
	FormSchema              *approval.FormDefinition `json:"formSchema,omitempty"`
	CreatedAt               timex.DateTime           `json:"createdAt"`
	FinishedAt              *timex.DateTime          `json:"finishedAt,omitempty"`
}

// TaskInfo holds task information within a detail view.
type TaskInfo struct {
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

// ActionLogInfo holds an action log entry within a detail view.
type ActionLogInfo struct {
	LogID                  string               `json:"logId"`
	Action                 string               `json:"action"`
	NodeID                 *string              `json:"nodeId,omitempty"`
	OperatorID             string               `json:"operatorId"`
	OperatorName           string               `json:"operatorName"`
	OperatorDepartmentName *string              `json:"operatorDepartmentName,omitempty"`
	TransferToID           *string              `json:"transferToId,omitempty"`
	TransferToName         *string              `json:"transferToName,omitempty"`
	RollbackToNodeID       *string              `json:"rollbackToNodeId,omitempty"`
	AddedAssignees         []approval.UserBrief `json:"addedAssignees,omitempty"`
	RemovedAssignees       []approval.UserBrief `json:"removedAssignees,omitempty"`
	CCUsers                []approval.UserBrief `json:"ccUsers,omitempty"`
	Opinion                *string              `json:"opinion,omitempty"`
	Attachments            []string             `json:"attachments,omitempty"`
	CreatedAt              timex.DateTime       `json:"createdAt"`
}
