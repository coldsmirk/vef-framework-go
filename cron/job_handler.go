package cron

import (
	"context"
	"fmt"
)

// JobHandler executes one named, durable job. Register implementations with
// vef.ProvideCronJobHandler — exactly one handler per job name; schedules
// reference the name as their JobName. A fire is claimed by at most one node,
// but a crashed run re-fires when Schedule.Recover is set, so handlers should
// be idempotent.
type JobHandler interface {
	// Name uniquely identifies the job.
	Name() string
	// Execute runs one fire. The context carries cancellation (graceful
	// shutdown, per-run timeout); a returned error journals the run as
	// failed.
	Execute(ctx context.Context, execution Execution) error
}

// DefaultScheduleProvider is an optional JobHandler capability: a handler
// shipping a default schedule. The store seeds it at boot when no schedule
// of that name exists yet — operator changes are never overwritten. The
// spec's Name falls back to the job name.
type DefaultScheduleProvider interface {
	// DefaultSchedule returns the schedule to seed.
	DefaultSchedule() ScheduleSpec
}

// JobHandlerOption configures a handler built by NewJobHandler or
// NewTypedJobHandler.
type JobHandlerOption func(*jobHandlerConfig)

type jobHandlerConfig struct {
	defaultSchedule *ScheduleSpec
}

// WithDefaultSchedule ships a default schedule with the handler; the store
// seeds it at boot when absent (see DefaultScheduleProvider).
func WithDefaultSchedule(spec ScheduleSpec) JobHandlerOption {
	return func(c *jobHandlerConfig) {
		c.defaultSchedule = &spec
	}
}

// NewJobHandler adapts a function to a JobHandler.
func NewJobHandler(name string, execute func(ctx context.Context, execution Execution) error, opts ...JobHandlerOption) JobHandler {
	return newConfiguredHandler(&funcJobHandler{name: name, execute: execute}, opts)
}

// NewTypedJobHandler adapts a typed function to a JobHandler: the schedule's
// params are decoded into P before the function runs; a decode failure
// journals the run as failed without invoking it.
func NewTypedJobHandler[P any](name string, execute func(ctx context.Context, params P) error, opts ...JobHandlerOption) JobHandler {
	return NewJobHandler(name, func(ctx context.Context, execution Execution) error {
		var params P
		if err := execution.BindParams(&params); err != nil {
			return fmt.Errorf("job %q: %w", name, err)
		}

		return execute(ctx, params)
	}, opts...)
}

// newConfiguredHandler applies options, wrapping the handler with its default
// schedule when one is shipped.
func newConfiguredHandler(handler JobHandler, opts []JobHandlerOption) JobHandler {
	var config jobHandlerConfig
	for _, opt := range opts {
		opt(&config)
	}

	if config.defaultSchedule == nil {
		return handler
	}

	return &seededJobHandler{JobHandler: handler, spec: *config.defaultSchedule}
}

// funcJobHandler adapts a plain function to the JobHandler interface.
type funcJobHandler struct {
	name    string
	execute func(ctx context.Context, execution Execution) error
}

func (h *funcJobHandler) Name() string {
	return h.name
}

func (h *funcJobHandler) Execute(ctx context.Context, execution Execution) error {
	return h.execute(ctx, execution)
}

// seededJobHandler decorates a handler with its shipped default schedule.
type seededJobHandler struct {
	JobHandler

	spec ScheduleSpec
}

func (h *seededJobHandler) DefaultSchedule() ScheduleSpec {
	return h.spec
}
