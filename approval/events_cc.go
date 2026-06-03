package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// CCNotifiedEvent fired when users are carbon-copied.
type CCNotifiedEvent struct {
	InstanceID   string            `json:"instanceId"`
	TenantID     string            `json:"tenantId"`
	NodeID       string            `json:"nodeId"`
	CCUserIDs    []string          `json:"ccUserIds"`
	CCUserNames  map[string]string `json:"ccUserNames"`
	IsManual     bool              `json:"isManual"`
	OccurredTime timex.DateTime    `json:"occurredTime"`
}

func NewCCNotifiedEvent(instanceID, tenantID, nodeID string, ccUserIDs []string, ccUserNames map[string]string, isManual bool) *CCNotifiedEvent {
	return &CCNotifiedEvent{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		NodeID:       nodeID,
		CCUserIDs:    ccUserIDs,
		CCUserNames:  ccUserNames,
		IsManual:     isManual,
		OccurredTime: timex.Now(),
	}
}

func (*CCNotifiedEvent) EventType() string { return EventTypeCCNotified }
