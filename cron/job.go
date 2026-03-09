package cron

import (
	"time"

	"github.com/go-co-op/gocron/v2"
)

// Job represents a scheduled task in the cron system.
// It provides methods to inspect and control individual job instances.
type Job interface {
	// ID returns the job's unique identifier as a string.
	ID() string
	// LastRun returns the time when the job was last executed.
	LastRun() (time.Time, error)
	// Name returns the human-readable name assigned to the job.
	Name() string
	// NextRun returns the time when the job is next scheduled to run.
	NextRun() (time.Time, error)
	// NextRuns returns the specified number of future scheduled run times.
	NextRuns(count int) ([]time.Time, error)
	// RunNow executes the job immediately without affecting its regular schedule.
	// This respects all job and scheduler limits and may affect future scheduling
	// if the job has run limits configured.
	RunNow() error
	// Tags returns the list of tags associated with the job for grouping and filtering.
	Tags() []string
}

// JobDefinition defines how a job should be scheduled and executed.
// Implementations specify different scheduling strategies (cron, duration, one-time, etc.).
type JobDefinition interface {
	// build converts the high-level job definition into gocron-specific components.
	// This is an internal method used by the scheduler implementation.
	build() (gocron.JobDefinition, gocron.Task, []gocron.JobOption, error)
}

// jobAdapter adapts gocron.Job to implement the framework's Job interface.
// It provides a clean abstraction layer over the underlying gocron job.
type jobAdapter struct {
	job gocron.Job
}

func (j *jobAdapter) ID() string {
	return j.job.ID().String()
}

func (j *jobAdapter) LastRun() (time.Time, error) {
	return j.job.LastRun()
}

func (j *jobAdapter) Name() string {
	return j.job.Name()
}

func (j *jobAdapter) NextRun() (time.Time, error) {
	return j.job.NextRun()
}

func (j *jobAdapter) NextRuns(count int) ([]time.Time, error) {
	return j.job.NextRuns(count)
}

func (j *jobAdapter) RunNow() error {
	return j.job.RunNow()
}

func (j *jobAdapter) Tags() []string {
	return j.job.Tags()
}
