package expression

import "errors"

// ErrUnexpectedType is returned when a Value cannot be read as the requested
// concrete type.
var ErrUnexpectedType = errors.New("expression: unexpected result type")
