package cron

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

// jobInfo contains metadata and configuration options for a cron job.
// It encapsulates all the scheduling parameters and runtime options.
type jobInfo struct {
	name             string
	tags             []string
	allowConcurrent  bool
	startAt          time.Time
	startImmediately bool
	stopAt           time.Time
	limitedRuns      uint
	ctx              context.Context
}

func (i *jobInfo) buildJobOptions() ([]gocron.JobOption, error) {
	if i.name == "" {
		return nil, ErrJobNameRequired
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	options := []gocron.JobOption{
		gocron.WithIdentifier(id),
		gocron.WithName(i.name),
	}

	if len(i.tags) > 0 {
		options = append(options, gocron.WithTags(i.tags...))
	}

	if !i.allowConcurrent {
		options = append(options, gocron.WithSingletonMode(gocron.LimitModeWait))
	}

	if !i.startAt.IsZero() {
		options = append(options, gocron.WithStartAt(gocron.WithStartDateTime(i.startAt)))
	} else if i.startImmediately {
		options = append(options, gocron.WithStartAt(gocron.WithStartImmediately()))
	}

	if !i.stopAt.IsZero() {
		options = append(options, gocron.WithStopAt(gocron.WithStopDateTime(i.stopAt)))
	}

	if i.limitedRuns > 0 {
		options = append(options, gocron.WithLimitedRuns(i.limitedRuns))
	}

	if i.ctx != nil {
		options = append(options, gocron.WithContext(i.ctx))
	}

	return options, nil
}

// jobTask represents the executable task of a cron job.
// It contains the handler function and its parameters.
type jobTask struct {
	handler any
	params  []any
}

func (t *jobTask) buildTask() (gocron.Task, error) {
	if t.handler == nil {
		return nil, ErrJobTaskHandlerRequired
	}

	if reflect.ValueOf(t.handler).Kind() != reflect.Func {
		return nil, ErrJobTaskHandlerMustFunc
	}

	return gocron.NewTask(t.handler, t.params...), nil
}

// jobDescriptor combines job metadata and task information.
// It serves as a builder pattern implementation for creating gocron jobs.
type jobDescriptor struct {
	jobInfo
	jobTask
}

func (d *jobDescriptor) buildDescriptor() (gocron.Task, []gocron.JobOption, error) {
	task, err := d.buildTask()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build job task: %w", err)
	}

	options, err := d.buildJobOptions()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build job options: %w", err)
	}

	return task, options, nil
}
