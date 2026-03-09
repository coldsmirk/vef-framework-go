package param

import "errors"

var (
	// ErrResolveHandlerParamType indicates failing to resolve handler parameter type.
	ErrResolveHandlerParamType = errors.New("failed to resolve api handler parameter type")
	// ErrResolveFactoryParamType indicates failing to resolve factory function parameter type.
	ErrResolveFactoryParamType = errors.New("failed to resolve api handler factory parameter type")
	// ErrRequestNotFound indicates that the request object was not found in the context.
	ErrRequestNotFound = errors.New("request not found in context")
)
