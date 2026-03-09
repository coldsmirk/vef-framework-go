package timex

import "errors"

var (
	// ErrInvalidDateFormat indicates date format is invalid.
	ErrInvalidDateFormat = errors.New("invalid date format")
	// ErrInvalidDateTimeFormat indicates datetime format is invalid.
	ErrInvalidDateTimeFormat = errors.New("invalid datetime format")
	// ErrInvalidTimeFormat indicates time format is invalid.
	ErrInvalidTimeFormat = errors.New("invalid time format")
	// ErrFailedScan indicates scan target type/value is invalid.
	ErrFailedScan = errors.New("failed to scan value")
	// ErrUnsupportedDestType indicates dest type is unsupported.
	ErrUnsupportedDestType = errors.New("unsupported destination type")
	// ErrInvalidJSONFormat indicates invalid JSON length/quotes for datetime types.
	ErrInvalidJSONFormat = errors.New("invalid JSON format: expected quoted value of specific length")
)
