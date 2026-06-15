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
	Ordered      = icqrs.Ordered
)

const (
	Command = icqrs.Command
	Query   = icqrs.Query
)

var (
	ErrHandlerNotFound    = icqrs.ErrHandlerNotFound
	ErrResultTypeMismatch = icqrs.ErrResultTypeMismatch
)

// NewBus delegates to the internal Bus constructor; see internal/cqrs.NewBus
// for the behavior-ordering contract. Hosts normally obtain the Bus via
// dependency injection; this constructor exists for standalone use and tests.
func NewBus(behaviors []Behavior) Bus {
	return icqrs.NewBus(behaviors)
}

// Register delegates to internal/cqrs.Register; see that function for the
// handler-registration contract.
func Register[TAction icqrs.Action, TResult any](bus Bus, handler Handler[TAction, TResult]) {
	icqrs.Register(bus, handler)
}

// Send delegates to internal/cqrs.Send; see that function for the dispatch
// contract.
func Send[TAction icqrs.Action, TResult any](ctx context.Context, bus Bus, action TAction) (TResult, error) {
	return icqrs.Send[TAction, TResult](ctx, bus, action)
}
