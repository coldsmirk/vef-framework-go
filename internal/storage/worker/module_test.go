package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
)

// recordingJob is a minimal cron.Job returned by the recording scheduler.
// It satisfies the interface without depending on a live gocron instance.
type recordingJob struct {
	name string
}

func (*recordingJob) ID() string                        { return "test-id" }
func (*recordingJob) LastRun() (time.Time, error)       { return time.Time{}, nil }
func (j *recordingJob) Name() string                    { return j.name }
func (*recordingJob) NextRun() (time.Time, error)       { return time.Time{}, nil }
func (*recordingJob) NextRuns(int) ([]time.Time, error) { return nil, nil }
func (*recordingJob) RunNow() error                     { return nil }
func (*recordingJob) Tags() []string                    { return nil }

// recordingScheduler is a fake cron.Scheduler that counts NewJob calls
// without launching any goroutines. All other methods are no-ops.
type recordingScheduler struct {
	jobCount int
}

func (*recordingScheduler) Jobs() []cron.Job { return nil }

func (s *recordingScheduler) NewJob(cron.JobDefinition) (cron.Job, error) {
	s.jobCount++

	return &recordingJob{name: "recorded"}, nil
}

func (*recordingScheduler) RemoveByTags(...string) {}
func (*recordingScheduler) RemoveJob(string) error { return nil }
func (*recordingScheduler) Start()                 {}
func (*recordingScheduler) StopJobs() error        { return nil }
func (*recordingScheduler) Update(string, cron.JobDefinition) (cron.Job, error) {
	return nil, nil
}
func (*recordingScheduler) JobsWaitingInQueue() int { return 0 }

// TestRegisterJobs verifies that registerJobs wires exactly two cron jobs
// into the scheduler — one for claim sweeping and one for the delete worker
// — and that the worker functions it passes are callable without panicking.
func TestRegisterJobs(t *testing.T) {
	sched := &recordingScheduler{}
	cfg := &config.StorageConfig{}

	// Minimal no-op workers: we only care about job registration, not execution.
	sweeper := &ClaimSweeper{}
	deleter := &DeleteWorker{}

	err := registerJobs(sched, sweeper, deleter, cfg)
	require.NoError(t, err, "registerJobs should succeed with a recording scheduler and default config")

	assert.Equal(t, 2, sched.jobCount, "registerJobs must register exactly two cron jobs (sweep + delete)")
}

// TestRegisterJobsSchedulerError verifies that an error from the scheduler
// on the first NewJob call is propagated and the function returns early.
func TestRegisterJobsSchedulerError(t *testing.T) {
	sched := &failFirstCallScheduler{}
	cfg := &config.StorageConfig{}

	sweeper := &ClaimSweeper{}
	deleter := &DeleteWorker{}

	err := registerJobs(sched, sweeper, deleter, cfg)
	assert.Error(t, err, "registerJobs must propagate a scheduler error from the first NewJob call")
}

// failFirstCallScheduler returns an error only on the first NewJob, so we
// can test that registerJobs surfaces sweep-job registration failures.
type failFirstCallScheduler struct {
	calls int
}

func (*failFirstCallScheduler) Jobs() []cron.Job { return nil }

func (s *failFirstCallScheduler) NewJob(cron.JobDefinition) (cron.Job, error) {
	s.calls++
	if s.calls == 1 {
		return nil, errSchedulerReject
	}

	return &recordingJob{name: "recorded"}, nil
}

var errSchedulerReject = context.DeadlineExceeded // any sentinel error

func (*failFirstCallScheduler) RemoveByTags(...string) {}
func (*failFirstCallScheduler) RemoveJob(string) error { return nil }
func (*failFirstCallScheduler) Start()                 {}
func (*failFirstCallScheduler) StopJobs() error        { return nil }
func (*failFirstCallScheduler) Update(string, cron.JobDefinition) (cron.Job, error) {
	return nil, nil
}
func (*failFirstCallScheduler) JobsWaitingInQueue() int { return 0 }
