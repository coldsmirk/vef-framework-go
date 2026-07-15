package jsevents

import "errors"

var (
	// ErrEmptyEventType is thrown into the script when events.publish is
	// called without an event type.
	ErrEmptyEventType = errors.New("jsevents: empty event type")
	// ErrEventTypeNotAllowed is thrown into the script when the event type is
	// outside the allowlist configured with WithAllowedTypes.
	ErrEventTypeNotAllowed = errors.New("jsevents: event type not allowed")
)
