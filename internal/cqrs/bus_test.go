package cqrs

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type CreateUserCmd struct {
	BaseCommand

	Name string
}

type CreateUserResult struct {
	ID string
}

type DeleteUserCmd struct {
	BaseCommand

	ID string
}

type GetUserQuery struct {
	BaseQuery

	ID string
}

type GetUserResult struct {
	Name string
}

func TestRegisterAndSend(t *testing.T) {
	t.Run("SingleHandler", func(t *testing.T) {
		bus := NewBus(nil)
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: "u_" + cmd.Name}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "alice"})

		require.NoError(t, err, "Should send command without error")
		assert.Equal(t, "u_alice", got.ID, "Should return handler result with prefixed name")
	})

	t.Run("UnitCommand", func(t *testing.T) {
		bus := NewBus(nil)

		var called bool
		Register(bus, HandlerFunc[DeleteUserCmd, Unit](func(context.Context, DeleteUserCmd) (Unit, error) {
			called = true

			return Unit{}, nil
		}))

		got, err := Send[DeleteUserCmd, Unit](context.Background(), bus, DeleteUserCmd{ID: "123"})

		require.NoError(t, err, "Should send unit command without error")
		assert.Equal(t, Unit{}, got, "Should return empty Unit value")
		assert.True(t, called, "Handler should have been invoked")
	})

	t.Run("MultipleHandlers", func(t *testing.T) {
		bus := NewBus(nil)
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: cmd.Name}, nil
		}))
		Register(bus, HandlerFunc[GetUserQuery, GetUserResult](func(_ context.Context, q GetUserQuery) (GetUserResult, error) {
			return GetUserResult{Name: "found_" + q.ID}, nil
		}))

		r1, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "bob"})
		require.NoError(t, err, "Should send create command without error")
		assert.Equal(t, "bob", r1.ID, "Should return create result with correct ID")

		r2, err := Send[GetUserQuery, GetUserResult](context.Background(), bus, GetUserQuery{ID: "42"})
		require.NoError(t, err, "Should send query without error")
		assert.Equal(t, "found_42", r2.Name, "Should return query result with correct name")
	})

	t.Run("HandlerReturnsError", func(t *testing.T) {
		bus := NewBus(nil)
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(context.Context, CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{}, errors.New("db error")
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "fail"})

		require.EqualError(t, err, "db error", "Should propagate handler error")
		assert.Empty(t, got.ID, "Should return zero value result on error")
	})

	t.Run("ZeroValueCommand", func(t *testing.T) {
		bus := NewBus(nil)
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: cmd.Name}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{})

		require.NoError(t, err, "Should handle zero value command without error")
		assert.Empty(t, got.ID, "Should return empty ID for zero value name")
	})

	t.Run("NilBehaviors", func(t *testing.T) {
		bus := NewBus(nil)
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: cmd.Name}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "test"})

		require.NoError(t, err, "Should send without error when behaviors is nil")
		assert.Equal(t, "test", got.ID, "Should return correct result with nil behaviors")
	})

	t.Run("EmptyBehaviors", func(t *testing.T) {
		bus := NewBus([]Behavior{})
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: cmd.Name}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "test"})

		require.NoError(t, err, "Should send without error when behaviors is empty")
		assert.Equal(t, "test", got.ID, "Should return correct result with empty behaviors")
	})
}

func TestSendUnregisteredCommand(t *testing.T) {
	bus := NewBus(nil)

	_, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "x"})

	require.ErrorIs(t, err, ErrHandlerNotFound, "Should return ErrHandlerNotFound for unregistered command")
	assert.Contains(t, err.Error(), "CreateUserCmd", "Error should include the command type name")
}

func TestRegisterDuplicatePanics(t *testing.T) {
	bus := NewBus(nil)
	Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(context.Context, CreateUserCmd) (CreateUserResult, error) {
		return CreateUserResult{}, nil
	}))

	assert.PanicsWithValue(t,
		"cqrs: handler already registered for cqrs.CreateUserCmd",
		func() {
			Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(context.Context, CreateUserCmd) (CreateUserResult, error) {
				return CreateUserResult{}, nil
			}))
		},
		"Should panic when registering duplicate handler for the same command type",
	)
}

