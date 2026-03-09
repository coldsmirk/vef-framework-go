package cqrs

import (
	"context"
	"fmt"
	"reflect"

	"github.com/coldsmirk/go-collections"
	"github.com/samber/lo"
)

// Dispatcher is the type-erased interface used for map lookup.
type Dispatcher interface {
	// dispatch executes the underlying typed handler using a type-erased action value.
	dispatch(ctx context.Context, cmd any) (any, error)
}

// TypedHandler wraps a generic Handler and provides a type-erased dispatch method.
type TypedHandler[TAction Action, TResult any] struct {
	handler Handler[TAction, TResult]
}

func (h *TypedHandler[TAction, TResult]) dispatch(ctx context.Context, action any) (any, error) {
	return h.handler.Handle(ctx, action.(TAction))
}

// CommandQueryBus is the concrete Bus implementation.
type CommandQueryBus struct {
	handlers  collections.ConcurrentMap[reflect.Type, Dispatcher]
	behaviors []Behavior
}

// NewBus creates a new Bus with the given behavior middlewares.
func NewBus(behaviors []Behavior) Bus {
	return &CommandQueryBus{
		handlers:  collections.NewConcurrentHashMap[reflect.Type, Dispatcher](),
		behaviors: behaviors,
	}
}

func (b *CommandQueryBus) register(key reflect.Type, d Dispatcher) {
	if _, inserted := b.handlers.PutIfAbsent(key, d); !inserted {
		panic(fmt.Sprintf("cqrs: handler already registered for %s", key))
	}
}

func (b *CommandQueryBus) send(ctx context.Context, key reflect.Type, action Action) (any, error) {
	h, ok := b.handlers.Get(key)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotFound, key)
	}

	// Wrap behaviors in reverse order so behaviors[0] is the outermost.
	execute := func(ctx context.Context) (any, error) {
		return h.dispatch(ctx, action)
	}
	for i := len(b.behaviors) - 1; i >= 0; i-- {
		bh := b.behaviors[i]
		next := execute
		execute = func(ctx context.Context) (any, error) {
			return bh.Handle(ctx, action, next)
		}
	}

	return execute(ctx)
}

// Register registers a type-safe handler for command type C.
// Panics if a handler is already registered for the same command type.
func Register[TAction Action, TResult any](bus Bus, handler Handler[TAction, TResult]) {
	bus.register(reflect.TypeFor[TAction](), &TypedHandler[TAction, TResult]{handler: handler})
}

// Send dispatches a command through the behavior pipeline to its registered handler.
func Send[TAction Action, TResult any](ctx context.Context, bus Bus, action TAction) (TResult, error) {
	raw, err := bus.send(ctx, reflect.TypeFor[TAction](), action)
	if err != nil {
		return lo.Empty[TResult](), err
	}

	if raw == nil {
		return lo.Empty[TResult](), nil
	}

	return raw.(TResult), nil
}
