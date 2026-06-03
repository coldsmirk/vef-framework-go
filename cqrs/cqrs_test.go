package cqrs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cqrs"
)

type CreateUserCmd struct {
	cqrs.BaseCommand

	Name string
}

type CreateUserResult struct {
	ID string
}

type GetUserResult struct {
	Name string
}

// OrderedBehavior asserts the public Ordered alias is usable as a compile-time
// conformance target by external behavior authors.
type OrderedBehavior struct{}

func (*OrderedBehavior) Handle(ctx context.Context, _ cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	return next(ctx)
}

func (*OrderedBehavior) Order() int { return 1000 }

var (
	_ cqrs.Behavior = (*OrderedBehavior)(nil)
	_ cqrs.Ordered  = (*OrderedBehavior)(nil)
)

func TestPublicFacade(t *testing.T) {
	t.Run("RegisterAndSend", func(t *testing.T) {
		bus := cqrs.NewBus(nil)
		cqrs.Register(bus, cqrs.HandlerFunc[CreateUserCmd, CreateUserResult](
			func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
				return CreateUserResult{ID: "u_" + cmd.Name}, nil
			},
		))

		got, err := cqrs.Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "alice"})

		require.NoError(t, err, "Public Send should dispatch the registered handler without error")
		assert.Equal(t, "u_alice", got.ID, "Public Send should return the handler result through the facade")
	})

	t.Run("HandlerNotFoundIdentity", func(t *testing.T) {
		bus := cqrs.NewBus(nil)

		_, err := cqrs.Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "x"})

		require.Error(t, err, "Public Send should error when no handler is registered")
		assert.True(t, errors.Is(err, cqrs.ErrHandlerNotFound),
			"errors.Is should match cqrs.ErrHandlerNotFound across the public alias")
	})

	t.Run("ResultTypeMismatchIdentity", func(t *testing.T) {
		bus := cqrs.NewBus([]cqrs.Behavior{
			cqrs.BehaviorFunc(func(context.Context, cqrs.Action, func(context.Context) (any, error)) (any, error) {
				return GetUserResult{Name: "wrong"}, nil
			}),
		})
		cqrs.Register(bus, cqrs.HandlerFunc[CreateUserCmd, CreateUserResult](
			func(context.Context, CreateUserCmd) (CreateUserResult, error) {
				return CreateUserResult{}, nil
			},
		))

		_, err := cqrs.Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{})

		require.Error(t, err, "Public Send should error on a wrong-typed short-circuit value")
		assert.True(t, errors.Is(err, cqrs.ErrResultTypeMismatch),
			"errors.Is should match cqrs.ErrResultTypeMismatch across the public alias")
	})

	t.Run("OrderedBehaviorThroughFacade", func(t *testing.T) {
		bus := cqrs.NewBus([]cqrs.Behavior{new(OrderedBehavior)})
		cqrs.Register(bus, cqrs.HandlerFunc[CreateUserCmd, CreateUserResult](
			func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
				return CreateUserResult{ID: cmd.Name}, nil
			},
		))

		got, err := cqrs.Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "bob"})

		require.NoError(t, err, "Public Ordered behavior should thread the pipeline without error")
		assert.Equal(t, "bob", got.ID, "Public Ordered behavior should return the handler result")
	})
}
