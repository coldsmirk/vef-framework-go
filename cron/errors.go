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
	// ErrTriggerIntervalTooShort indicates a fixed rate below MinInterval.
	ErrTriggerIntervalTooShort = errors.New("trigger interval too short")
	// ErrTriggerFireTimeRequired indicates a one-shot trigger without a fire time.
	ErrTriggerFireTimeRequired = errors.New("one-shot trigger requires a fire time")
)
