package cron

import (
	"time"

	"github.com/go-co-op/gocron/v2"
)

// applyOptions applies the given options to the job descriptor.
func applyOptions(d *jobDescriptor, options []JobDescriptorOption) {
	for _, option := range options {
		option(d)
	}
}

// OneTimeJobDefinition defines a job that runs once at specified times.
// It supports running immediately, at a single time, or at multiple specific times.
type OneTimeJobDefinition struct {
	jobDescriptor

	times []time.Time
}

func (d *OneTimeJobDefinition) build() (gocron.JobDefinition, gocron.Task, []gocron.JobOption, error) {
	var startAt gocron.OneTimeJobStartAtOption

	switch len(d.times) {
	case 0:
		startAt = gocron.OneTimeJobStartImmediately()
	case 1:
		startAt = gocron.OneTimeJobStartDateTime(d.times[0])
	default:
		startAt = gocron.OneTimeJobStartDateTimes(d.times...)
	}

	definition := gocron.OneTimeJob(startAt)

	task, options, err := d.buildDescriptor()
	if err != nil {
		return nil, nil, nil, err
	}

	return definition, task, options, nil
}

// DurationJobDefinition defines a job that runs repeatedly at fixed intervals.
// The interval is specified as a time.Duration.
type DurationJobDefinition struct {
	jobDescriptor

	interval time.Duration
}

func (d *DurationJobDefinition) build() (gocron.JobDefinition, gocron.Task, []gocron.JobOption, error) {
	definition := gocron.DurationJob(d.interval)

	task, options, err := d.buildDescriptor()
	if err != nil {
		return nil, nil, nil, err
	}

	return definition, task, options, nil
}

// DurationRandomJobDefinition defines a job that runs at random intervals.
// The interval is randomly chosen between MinInterval and MaxInterval for each execution.
type DurationRandomJobDefinition struct {
	jobDescriptor

	minInterval time.Duration
	maxInterval time.Duration
}

func (d *DurationRandomJobDefinition) build() (gocron.JobDefinition, gocron.Task, []gocron.JobOption, error) {
	definition := gocron.DurationRandomJob(d.minInterval, d.maxInterval)

	task, options, err := d.buildDescriptor()
	if err != nil {
		return nil, nil, nil, err
	}

	return definition, task, options, nil
}

// CronJobDefinition defines a job using standard cron expression syntax.
// It supports both standard 5-field and extended 6-field (with seconds) cron expressions.
type CronJobDefinition struct {
	jobDescriptor

	expression  string
	withSeconds bool
}

func (d *CronJobDefinition) build() (gocron.JobDefinition, gocron.Task, []gocron.JobOption, error) {
	definition := gocron.CronJob(d.expression, d.withSeconds)

	task, options, err := d.buildDescriptor()
	if err != nil {
		return nil, nil, nil, err
	}

	return definition, task, options, nil
}

// NewOneTimeJob creates a new one-time job definition with the specified execution times.
// If times is empty, the job will run immediately. If it contains one time, the job runs once at that time.
// If it contains multiple times, the job will run at each specified time.
func NewOneTimeJob(times []time.Time, options ...JobDescriptorOption) *OneTimeJobDefinition {
	definition := &OneTimeJobDefinition{times: times}
	applyOptions(&definition.jobDescriptor, options)

	return definition
}

// NewDurationJob creates a new duration-based job definition that runs at fixed intervals.
func NewDurationJob(interval time.Duration, options ...JobDescriptorOption) *DurationJobDefinition {
	definition := &DurationJobDefinition{interval: interval}
	applyOptions(&definition.jobDescriptor, options)

	return definition
}

// NewDurationRandomJob creates a new random duration job definition.
// The job will execute with a random interval between minInterval and maxInterval for each run.
func NewDurationRandomJob(minInterval, maxInterval time.Duration, options ...JobDescriptorOption) *DurationRandomJobDefinition {
	definition := &DurationRandomJobDefinition{
		minInterval: minInterval,
		maxInterval: maxInterval,
	}
	applyOptions(&definition.jobDescriptor, options)

	return definition
}

// NewCronJob creates a new cron-based job definition using cron expression syntax.
// Set withSeconds to true if the expression includes seconds (6 fields), false for standard 5-field format.
func NewCronJob(expression string, withSeconds bool, options ...JobDescriptorOption) *CronJobDefinition {
	definition := &CronJobDefinition{
		expression:  expression,
		withSeconds: withSeconds,
	}
	applyOptions(&definition.jobDescriptor, options)

	return definition
}
