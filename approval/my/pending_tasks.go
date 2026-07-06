package my

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// PendingTask represents a task that is awaiting the current user's action.
type PendingTask struct {
	TaskID        string            `json:"taskId"`
	InstanceID    string            `json:"instanceId"`
	InstanceTitle string            `json:"instanceTitle"`
	InstanceNo    string            `json:"instanceNo"`
	FlowName      string            `json:"flowName"`
	FlowIcon      *string           `json:"flowIcon,omitempty"`
	Applicant     approval.UserInfo `json:"applicant"`
	NodeName      string            `json:"nodeName"`
	CreatedAt     timex.DateTime    `json:"createdAt"`
	Deadline      *timex.DateTime   `json:"deadline,omitempty"`
	IsTimeout     bool              `json:"isTimeout"`
}
