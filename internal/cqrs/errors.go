package cqrs

import "errors"

// ErrHandlerNotFound is returned when no handler is registered for a command/query type.
var ErrHandlerNotFound = errors.New("cqrs: handler not found")
