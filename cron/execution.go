package cron

import (
	"encoding/json"
	"fmt"
	"time"
)

// Execution describes the run a JobHandler is executing: the identity of the
// run and its schedule, the logical fire time, and the schedule's params. It
// is a read-only view — the journal record itself stays engine-owned.
type Execution struct {
	// RunID is the journal record's identifier, for run-scoped correlation.
	// A Recover re-fire is a fresh run with a fresh RunID.
	RunID string
	// ScheduleID and ScheduleName identify the schedule that fired.
	ScheduleID   string
	ScheduleName string
	// JobName is the handler's own registered name.
	JobName string
	// ScheduledAt is the logical fire time; a catch-up fire executes later
	// than it.
	ScheduledAt time.Time
	// Params is the schedule's params column, verbatim.
	Params json.RawMessage
}

// BindParams decodes the schedule's params into v; absent params leave v
// untouched.
func (e Execution) BindParams(v any) error {
	if len(e.Params) == 0 {
		return nil
	}

	if err := json.Unmarshal(e.Params, v); err != nil {
		return fmt.Errorf("cron: bind schedule params: %w", err)
	}

	return nil
}
