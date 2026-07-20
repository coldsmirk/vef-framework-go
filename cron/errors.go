package cron

import "errors"

var (
	// ErrJobNameRequired indicates job name is required.
	ErrJobNameRequired = errors.New("job name is required")
	// ErrJobTaskHandlerRequired indicates job task handler is required.
	ErrJobTaskHandlerRequired = errors.New("job task handler is required")
	// ErrJobTaskHandlerMustFunc indicates job task handler must be a function.
	ErrJobTaskHandlerMustFunc = errors.New("job task handler must be a function")
)

// TriggerSpec.Validate sentinels. The store's ScheduleManager wraps them into
// the outward ErrTriggerInvalid; they stay assertable for direct spec users.
var (
	// ErrTriggerKindUnknown indicates a kind outside the TriggerKind vocabulary.
	ErrTriggerKindUnknown = errors.New("unknown trigger kind")
	// ErrTriggerExprRequired indicates a cron trigger without an expression.
	ErrTriggerExprRequired = errors.New("cron trigger requires an expression")
	// ErrTriggerExprInvalid indicates an unparsable cron expression.
	ErrTriggerExprInvalid = errors.New("invalid cron expression")
	// ErrTriggerTimezoneInvalid indicates an unloadable IANA timezone.
	ErrTriggerTimezoneInvalid = errors.New("invalid trigger timezone")
	// ErrTriggerFieldsConflict indicates fields that do not belong to the
	// selected trigger kind.
	ErrTriggerFieldsConflict = errors.New("trigger fields conflict with its kind")
	// ErrTriggerIntervalTooShort indicates a fixed rate below MinInterval.
	ErrTriggerIntervalTooShort = errors.New("trigger interval too short")
	// ErrTriggerIntervalTooLong indicates milliseconds outside time.Duration.
	ErrTriggerIntervalTooLong = errors.New("trigger interval exceeds the supported duration")
	// ErrTriggerFireTimeRequired indicates a one-shot trigger without a fire time.
	ErrTriggerFireTimeRequired = errors.New("one-shot trigger requires a fire time")
)
