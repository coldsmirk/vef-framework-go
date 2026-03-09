package cron

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestScheduler() (gocron.Scheduler, error) {
	return gocron.NewScheduler(
		gocron.WithLocation(time.Local),
		gocron.WithStopTimeout(5*time.Second),
	)
}

// TestNewScheduler tests scheduler creation with a gocron scheduler.
func TestNewScheduler(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)
	assert.NotNil(t, scheduler, "Scheduler should not be nil")
}

// TestSchedulerNewJobOneTime tests creating and executing a one-time job.
func TestSchedulerNewJobOneTime(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed int32

	testFunc := func() {
		atomic.AddInt32(&executed, 1)
	}

	jobDef := NewOneTimeJob(nil,
		WithName("test-one-time"),
		WithTags("test", "one-time"),
		WithTask(testFunc),
	)

	job, err := scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create one-time job")
	assert.NotNil(t, job, "Job should not be nil")
	assert.Equal(t, "test-one-time", job.Name(), "Job name should match")
	assert.Contains(t, job.Tags(), "test", "Job should have 'test' tag")
	assert.Contains(t, job.Tags(), "one-time", "Job should have 'one-time' tag")

	scheduler.Start()
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&executed), "One-time job should execute exactly once")
}

// TestSchedulerNewJobDuration tests creating and executing a duration-based job with limited runs.
func TestSchedulerNewJobDuration(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed int32

	testFunc := func() {
		atomic.AddInt32(&executed, 1)
	}

	jobDef := NewDurationJob(50*time.Millisecond,
		WithName("test-duration"),
		WithTags("test", "duration"),
		WithLimitedRuns(3),
		WithTask(testFunc),
	)

	job, err := scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create duration job")
	assert.NotNil(t, job, "Job should not be nil")
	assert.Equal(t, "test-duration", job.Name(), "Job name should match")

	scheduler.Start()
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(3), atomic.LoadInt32(&executed), "Duration job should execute exactly 3 times")
}

// TestSchedulerNewJobCron tests creating and executing a cron-based job with limited runs.
func TestSchedulerNewJobCron(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed int32

	testFunc := func() {
		atomic.AddInt32(&executed, 1)
	}

	jobDef := NewCronJob("* * * * * *", true,
		WithName("test-cron"),
		WithTags("test", "cron"),
		WithLimitedRuns(2),
		WithTask(testFunc),
	)

	job, err := scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create cron job")
	assert.NotNil(t, job, "Job should not be nil")
	assert.Equal(t, "test-cron", job.Name(), "Job name should match")

	scheduler.Start()
	time.Sleep(2500 * time.Millisecond)

	assert.Equal(t, int32(2), atomic.LoadInt32(&executed), "Cron job should execute exactly 2 times")
}

// TestSchedulerNewJobDurationRandom tests creating and executing a random duration job.
func TestSchedulerNewJobDurationRandom(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed int32

	testFunc := func() {
		atomic.AddInt32(&executed, 1)
	}

	jobDef := NewDurationRandomJob(10*time.Millisecond, 50*time.Millisecond,
		WithName("test-random"),
		WithTags("test", "random"),
		WithLimitedRuns(2),
		WithTask(testFunc),
	)

	job, err := scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create random duration job")
	assert.NotNil(t, job, "Job should not be nil")
	assert.Equal(t, "test-random", job.Name(), "Job name should match")

	scheduler.Start()
	time.Sleep(200 * time.Millisecond)

	executions := atomic.LoadInt32(&executed)
	assert.GreaterOrEqual(t, executions, int32(1), "Random duration job should execute at least once")
	assert.LessOrEqual(t, executions, int32(2), "Random duration job should not exceed limit")
}

// TestSchedulerJobs tests retrieving all scheduled jobs.
func TestSchedulerJobs(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	jobs := scheduler.Jobs()
	assert.Len(t, jobs, 0, "Scheduler should have no jobs initially")

	jobDef := NewOneTimeJob(nil,
		WithName("test-job"),
		WithTask(func() {}),
	)

	_, err = scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create job")

	jobs = scheduler.Jobs()
	assert.Len(t, jobs, 1, "Scheduler should have exactly 1 job")
	assert.Equal(t, "test-job", jobs[0].Name(), "Job name should match")
}

// TestSchedulerRemoveJob tests removing a job by ID.
func TestSchedulerRemoveJob(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	jobDef := NewOneTimeJob(nil,
		WithName("test-job"),
		WithTask(func() {}),
	)

	job, err := scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create job")

	jobs := scheduler.Jobs()
	assert.Len(t, jobs, 1, "Scheduler should have 1 job before removal")

	err = scheduler.RemoveJob(job.ID())
	require.NoError(t, err, "Should remove job")

	jobs = scheduler.Jobs()
	assert.Len(t, jobs, 0, "Scheduler should have no jobs after removal")
}