func TestBehaviorPipeline(t *testing.T) {
	t.Run("SingleBehavior", func(t *testing.T) {
		var order []string

		b := BehaviorFunc(func(ctx context.Context, _ Action, next func(context.Context) (any, error)) (any, error) {
			order = append(order, "before")
			res, err := next(ctx)

			order = append(order, "after")

			return res, err
		})

		bus := NewBus([]Behavior{b})
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
			order = append(order, "handler")

			return CreateUserResult{ID: cmd.Name}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "test"})

		require.NoError(t, err, "Should send through behavior pipeline without error")
		assert.Equal(t, "test", got.ID, "Should return handler result")
		assert.Equal(t, []string{"before", "handler", "after"}, order, "Should execute behavior before and after handler")
	})

	t.Run("MultipleBehaviorsOrder", func(t *testing.T) {
		var order []string

		makeBehavior := func(name string) Behavior {
			return BehaviorFunc(func(ctx context.Context, _ Action, next func(context.Context) (any, error)) (any, error) {
				order = append(order, name+"-before")
				res, err := next(ctx)

				order = append(order, name+"-after")

				return res, err
			})
		}

		bus := NewBus([]Behavior{makeBehavior("b1"), makeBehavior("b2")})
		Register(bus, HandlerFunc[CreateUserCmd, Unit](func(context.Context, CreateUserCmd) (Unit, error) {
			order = append(order, "handler")

			return Unit{}, nil
		}))

		_, err := Send[CreateUserCmd, Unit](context.Background(), bus, CreateUserCmd{})

		require.NoError(t, err, "Should send through multiple behaviors without error")
		assert.Equal(t, []string{"b1-before", "b2-before", "handler", "b2-after", "b1-after"}, order,
			"Should execute behaviors as nested middleware in registration order")
	})

	t.Run("ShortCircuit", func(t *testing.T) {
		shortCircuit := BehaviorFunc(func(context.Context, Action, func(context.Context) (any, error)) (any, error) {
			return CreateUserResult{ID: "short-circuited"}, nil
		})

		var handlerCalled bool

		bus := NewBus([]Behavior{shortCircuit})
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(context.Context, CreateUserCmd) (CreateUserResult, error) {
			handlerCalled = true

			return CreateUserResult{}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{})

		require.NoError(t, err, "Should short-circuit without error")
		assert.Equal(t, "short-circuited", got.ID, "Should return behavior's short-circuit result")
		assert.False(t, handlerCalled, "Handler should not be called when behavior short-circuits")
	})

	t.Run("ModifyContext", func(t *testing.T) {
		type ctxKey struct{}

		b := BehaviorFunc(func(ctx context.Context, _ Action, next func(context.Context) (any, error)) (any, error) {
			return next(context.WithValue(ctx, ctxKey{}, "injected"))
		})

		bus := NewBus([]Behavior{b})
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(ctx context.Context, _ CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: ctx.Value(ctxKey{}).(string)}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{})

		require.NoError(t, err, "Should send with modified context without error")
		assert.Equal(t, "injected", got.ID, "Handler should receive context value injected by behavior")
	})

	t.Run("BehaviorReturnsError", func(t *testing.T) {
		b := BehaviorFunc(func(context.Context, Action, func(context.Context) (any, error)) (any, error) {
			return nil, errors.New("behavior error")
		})

		bus := NewBus([]Behavior{b})
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(context.Context, CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: "should-not-reach"}, nil
		}))

		_, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{})

		require.EqualError(t, err, "behavior error", "Should propagate behavior error")
	})

	t.Run("BehaviorReceivesCommand", func(t *testing.T) {
		var receivedCmd any

		b := BehaviorFunc(func(ctx context.Context, action Action, next func(context.Context) (any, error)) (any, error) {
			receivedCmd = action

			return next(ctx)
		})

		bus := NewBus([]Behavior{b})
		Register(bus, HandlerFunc[CreateUserCmd, Unit](func(context.Context, CreateUserCmd) (Unit, error) {
			return Unit{}, nil
		}))

		sent := CreateUserCmd{Name: "inspect"}
		_, err := Send[CreateUserCmd, Unit](context.Background(), bus, sent)

		require.NoError(t, err, "Should send without error")
		assert.Equal(t, sent, receivedCmd, "Behavior should receive the original command")
	})

	t.Run("BehaviorReturnsNil", func(t *testing.T) {
		b := BehaviorFunc(func(context.Context, Action, func(context.Context) (any, error)) (any, error) {
			return nil, nil
		})

		bus := NewBus([]Behavior{b})
		Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(context.Context, CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: "unreachable"}, nil
		}))

		got, err := Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{})

		require.NoError(t, err, "Should not return error when behavior returns nil")
		assert.Empty(t, got.ID, "Should return zero value when behavior returns nil")
	})
}

