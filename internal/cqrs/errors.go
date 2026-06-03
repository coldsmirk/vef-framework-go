package cqrs

import "errors"

// ErrHandlerNotFound is returned when no handler is registered for a command/query type.
var ErrHandlerNotFound = errors.New("cqrs: handler not found")

// ErrResultTypeMismatch is returned when the dispatched value cannot be
// asserted to the caller's expected result type. The normal handler path
// always yields the correct type; this guards a misbehaving Behavior that
// short-circuits with a non-nil value of the wrong concrete type, turning an
// opaque runtime panic into a typed error.
var ErrResultTypeMismatch = errors.New("cqrs: result type mismatch")
