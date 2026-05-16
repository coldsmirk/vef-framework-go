package admin

import "github.com/coldsmirk/vef-framework-go/timex"

// Metrics aggregates approval engine health and throughput indicators. It is
// shaped for admin dashboards and ops alerting, not for fine-grained APM.
// All counts are tenant-scoped when TenantID is set on the query; super-
// admin callers can request a cross-tenant view.
type Metrics struct {
	// TenantID is the tenant scope of the snapshot. Empty when the
	// snapshot is cross-tenant (super-admin only).
	TenantID string `json:"tenantId"`
	// CapturedAt is the moment the metrics were materialized.
	CapturedAt timex.DateTime `json:"capturedAt"`
	// InstanceCounts reports running/approved/rejected/withdrawn/returned/
	// terminated instance counts. Keys are the InstanceStatus string values.
	InstanceCounts map[string]int `json:"instanceCounts"`
	// TaskCounts reports task counts indexed by TaskStatus string.
	TaskCounts map[string]int `json:"taskCounts"`
	// TimeoutTaskCount is the number of pending tasks past their deadline.
	TimeoutTaskCount int `json:"timeoutTaskCount"`
	// AvgCompletionSeconds is the average end-to-end duration (created_at →
	// finished_at) for instances that reached a final status in the
	// reporting window. -1 indicates "no completed instances yet".
	AvgCompletionSeconds float64 `json:"avgCompletionSeconds"`
	// PendingBindingFailures is the number of distinct instances that have
	// an unresolved business-binding failure event (best-effort estimate).
	PendingBindingFailures int `json:"pendingBindingFailures"`
}
