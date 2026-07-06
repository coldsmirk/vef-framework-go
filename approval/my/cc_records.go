package my

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// CCRecord represents a CC notification addressed to the current user.
type CCRecord struct {
	CCRecordID    string            `json:"ccRecordId"`
	InstanceID    string            `json:"instanceId"`
	InstanceTitle string            `json:"instanceTitle"`
	InstanceNo    string            `json:"instanceNo"`
	FlowName      string            `json:"flowName"`
	FlowIcon      *string           `json:"flowIcon,omitempty"`
	Applicant     approval.UserInfo `json:"applicant"`
	NodeName      *string           `json:"nodeName,omitempty"`
	IsRead        bool              `json:"isRead"`
	CreatedAt     timex.DateTime    `json:"createdAt"`
}
