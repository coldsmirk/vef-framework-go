package sequence

import "errors"

var (
	// ErrRuleNotFound indicates the sequence rule was not found or is inactive.
	ErrRuleNotFound = errors.New("sequence rule not found or inactive")
	// ErrSequenceOverflow indicates the sequence value has exceeded its configured MaxValue.
	ErrSequenceOverflow = errors.New("sequence value exceeded max value")
	// ErrInvalidCount indicates the requested count is invalid (must be >= 1).
	ErrInvalidCount = errors.New("sequence generate count must be >= 1")
)
