package taskpool

import (
	"context"
	"time"

	"github.com/rs/xid"
)

type Priority int

const (
	PriorityLow    Priority = 0
	PriorityMedium Priority = 1
	PriorityHigh   Priority = 2
)

// Task is a unit of work executed by a worker.
type Task[TIn, TOut any] struct {
	ID          string
	Context     context.Context
	Priority    Priority
	Payload     TIn
	Result      chan<- Result[TOut]
	Done        chan struct{}
	SubmittedAt time.Time
}

// Result contains the outcome of task execution.
type Result[TOut any] struct {
	TaskID   string
	Data     TOut
	Error    error
	Duration time.Duration
}

func generateTaskID() string {
	return xid.New().String()
}
