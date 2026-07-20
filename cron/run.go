package cron

import "github.com/coldsmirk/vef-framework-go/orm"

// RunStatus is the lifecycle state of one journaled run.
type RunStatus string

const (
	// RunRunning marks a claimed fire that is executing (or about to).
	RunRunning RunStatus = "running"
	// RunSucceeded marks a run whose handler returned nil.
	RunSucceeded RunStatus = "succeeded"
	// RunFailed marks a run whose handler returned an error, panicked, or
	// exceeded its timeout.
	RunFailed RunStatus = "failed"
	// RunMissed records occurrences that misfire handling decided will never
	// run; one row covers a whole catch-up gap (see MissedCount).
	RunMissed RunStatus = "missed"
	// RunSkipped records a fire suppressed by ConcurrencyForbid.
	RunSkipped RunStatus = "skipped"
	// RunAbandoned marks a running row whose executor stopped heartbeating;
	// schedules with Recover set re-fire it.
	RunAbandoned RunStatus = "abandoned"
	// RunCanceled marks a run interrupted by graceful shutdown.
	RunCanceled RunStatus = "canceled"
)

// IsTerminal reports whether the status is final.
func (s RunStatus) IsTerminal() bool {
	return s != RunRunning
}

// Run is the journal record of one fire: its logical time, execution window,
// executing node, and outcome. Rows survive schedule deletion — ScheduleName
// and JobName are denormalized for that reason.
type Run struct {
	orm.BaseModel `bun:"table:crn_run,alias:cr"`
	orm.CreationAuditedModel

	ScheduleID   string `json:"scheduleId" bun:"schedule_id"`
	ScheduleName string `json:"scheduleName" bun:"schedule_name"`
	JobName      string `json:"jobName" bun:"job_name"`

	// ScheduledAtUnixMs is the logical fire time; a catch-up fire starts later
	// than it. It is deliberately not unique: manual and recovery fires may
	// legitimately share one instant.
	ScheduledAtUnixMs int64 `json:"scheduledAtUnixMs" bun:"scheduled_at_unix_ms"`
	ClaimedAtUnixMs   int64 `json:"claimedAtUnixMs" bun:"claimed_at_unix_ms"`

	Status RunStatus `json:"status" bun:"status"`

	// NodeID identifies the executing node; empty on rows that never
	// executed (missed, skipped).
	NodeID string `json:"nodeId" bun:"node_id"`

	StartedAtUnixMs  *int64 `json:"startedAtUnixMs,omitempty" bun:"started_at_unix_ms"`
	FinishedAtUnixMs *int64 `json:"finishedAtUnixMs,omitempty" bun:"finished_at_unix_ms"`
	DurationMs       int64  `json:"durationMs" bun:"duration_ms"`

	// HeartbeatAtUnixMs is the executor's liveness signal, renewed while the
	// run executes; a stale heartbeat turns the run abandoned.
	HeartbeatAtUnixMs *int64 `json:"heartbeatAtUnixMs,omitempty" bun:"heartbeat_at_unix_ms"`

	// Error is the failure message, truncated; empty on success.
	Error string `json:"error,omitempty" bun:"error"`

	// MissedCount is the number of occurrences a missed row covers.
	MissedCount int `json:"missedCount,omitempty" bun:"missed_count"`
}