func TestConcurrentSend(t *testing.T) {
	bus := NewBus(nil)
	Register(bus, HandlerFunc[CreateUserCmd, CreateUserResult](func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
		return CreateUserResult{ID: cmd.Name}, nil
	}))

	const n = 100

	var wg sync.WaitGroup

	errs := make([]error, n)
	results := make([]CreateUserResult, n)

	for i := range n {
		wg.Go(func() {
			results[i], errs[i] = Send[CreateUserCmd, CreateUserResult](context.Background(), bus, CreateUserCmd{Name: "user"})
		})
	}

	wg.Wait()

	for i := range n {
		require.NoError(t, errs[i], "Concurrent send should not return error")
		assert.Equal(t, "user", results[i].ID, "Concurrent send should return correct result")
	}
}

func TestHandlerFunc(t *testing.T) {
	var h Handler[CreateUserCmd, CreateUserResult] = HandlerFunc[CreateUserCmd, CreateUserResult](
		func(_ context.Context, cmd CreateUserCmd) (CreateUserResult, error) {
			return CreateUserResult{ID: cmd.Name}, nil
		},
	)

	got, err := h.Handle(context.Background(), CreateUserCmd{Name: "test"})

	require.NoError(t, err, "HandlerFunc should handle command without error")
	assert.Equal(t, "test", got.ID, "HandlerFunc should return expected result")
}

func TestBehaviorFunc(t *testing.T) {
	var b Behavior = BehaviorFunc(func(ctx context.Context, _ Action, next func(context.Context) (any, error)) (any, error) {
		return next(ctx)
	})

	got, err := b.Handle(context.Background(), CreateUserCmd{}, func(context.Context) (any, error) {
		return "ok", nil
	})

	require.NoError(t, err, "BehaviorFunc should pass through without error")
	assert.Equal(t, "ok", got, "BehaviorFunc should return next handler's result")
}

func TestActionKind(t *testing.T) {
	t.Run("BaseCommand", func(t *testing.T) {
		cmd := CreateUserCmd{Name: "test"}
		assert.Equal(t, Command, cmd.Kind(), "BaseCommand should return Command kind")
	})

	t.Run("BaseQuery", func(t *testing.T) {
		q := GetUserQuery{ID: "1"}
		assert.Equal(t, Query, q.Kind(), "BaseQuery should return Query kind")
	})

	t.Run("BehaviorReceivesActionKind", func(t *testing.T) {
		var receivedKind ActionKind

		b := BehaviorFunc(func(ctx context.Context, action Action, next func(context.Context) (any, error)) (any, error) {
			receivedKind = action.Kind()

			return next(ctx)
		})

		bus := NewBus([]Behavior{b})
		Register(bus, HandlerFunc[CreateUserCmd, Unit](func(context.Context, CreateUserCmd) (Unit, error) {
			return Unit{}, nil
		}))

		_, err := Send[CreateUserCmd, Unit](context.Background(), bus, CreateUserCmd{})
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, Command, receivedKind, "Behavior should receive Command kind for command type")
	})
}
