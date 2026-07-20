package cron

import (
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

// jobMonitor implements the gocron.Monitor interface to track job execution
// metrics. Routine lifecycle events (scheduled, completed successfully) log
// at Debug — framework maintenance jobs tick every few seconds and would
// otherwise flood steady-state logs; the durable store's run journal, not
// this chatter, is the execution record. Failures keep their level.
type jobMonitor struct{}

func (*jobMonitor) RecordJobTimingWithStatus(startTime, endTime time.Time, id uuid.UUID, name string, tags []string, status gocron.JobStatus, err error) {
	switch status {
	case gocron.Success:
		logger.Debugf(
			"Job %q completed | id: %s | tags: %s | elapsed: %s | status: %s",
			name,
			id.String(),
			strings.Join(tags, ", "),
			endTime.Sub(startTime),
			status,
		)

	case gocron.Fail:
		logger.Errorf(
			"Job %q completed | id: %s | tags: %s | elapsed: %s | status: %s | error: %v",
			name,
			id.String(),
			strings.Join(tags, ", "),
			endTime.Sub(startTime),
			status,
			err,
		)

	default:
		logger.Warnf(
			"Job %q completed | id: %s | tags: %s | elapsed: %s | status: %s",
			name,
			id.String(),
			strings.Join(tags, ", "),
			endTime.Sub(startTime),
			status,
		)
	}
}

func (*jobMonitor) IncrementJob(id uuid.UUID, name string, tags []string, status gocron.JobStatus) {
	logger.Debugf(
		"Job %q scheduled | id: %s | tags: %s | status: %s",
		name,
		id.String(),
		strings.Join(tags, ", "),
		status,
	)
}

func (*jobMonitor) RecordJobTiming(startTime, endTime time.Time, id uuid.UUID, name string, tags []string) {
	logger.Debugf(
		"Job %q completed | id: %s | tags: %s | elapsed: %s",
		name,
		id.String(),
		strings.Join(tags, ", "),
		endTime.Sub(startTime),
	)
}

func newJobMonitor() *jobMonitor {
	return &jobMonitor{}
}
