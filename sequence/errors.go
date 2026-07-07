package sequence

import "errors"

var (
	// ErrRuleNotFound indicates the sequence rule was not found or is inactive.
	ErrRuleNotFound = errors.New("sequence rule not found or inactive")
	// ErrSequenceOverflow indicates the sequence value has exceeded its configured MaxValue.
	ErrSequenceOverflow = errors.New("sequence value exceeded max value")
	// ErrInvalidCount indicates the requested count is invalid (must be >= 1).
	ErrInvalidCount = errors.New("sequence generate count must be >= 1")
	// ErrInvalidStep indicates the rule's SeqStep is invalid (must be >= 1).
	// A zero step would mint identical serial numbers for a whole batch while
	// the overflow ceiling never triggers; a negative step walks the counter
	// backwards past already-issued numbers.
	ErrInvalidStep = errors.New("sequence rule SeqStep must be >= 1")

	// errMissingField indicates a required field is missing from Redis hash data.
	errMissingField = errors.New("missing field")
)
