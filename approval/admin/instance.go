package admin

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Instance represents an approval instance in the admin view.
type Instance struct {
	InstanceID      string            `json:"instanceId"`
	InstanceNo      string            `json:"instanceNo"`
	Title           string            `json:"title"`
	TenantID        string            `json:"tenantId"`
	FlowID          string            `json:"flowId"`
	FlowName        string            `json:"flowName"`
	Applicant       approval.UserInfo `json:"applicant"`
	Status          string            `json:"status"`
	CurrentNodeName *string           `json:"currentNodeName,omitempty"`
	CreatedAt       timex.DateTime    `json:"createdAt"`
	FinishedAt      *timex.DateTime   `json:"finishedAt,omitempty"`
}
