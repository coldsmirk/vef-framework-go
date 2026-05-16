package cqrs

import (
	"context"
	"reflect"
)

// Bus is the command/query dispatch bus interface.
type Bus interface {
	// register binds a dispatcher to an action type key.
	register(key reflect.Type, d Dispatcher)
	// send dispatches an action by type key through the behavior pipeline.
	send(ctx context.Context, key reflect.Type, action Action) (any, error)
}

// Action is the base interface for all commands and queries.
type Action interface {
	// Kind returns whether this action is a Command or a Query.
	Kind() ActionKind
}

// Handler is a type-safe command/query handler.
type Handler[TAction Action, TResult any] interface {
	// Handle executes the given command or query and returns the result.
	Handle(ctx context.Context, action TAction) (TResult, error)
}

// Behavior is a Bus middleware that can intercept all commands/queries.
type Behavior interface {
	// Handle intercepts command/query execution; call next to continue the pipeline.
	Handle(ctx context.Context, action Action, next func(ctx context.Context) (any, error)) (any, error)
}

// Ordered is an optional interface for Behavior implementations that need
// deterministic wrapping order. The Bus sorts behaviors by Order ascending
// at construction time, so a behavior with a lower Order wraps a behavior
// with a higher Order (outermost first → innermost last). Behaviors that
// do not implement Ordered are placed at Order 0, in the order Uber FX
// produced them — which is not stable for value groups, so production
// pipelines should implement Ordered.
//
// Conventional bands:
//
//   - 0–99    : transactional / contextual setup (must wrap everything)
//   - 100–199 : audit / collector lifecycle (writes after handler succeeds)
//   - 200–299 : event publish / outbox (last buffered side effect)
//   - 1000+   : custom host behaviors
type Ordered interface {
	// Order returns the sort key for the behavior. Lower values wrap
	// outer; behaviors share an Order at the cost of an unstable relative
	// position between them.
	Order() int
}

// ActionKind distinguishes commands from queries.
type ActionKind int

const (
	Command ActionKind = iota
	Query
)

// BaseCommand is embedded by command types to mark them as commands.
type BaseCommand struct{}

func (BaseCommand) Kind() ActionKind { return Command }

// BaseQuery is embedded by query types to mark them as queries.
type BaseQuery struct{}

func (BaseQuery) Kind() ActionKind { return Query }

// Unit is a placeholder return type for commands that produce no result.
type Unit struct{}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc[TAction Action, TResult any] func(ctx context.Context, action TAction) (TResult, error)

func (f HandlerFunc[TAction, TResult]) Handle(ctx context.Context, action TAction) (TResult, error) {
	return f(ctx, action)
}

// BehaviorFunc is a function adapter for Behavior.
type BehaviorFunc func(ctx context.Context, action Action, next func(ctx context.Context) (any, error)) (any, error)

func (f BehaviorFunc) Handle(ctx context.Context, action Action, next func(ctx context.Context) (any, error)) (any, error) {
	return f(ctx, action, next)
}
