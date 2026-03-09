package cron

import (
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

// Scheduler manages the lifecycle and execution of cron jobs.
// It provides a high-level interface for job scheduling, management, and control.
type Scheduler interface {
	// Jobs returns all jobs currently registered with the scheduler.
	Jobs() []Job
	// NewJob creates and registers a new job with the scheduler.
	// The job will be scheduled according to its definition when the scheduler is running.
	// If the task function accepts a context.Context as its first parameter,
	// the scheduler will provide cancellation support for graceful shutdown.
	NewJob(definition JobDefinition) (Job, error)
	// RemoveByTags removes all jobs that have any of the specified tags.
	RemoveByTags(tags ...string)
	// RemoveJob removes the job with the specified unique identifier.
	RemoveJob(id string) error
	// Start begins scheduling and executing jobs according to their definitions.
	// Jobs added to a running scheduler are scheduled immediately. This method is non-blocking.
	Start()
	// StopJobs stops the execution of all jobs without removing them from the scheduler.
	// Jobs can be restarted by calling Start() again.
	StopJobs() error
	// Update replaces an existing job's definition while preserving its unique identifier.
	// This allows for dynamic job reconfiguration without losing job history.
	Update(id string, definition JobDefinition) (Job, error)
	// JobsWaitingInQueue returns the number of jobs waiting in the execution queue.
	// This is only relevant when using LimitModeWait; otherwise it returns zero.
	JobsWaitingInQueue() int
}

// schedulerAdapter implements the Scheduler interface by adapting a gocron.Scheduler.
// It provides a clean abstraction layer over the underlying gocron scheduler.
type schedulerAdapter struct {
	scheduler gocron.Scheduler
}

func (s *schedulerAdapter) Jobs() []Job {
	return lo.Map(
		s.scheduler.Jobs(),
		func(job gocron.Job, _ int) Job {
			return &jobAdapter{job: job}
		},
	)
}

func (s *schedulerAdapter) NewJob(definition JobDefinition) (Job, error) {
	def, task, options, err := definition.build()
	if err != nil {
		return nil, err
	}

	job, err := s.scheduler.NewJob(def, task, options...)
	if err != nil {
		return nil, err
	}

	return &jobAdapter{job: job}, nil
}

func (s *schedulerAdapter) RemoveByTags(tags ...string) {
	s.scheduler.RemoveByTags(tags...)
}

func (s *schedulerAdapter) RemoveJob(id string) error {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.scheduler.RemoveJob(uuid)
}

func (s *schedulerAdapter) Start() {
	s.scheduler.Start()
}

func (s *schedulerAdapter) StopJobs() error {
	return s.scheduler.StopJobs()
}

func (s *schedulerAdapter) Update(id string, definition JobDefinition) (Job, error) {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	def, task, options, err := definition.build()
	if err != nil {
		return nil, err
	}

	job, err := s.scheduler.Update(uuid, def, task, options...)
	if err != nil {
		return nil, err
	}

	return &jobAdapter{job: job}, nil
}

func (s *schedulerAdapter) JobsWaitingInQueue() int {
	return s.scheduler.JobsWaitingInQueue()
}

// NewScheduler creates a new Scheduler implementation wrapping the provided gocron.Scheduler.
// This is the main entry point for creating scheduler instances in the application.
func NewScheduler(scheduler gocron.Scheduler) Scheduler {
	return &schedulerAdapter{
		scheduler: scheduler,
	}
}
