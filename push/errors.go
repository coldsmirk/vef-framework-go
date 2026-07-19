package push

import "errors"

var (
	// ErrNoTarget reports a Push call with an empty target set or an empty
	// users/roles selector; delivering to nobody is always a caller bug, never
	// a valid broadcast.
	ErrNoTarget = errors.New("push: at least one target is required")
	// ErrTypeRequired reports a message without a type; clients dispatch on it.
	ErrTypeRequired = errors.New("push: message type is required")
	// ErrUnknownTargetKind reports a hand-built target whose kind is not part
	// of the TargetKind vocabulary.
	ErrUnknownTargetKind = errors.New("push: unknown target kind")
)
