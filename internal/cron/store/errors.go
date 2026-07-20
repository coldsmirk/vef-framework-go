package store

import "errors"

var (
	// ErrJobHandlerNameEmpty indicates a registered handler without a job name.
	ErrJobHandlerNameEmpty = errors.New("cron store: job handler name is empty")
	// ErrJobHandlerDuplicate indicates two handlers registered for one job name.
	ErrJobHandlerDuplicate = errors.New("cron store: duplicate job handler")
	// ErrScheduleNameRequired indicates a schedule spec without a name.
	ErrScheduleNameRequired = errors.New("cron store: schedule name is required")
	// ErrScheduleNameTooLong indicates a schedule name beyond the column width.
	ErrScheduleNameTooLong = errors.New("cron store: schedule name exceeds 128 characters")
	// ErrScheduleTimeoutNegative indicates a negative per-run timeout.
	ErrScheduleTimeoutNegative = errors.New("cron store: schedule timeout must not be negative")
	// ErrScheduleWindowInverted indicates EndsAt at or before StartsAt.
	ErrScheduleWindowInverted = errors.New("cron store: schedule window ends before it starts")
	// ErrJobPanicked wraps a recovered handler panic into the run's failure.
	ErrJobPanicked = errors.New("cron store: job panicked")
	// ErrRunTimedOut marks a run that outlived its timeout, whatever its
	// handler returned.
	ErrRunTimedOut = errors.New("cron store: run timed out")
)
