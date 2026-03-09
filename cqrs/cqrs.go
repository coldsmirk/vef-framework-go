package cqrs

import (
	"context"

	icqrs "github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

type (
	ActionKind = icqrs.ActionKind
	Action     = icqrs.Action

	BaseCommand = icqrs.BaseCommand
	BaseQuery   = icqrs.BaseQuery

	Unit = icqrs.Unit
	Bus  = icqrs.Bus

	Handler[TAction icqrs.Action, TResult any]     = icqrs.Handler[TAction, TResult]
	HandlerFunc[TAction icqrs.Action, TResult any] = icqrs.HandlerFunc[TAction, TResult]

	Behavior     = icqrs.Behavior
	BehaviorFunc = icqrs.BehaviorFunc
)

const (
	Command = icqrs.Command
	Query   = icqrs.Query
)

var ErrHandlerNotFound = icqrs.ErrHandlerNotFound

// Register registers a type-safe handler for command type C.
// Panics if a handler is already registered for the same command type.
func Register[TAction icqrs.Action, TResult any](bus Bus, handler Handler[TAction, TResult]) {
	icqrs.Register(bus, handler)
}

// Send dispatches a command through the behavior pipeline to its registered handler.
func Send[TAction icqrs.Action, TResult any](ctx context.Context, bus Bus, action TAction) (TResult, error) {
	return icqrs.Send[TAction, TResult](ctx, bus, action)
}
