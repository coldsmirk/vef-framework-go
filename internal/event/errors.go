package event

import "errors"

var (
	// ErrEventBusAlreadyStarted indicates event bus already started.
	ErrEventBusAlreadyStarted = errors.New("event bus already started")
	// ErrShutdownTimeoutExceeded indicates shutdown wait timeout.
	ErrShutdownTimeoutExceeded = errors.New("shutdown timeout exceeded")
)
