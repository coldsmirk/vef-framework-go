package shared

import (
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/api"
)

// Sentinel errors for API engine operations.
var (
	ErrResourceNil              = errors.New("resource cannot be nil")
	ErrResourceNameEmpty        = errors.New("resource name cannot be empty")
	ErrOperationNotFound        = errors.New("operation not found")
	ErrOperationActionEmpty     = errors.New("operation action cannot be empty")
	ErrNoRouterForKind          = errors.New("no router can handle operation type")
	ErrNoRouterFound            = errors.New("no router found")
	ErrNoHandlerResolverFound   = errors.New("no handler resolver found")
	ErrNoHandlerAdapterFound    = errors.New("no handler adapter found")
	ErrHandlerRequired          = errors.New("handler is required for REST operations")
	ErrMethodNotFound           = errors.New("api action method not found")
	ErrMethodAmbiguous          = errors.New("api action method matches multiple methods")
	ErrHandlerInvalidReturnType = errors.New("handler method has invalid return type, must be 'error'")
	ErrHandlerTooManyReturns    = errors.New("handler method has too many return values")
	ErrHandlerMustBeFunc        = errors.New("provided handler must be a function")
	ErrHandlerNil               = errors.New("provided handler function cannot be nil")
)

// formatIdentifier returns a formatted string for an api.Identifier.
func formatIdentifier(id *api.Identifier) string {
	return fmt.Sprintf("resource=%q action=%q version=%q", id.Resource, id.Action, id.Version)
}

type BaseError struct {
	Identifier *api.Identifier
	Err        error
}

func (e *BaseError) Error() string {
	if e.Identifier != nil {
		return fmt.Sprintf("%s - %v", formatIdentifier(e.Identifier), e.Err)
	}

	return e.Err.Error()
}

// Unwrap returns the underlying error, allowing errors.As and errors.Is to work correctly.
func (e *BaseError) Unwrap() error {
	return e.Err
}

type DuplicateError struct {
	BaseError

	Existing *api.Operation
}

func (e *DuplicateError) Error() string {
	if e.Identifier != nil {
		return fmt.Sprintf("duplicate api definition: %s (attempting to override existing api)", formatIdentifier(e.Identifier))
	}

	return "duplicate api definition"
}

type NotFoundError struct {
	BaseError

	Suggestion *api.Identifier
}

func (e *NotFoundError) Error() string {
	if e.Identifier == nil {
		return "api not found"
	}

	msg := "api not found: " + formatIdentifier(e.Identifier)

	if e.Suggestion != nil {
		msg += " - did you mean: " + formatIdentifier(e.Suggestion) + " ?"
	}

	return msg
}
