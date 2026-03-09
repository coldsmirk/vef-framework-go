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
