package cron

// Cron store event topics. Both are best-effort operational notifications
// published outside any transaction on the default event route — subscribe
// for alerting; never drive correctness from them (the run journal is the
// durable truth).
const (
	// EventTypeRunFailed is published when a run finishes failed: handler
	// error, panic, or per-run timeout.
	EventTypeRunFailed = "vef.cron.run.failed"
	// EventTypeRunAbandoned is published when the recovery sweep marks a
	// running row abandoned because its node stopped heartbeating.
	EventTypeRunAbandoned = "vef.cron.run.abandoned"
)

// RunFailedEvent reports one failed run.
type RunFailedEvent struct {
	RunID        string `json:"runId"`
	ScheduleName string `json:"scheduleName"`
	JobName      string `json:"jobName"`
	// ScheduledAtUnixMs is the run's logical fire time.
	ScheduledAtUnixMs int64 `json:"scheduledAtUnixMs"`
	// NodeID identifies the node that executed the run.
	NodeID string `json:"nodeId"`
	// Error is the journaled failure message.
	Error string `json:"error"`
}

// NewRunFailedEvent creates a run-failed event from the journal record.
func NewRunFailedEvent(run *Run) *RunFailedEvent {
	return &RunFailedEvent{
		RunID:             run.ID,
		ScheduleName:      run.ScheduleName,
		JobName:           run.JobName,
		ScheduledAtUnixMs: run.ScheduledAtUnixMs,
		NodeID:            run.NodeID,
		Error:             run.Error,
	}
}

// EventType implements event.Event.
func (*RunFailedEvent) EventType() string { return EventTypeRunFailed }

// RunAbandonedEvent reports one abandoned run. Whether it re-fires is the
// schedule's Recover setting.
type RunAbandonedEvent struct {
	RunID        string `json:"runId"`
	ScheduleName string `json:"scheduleName"`
	JobName      string `json:"jobName"`
	// ScheduledAtUnixMs is the run's logical fire time.
	ScheduledAtUnixMs int64 `json:"scheduledAtUnixMs"`
	// NodeID identifies the node that stopped heartbeating.
	NodeID string `json:"nodeId"`
}

// NewRunAbandonedEvent creates a run-abandoned event from the journal record.
func NewRunAbandonedEvent(run *Run) *RunAbandonedEvent {
	return &RunAbandonedEvent{
		RunID:             run.ID,
		ScheduleName:      run.ScheduleName,
		JobName:           run.JobName,
		ScheduledAtUnixMs: run.ScheduledAtUnixMs,
		NodeID:            run.NodeID,
	}
}

// EventType implements event.Event.
func (*RunAbandonedEvent) EventType() string { return EventTypeRunAbandoned }