// TestSchedulerRemoveByTags tests removing jobs by tag.
func TestSchedulerRemoveByTags(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	job1Def := NewOneTimeJob(nil,
		WithName("job1"),
		WithTags("group1", "test"),
		WithTask(func() {}),
	)

	job2Def := NewOneTimeJob(nil,
		WithName("job2"),
		WithTags("group2", "test"),
		WithTask(func() {}),
	)

	job3Def := NewOneTimeJob(nil,
		WithName("job3"),
		WithTags("group1"),
		WithTask(func() {}),
	)

	_, err = scheduler.NewJob(job1Def)
	require.NoError(t, err, "Should create job1")
	_, err = scheduler.NewJob(job2Def)
	require.NoError(t, err, "Should create job2")
	_, err = scheduler.NewJob(job3Def)
	require.NoError(t, err, "Should create job3")

	jobs := scheduler.Jobs()
	assert.Len(t, jobs, 3, "Scheduler should have 3 jobs before removal")

	scheduler.RemoveByTags("test")

	jobs = scheduler.Jobs()
	assert.Len(t, jobs, 1, "Scheduler should have 1 job after removing by tag")
	assert.Equal(t, "job3", jobs[0].Name(), "Remaining job should be job3")
}

// TestSchedulerUpdateJob tests updating an existing job.
func TestSchedulerUpdateJob(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed1, executed2 int32

	testFunc1 := func() {
		atomic.AddInt32(&executed1, 1)
	}
	testFunc2 := func() {
		atomic.AddInt32(&executed2, 1)
	}

	jobDef1 := NewOneTimeJob(nil,
		WithName("test-job"),
		WithTask(testFunc1),
	)

	job, err := scheduler.NewJob(jobDef1)
	require.NoError(t, err, "Should create original job")

	originalID := job.ID()

	jobDef2 := NewOneTimeJob(nil,
		WithName("updated-job"),
		WithTask(testFunc2),
	)

	updatedJob, err := scheduler.Update(originalID, jobDef2)
	require.NoError(t, err, "Should update job")
	assert.Equal(t, "updated-job", updatedJob.Name(), "Updated job name should match")

	scheduler.Start()
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(0), atomic.LoadInt32(&executed1), "Original task should not execute")
	assert.Equal(t, int32(1), atomic.LoadInt32(&executed2), "Updated task should execute once")
}

// TestSchedulerWithContext tests job cancellation via context.
func TestSchedulerWithContext(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed int32

	ctx, cancel := context.WithCancel(context.Background())

	testFunc := func(jobCtx context.Context) {
		select {
		case <-jobCtx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			atomic.AddInt32(&executed, 1)
		}
	}

	jobDef := NewDurationJob(50*time.Millisecond,
		WithName("test-context"),
		WithContext(ctx),
		WithTask(testFunc),
	)

	_, err = scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create job with context")

	scheduler.Start()
	time.Sleep(25 * time.Millisecond)

	cancel()
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(0), atomic.LoadInt32(&executed), "Job should not execute after context cancellation")
}

// TestSchedulerStopJobs tests stopping all jobs.
func TestSchedulerStopJobs(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed int32

	testFunc := func() {
		atomic.AddInt32(&executed, 1)
	}

	jobDef := NewDurationJob(50*time.Millisecond,
		WithName("test-stop"),
		WithTask(testFunc),
	)

	_, err = scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create job")

	scheduler.Start()
	time.Sleep(100 * time.Millisecond)

	err = scheduler.StopJobs()
	require.NoError(t, err, "Should stop jobs")

	executedAfterStop := atomic.LoadInt32(&executed)

	time.Sleep(150 * time.Millisecond)

	finalExecuted := atomic.LoadInt32(&executed)

	assert.Equal(t, executedAfterStop, finalExecuted, "Jobs should not execute after being stopped")
}

// TestJobRunNow tests triggering a job immediately.
func TestJobRunNow(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	var executed int32

	testFunc := func() {
		atomic.AddInt32(&executed, 1)
	}

	futureTime := time.Now().Add(1 * time.Hour)
	jobDef := NewOneTimeJob([]time.Time{futureTime},
		WithName("test-run-now"),
		WithTask(testFunc),
	)

	job, err := scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create job")

	scheduler.Start()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&executed), "Job should not execute before scheduled time")

	err = job.RunNow()
	require.NoError(t, err, "Should trigger job immediately")

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&executed), "Job should execute immediately after RunNow")
}

// TestJobNextRuns tests retrieving future run times.
func TestJobNextRuns(t *testing.T) {
	gocronScheduler, err := createTestScheduler()
	require.NoError(t, err, "Should create gocron scheduler")

	defer func() {
		if err := gocronScheduler.Shutdown(); err != nil {
			t.Errorf("Failed to shutdown gocron scheduler: %v", err)
		}
	}()

	scheduler := NewScheduler(gocronScheduler)

	jobDef := NewCronJob("0 * * * * *", true,
		WithName("test-next-runs"),
		WithTask(func() {}),
	)

	job, err := scheduler.NewJob(jobDef)
	require.NoError(t, err, "Should create job")

	scheduler.Start()
	time.Sleep(10 * time.Millisecond)

	nextRuns, err := job.NextRuns(3)
	require.NoError(t, err, "Should get next runs")
	assert.Len(t, nextRuns, 3, "Should return 3 next run times")

	for i := 1; i < len(nextRuns); i++ {
		diff := nextRuns[i].Sub(nextRuns[i-1])
		assert.InDelta(t, time.Minute, diff, float64(time.Second), "Next runs should be ~1 minute apart")
	}
}
