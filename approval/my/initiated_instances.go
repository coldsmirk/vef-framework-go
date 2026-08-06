package my

import "github.com/coldsmirk/vef-framework-go/timex"

// InitiatedInstance represents an approval instance submitted by the current user.
type InitiatedInstance struct {
	InstanceID string  `json:"instanceId"`
	InstanceNo string  `json:"instanceNo"`
	Title      string  `json:"title"`
	FlowName   string  `json:"flowName"`
	FlowIcon   *string `json:"flowIcon,omitempty"`
	// Labels are the flow's host-owned selection metadata, read from the
	// mutable flow row rather than the instance's version snapshot — so a
	// relabelled flow reads consistently across every list that carries them.
	Labels          map[string]string `json:"labels,omitempty"`
	Status          string            `json:"status"`
	CurrentNodeName *string           `json:"currentNodeName,omitempty"`
	CreatedAt       timex.DateTime    `json:"createdAt"`
	FinishedAt      *timex.DateTime   `json:"finishedAt,omitempty"`
}
