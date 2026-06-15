package my

import "github.com/coldsmirk/vef-framework-go/timex"

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
