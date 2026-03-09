package taskpool

import "errors"

var (
	ErrQueueFull          = errors.New("taskpool: queue is full")
	ErrWorkerStopped      = errors.New("taskpool: worker is stopped")
	ErrPoolShutdown       = errors.New("taskpool: pool is shutdown")
	ErrInvalidPriority    = errors.New("taskpool: invalid priority")
	ErrInvalidConfig      = errors.New("taskpool: invalid configuration")
	ErrDelegateInitFailed = errors.New("taskpool: delegate initialization failed")
	ErrMaxWorkersReached  = errors.New("taskpool: max workers reached")
)
