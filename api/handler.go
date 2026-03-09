package api

import (
	"reflect"

	"github.com/gofiber/fiber/v3"
)

// Middleware represents a processing step in the request pipeline.
type Middleware interface {
	// Name returns the middleware identifier.
	Name() string
	// Order determines execution order.
	// Negative values execute before handler, positive after.
	// Lower values execute first within the same phase.
	Order() int
	// Process handles the request.
	// Call next() to continue the chain.
	Process(ctx fiber.Ctx) error
}

// HandlerResolver resolves a handler from a resource and spec.
type HandlerResolver interface {
	// Resolve finds a handler on the resource and spec.
	// Returns the handler (any type) or an error if not found.
	Resolve(resource Resource, spec OperationSpec) (any, error)
}

// HandlerAdapter converts various handler variants to fiber.Handler.
type HandlerAdapter interface {
	// Adapt converts the handler to a fiber.Handler.
	Adapt(handler any, op *Operation) (fiber.Handler, error)
}

// HandlerParamResolver resolves a handler parameter from the request context.
type HandlerParamResolver interface {
	// Type returns the parameter type this resolver handles.
	Type() reflect.Type
	// Resolve extracts the parameter value from the request.
	Resolve(ctx fiber.Ctx) (reflect.Value, error)
}

// FactoryParamResolver resolves a factory function parameter at startup time.
// Factory functions enable dependency injection at startup while keeping handlers clean.
type FactoryParamResolver interface {
	// Type returns the parameter type this resolver handles.
	Type() reflect.Type
	// Resolve returns the parameter value (called once at startup).
	Resolve() (reflect.Value, error)
}
