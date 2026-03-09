package handler

import "reflect"

// Func wraps a handler function for the API dispatch system.
type Func interface {
	// IsFactory returns true if this handler is a factory function that returns the actual handler.
	IsFactory() bool
	// H returns the underlying handler function as a reflect.Value for reflective invocation.
	H() reflect.Value
}
