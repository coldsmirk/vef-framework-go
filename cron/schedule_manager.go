package cron

import (
	"context"
	"time"
)

// ScheduleManager is the management surface of the durable schedule store:
// create, reshape, and control schedules, and read the run journal. It is
// available in DI whenever the cron module is loaded; with the store disabled
// (vef.cron.store.enabled=false) every method returns ErrStoreDisabled.
type ScheduleManager interface {
	// Create validates and persists a new schedule. The trigger must be
	// structurally sound and the job name registered on this node; a taken
	// name fails with ErrScheduleExists.
	Create(ctx context.Context, spec ScheduleSpec) (*Schedule, error)
	// Update reshapes the named schedule to the spec — including a rename
	// when the spec carries a different, untaken name. Trigger or window
	// changes recompute the next fire.
	Update(ctx context.Context, name string, spec ScheduleSpec) (*Schedule, error)
	// Delete removes the schedule. Journaled runs are kept — they carry the
	// schedule and job names denormalized.
	Delete(ctx context.Context, name string) error
	// Pause stops fire claiming until Resume; running fires are unaffected.
	Pause(ctx context.Context, name string) error
	// Resume re-enables the schedule. Occurrences missed while paused are
	// handled by the schedule's misfire policy: MisfireFireNow runs one
	// catch-up immediately, MisfireSkip waits for the next regular fire.
	Resume(ctx context.Context, name string) error
	// TriggerNow persists one independent immediate fire request (single node,
	// journaled, concurrency policy respected) without moving the regular
	// trigger cursor. A paused schedule fails with ErrScheduleDisabled.
	TriggerNow(ctx context.Context, name string) error
	// Get returns the named schedule, or ErrScheduleNotFound.
	Get(ctx context.Context, name string) (*Schedule, error)
	// List returns schedules matching the filter, ordered by name.
	List(ctx context.Context, filter ScheduleFilter) ([]Schedule, error)
	// ListRuns returns journal records matching the filter, newest first.
	ListRuns(ctx context.Context, filter RunFilter) ([]Run, error)
}

// ScheduleFilter narrows List; zero values match everything.
type ScheduleFilter struct {
	// JobName matches schedules of one job.
	JobName string
	// Enabled matches by enablement when non-nil.
	Enabled *bool
}

// RunFilter narrows ListRuns; zero values match everything.
type RunFilter struct {
	// ScheduleName matches runs of one schedule.
	ScheduleName string
	// JobName matches runs of one job.
	JobName string
	// Statuses matches any of the given statuses.
	Statuses []RunStatus
	// Since and Until bound the logical fire time (inclusive since,
	// exclusive until).
	Since *time.Time
	Until *time.Time
	// Limit caps the result; zero resolves to 100, the cap is 1000.
	Limit int
}
