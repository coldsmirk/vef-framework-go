package my

import "github.com/coldsmirk/vef-framework-go/timex"

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
