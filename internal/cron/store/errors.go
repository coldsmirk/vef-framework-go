package store

import "errors"

var (
	// ErrJobHandlerNameEmpty indicates a registered handler without a job name.
	ErrJobHandlerNameEmpty = errors.New("cron store: job handler name is empty")
	// ErrJobHandlerNameTooLong indicates a job name beyond the persisted width.
	ErrJobHandlerNameTooLong = errors.New("cron store: job handler name exceeds 128 characters")
	// ErrJobHandlerDuplicate indicates two handlers registered for one job name.
	ErrJobHandlerDuplicate = errors.New("cron store: duplicate job handler")
	// ErrScheduleNameRequired indicates a schedule spec without a name.
	ErrScheduleNameRequired = errors.New("cron store: schedule name is required")
	// ErrScheduleNameTooLong indicates a schedule name beyond the column width.
	ErrScheduleNameTooLong = errors.New("cron store: schedule name exceeds 128 characters")
	// ErrScheduleTimeoutNegative indicates a negative per-run timeout.
	ErrScheduleTimeoutNegative = errors.New("cron store: schedule timeout must not be negative")
	// ErrScheduleTimeoutTooLong indicates milliseconds outside time.Duration.
	ErrScheduleTimeoutTooLong = errors.New("cron store: schedule timeout exceeds the supported duration")
	// ErrScheduleTimeoutPrecision indicates a timeout that cannot be persisted
	// exactly in whole milliseconds.
	ErrScheduleTimeoutPrecision = errors.New("cron store: schedule timeout must use whole milliseconds")
	// ErrScheduleWindowInverted indicates EndsAt at or before StartsAt.
	ErrScheduleWindowInverted = errors.New("cron store: schedule window must end after it starts")
	// ErrAbandonTakeoverIncomplete indicates a takeover that could not
	// finalize every locked stale run — a broken invariant, since the rows
	// were selected and locked in the same transaction.
	ErrAbandonTakeoverIncomplete = errors.New("cron store: abandoned-run takeover left locked runs unmarked")
	// ErrJobPanicked wraps a recovered handler panic into the run's failure.
	ErrJobPanicked = errors.New("cron store: job panicked")
	// ErrRunTimedOut marks a run that outlived its timeout, whatever its
	// handler returned.
	ErrRunTimedOut = errors.New("cron store: run timed out")
)
