package taskpool

import "sync/atomic"

// poolStats tracks pool-level statistics using atomic operations for thread-safety.
type poolStats struct {
	// totalSubmitted counts all tasks submitted to the pool
	totalSubmitted atomic.Uint64

	// totalCompleted counts successfully completed tasks
	totalCompleted atomic.Uint64

	// totalFailed counts tasks that failed with errors
	totalFailed atomic.Uint64

	// activeWorkers counts workers currently executing tasks
	activeWorkers atomic.Int32

	// idleWorkers counts workers waiting for tasks
	idleWorkers atomic.Int32
}

// snapshot returns current statistics as SchedulerStats.
func (s *poolStats) snapshot(totalWorkers, queuedTasks int) SchedulerStats {
	return SchedulerStats{
		TotalSubmitted: s.totalSubmitted.Load(),
		TotalCompleted: s.totalCompleted.Load(),
		TotalFailed:    s.totalFailed.Load(),
		ActiveWorkers:  int(s.activeWorkers.Load()),
		IdleWorkers:    int(s.idleWorkers.Load()),
		TotalWorkers:   totalWorkers,
		QueuedTasks:    queuedTasks,
	}
}
