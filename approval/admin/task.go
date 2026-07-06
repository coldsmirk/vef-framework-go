package admin

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Task represents an approval task in the admin view.
type Task struct {
	TaskID        string            `json:"taskId"`
	InstanceID    string            `json:"instanceId"`
	InstanceTitle string            `json:"instanceTitle"`
	FlowName      string            `json:"flowName"`
	NodeName      string            `json:"nodeName"`
	Assignee      approval.UserInfo `json:"assignee"`
	Status        string            `json:"status"`
	CreatedAt     timex.DateTime    `json:"createdAt"`
	Deadline      *timex.DateTime   `json:"deadline,omitempty"`
	FinishedAt    *timex.DateTime   `json:"finishedAt,omitempty"`
}
