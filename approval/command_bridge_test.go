package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cqrs"
)

// ShipOrderCmd is the pointer-dispatched bridge test command.
type ShipOrderCmd struct {
	cqrs.BaseCommand

	InstanceID string
}

// CloseOrderCmd is the value-dispatched bridge test command.
type CloseOrderCmd struct {
	cqrs.BaseCommand

	InstanceID string
}

// FindOrderQry exercises the query-rejection guard.
type FindOrderQry struct {
	cqrs.BaseQuery
}

// registerRecorder wires a recording handler for *ShipOrderCmd into a fresh
// CQRS bus and returns the bus plus the captured commands.
func registerRecorder(handlerErr error) (cqrs.Bus, *[]*ShipOrderCmd) {
	bus := cqrs.NewBus(nil)

	var received []*ShipOrderCmd

	cqrs.Register(bus, cqrs.HandlerFunc[*ShipOrderCmd, cqrs.Unit](
		func(_ context.Context, cmd *ShipOrderCmd) (cqrs.Unit, error) {
			received = append(received, cmd)

			return cqrs.Unit{}, handlerErr
		}))

	return bus, &received
}

func shipMapper(evt *InstanceCompletedEvent) (*ShipOrderCmd, bool) {
	return &ShipOrderCmd{InstanceID: evt.InstanceID}, true
}

func TestBindCommand(t *testing.T) {
	t.Run("DerivesGroupFromCommandType", func(t *testing.T) {
		spy := new(SubscribeSpyBus)
		commands, _ := registerRecorder(nil)

		unsubscribe, err := BindCommand(spy, commands, shipMapper)
		require.NoError(t, err, "Binding a named command should succeed")

		defer unsubscribe()

		assert.Equal(t, "vef:cmd:approval.ShipOrderCmd", spy.group,
			"Group must derive from the command type, not the mapper's identity")
	})

	t.Run("ValueCommandTypeDerivesSameShape", func(t *testing.T) {
		spy := new(SubscribeSpyBus)

		unsubscribe, err := BindCommand(spy, cqrs.NewBus(nil),
			func(*InstanceCompletedEvent) (CloseOrderCmd, bool) { return CloseOrderCmd{}, false })
		require.NoError(t, err, "A value command type should bind")

		defer unsubscribe()

		assert.Equal(t, "vef:cmd:approval.CloseOrderCmd", spy.group,
			"Value and pointer command types must derive the same group shape")
	})

	t.Run("DispatchesMappedCommand", func(t *testing.T) {
		spy := new(SubscribeSpyBus)
		commands, received := registerRecorder(nil)

		unsubscribe, err := BindCommand(spy, commands, shipMapper)
		require.NoError(t, err, "Binding should succeed")

		defer unsubscribe()

		evt := completedEvent("order", "t1")
		evt.InstanceID = "inst-1"

		require.NoError(t, spy.emit(t, evt), "Delivery should dispatch without error")
		require.Len(t, *received, 1, "Exactly one command should reach the handler")
		assert.Equal(t, "inst-1", (*received)[0].InstanceID, "Mapper output must arrive unchanged")
	})

	t.Run("DiscardsHandlerResult", func(t *testing.T) {
		spy := new(SubscribeSpyBus)
		commands := cqrs.NewBus(nil)

		cqrs.Register(commands, cqrs.HandlerFunc[*ShipOrderCmd, string](
			func(context.Context, *ShipOrderCmd) (string, error) { return "tracking-no", nil }))

		unsubscribe, err := BindCommand(spy, commands, shipMapper)
		require.NoError(t, err, "Binding should succeed")

		defer unsubscribe()

		assert.NoError(t, spy.emit(t, completedEvent("order", "t1")),
			"A handler returning a non-Unit result must not fail the dispatch")
	})

	t.Run("SkipsWhenMapperDeclines", func(t *testing.T) {
		spy := new(SubscribeSpyBus)
		commands, received := registerRecorder(nil)

		unsubscribe, err := BindCommand(spy, commands,
			func(*InstanceCompletedEvent) (*ShipOrderCmd, bool) { return nil, false })
		require.NoError(t, err, "Binding should succeed")

		defer unsubscribe()

		require.NoError(t, spy.emit(t, completedEvent("order", "t1")),
			"A declined event should acknowledge without error")
		assert.Empty(t, *received, "No command may be dispatched when the mapper declines")
	})

	t.Run("FiltersShortCircuitBeforeMapper", func(t *testing.T) {
		spy := new(SubscribeSpyBus)
		commands, received := registerRecorder(nil)

		mapperCalls := 0
		unsubscribe, err := BindCommand(spy, commands,
			func(evt *InstanceCompletedEvent) (*ShipOrderCmd, bool) {
				mapperCalls++

				return &ShipOrderCmd{InstanceID: evt.InstanceID}, true
			}, ForFlows("leave"))
		require.NoError(t, err, "Binding with filters should succeed")

		defer unsubscribe()

		require.NoError(t, spy.emit(t, completedEvent("order", "t1")),
			"A filtered-out event should acknowledge without error")
		assert.Zero(t, mapperCalls, "Routing filters must run before the mapper")
		assert.Empty(t, *received, "No command may be dispatched for filtered-out events")
	})

	t.Run("PropagatesDispatchError", func(t *testing.T) {
		spy := new(SubscribeSpyBus)
		boom := errors.New("handler failed")
		commands, _ := registerRecorder(boom)

		unsubscribe, err := BindCommand(spy, commands, shipMapper)
		require.NoError(t, err, "Binding should succeed")

		defer unsubscribe()

		assert.ErrorIs(t, spy.emit(t, completedEvent("order", "t1")), boom,
			"Handler failure must propagate so the transport can redeliver")
	})

	t.Run("UnregisteredCommandFailsDispatch", func(t *testing.T) {
		spy := new(SubscribeSpyBus)

		unsubscribe, err := BindCommand(spy, cqrs.NewBus(nil), shipMapper)
		require.NoError(t, err, "Binding does not require an eagerly registered handler")

		defer unsubscribe()

		assert.ErrorIs(t, spy.emit(t, completedEvent("order", "t1")), cqrs.ErrHandlerNotFound,
			"A missing handler must fail loudly instead of dropping the event")
	})

	t.Run("RejectsQueryType", func(t *testing.T) {
		_, err := BindCommand(new(SubscribeSpyBus), cqrs.NewBus(nil),
			func(*InstanceCompletedEvent) (*FindOrderQry, bool) { return nil, false })
		assert.ErrorIs(t, err, ErrNonCommandAction, "Queries have no side effect to bind")
	})

	t.Run("RejectsUnnamedCommandType", func(t *testing.T) {
		_, err := BindCommand(new(SubscribeSpyBus), cqrs.NewBus(nil),
			func(*InstanceCompletedEvent) (*struct{ cqrs.BaseCommand }, bool) { return nil, false })
		assert.ErrorIs(t, err, ErrUnnamedCommandType,
			"An anonymous struct has no identity to derive a group from")
	})

	t.Run("RejectsInterfaceCommandType", func(t *testing.T) {
		_, err := BindCommand[*InstanceCompletedEvent, cqrs.Action](new(SubscribeSpyBus), cqrs.NewBus(nil),
			func(*InstanceCompletedEvent) (cqrs.Action, bool) { return nil, false })
		assert.ErrorIs(t, err, ErrUnnamedCommandType,
			"An interface-typed command has no concrete identity to dispatch on")
	})

	t.Run("DuplicateDefaultGroupConflicts", func(t *testing.T) {
		commands, _ := registerRecorder(nil)

		unsubscribe, err := BindCommand(new(SubscribeSpyBus), commands, shipMapper)
		require.NoError(t, err, "First binding should claim the derived group")

		defer unsubscribe()

		_, err = BindCommand(new(SubscribeSpyBus), commands, shipMapper)
		require.ErrorIs(t, err, ErrDerivedGroupConflict,
			"The same command type must not silently split one derived group")

		second, err := BindCommand(new(SubscribeSpyBus), commands, shipMapper, WithGroup("vef:cmd:ship-orders-2"))
		require.NoError(t, err, "An explicit group should disambiguate a second binding")
		second()
	})

	t.Run("ExplicitGroupSkipsDerivation", func(t *testing.T) {
		spy := new(SubscribeSpyBus)
		commands, _ := registerRecorder(nil)

		unsubscribe, err := BindCommand(spy, commands, shipMapper, WithGroup("vef:cmd:custom"))
		require.NoError(t, err, "Binding with an explicit group should succeed")

		defer unsubscribe()

		assert.Equal(t, "vef:cmd:custom", spy.group, "Explicit group must override derivation")
	})

	t.Run("UnsubscribeReleasesDerivedGroup", func(t *testing.T) {
		commands, _ := registerRecorder(nil)

		unsubscribe, err := BindCommand(new(SubscribeSpyBus), commands, shipMapper)
		require.NoError(t, err, "First binding should succeed")
		unsubscribe()

		rebound, err := BindCommand(new(SubscribeSpyBus), commands, shipMapper)
		require.NoError(t, err, "Unsubscribing must release the derived group for rebinding")
		rebound()
	})

	t.Run("SubscribeFailureReleasesDerivedGroup", func(t *testing.T) {
		commands, _ := registerRecorder(nil)
		failing := &SubscribeSpyBus{subscribeErr: errors.New("transport down")}

		_, err := BindCommand(failing, commands, shipMapper)
		require.Error(t, err, "Subscription failure must surface")

		unsubscribe, err := BindCommand(new(SubscribeSpyBus), commands, shipMapper)
		require.NoError(t, err, "A failed binding must not leak its derived group claim")
		unsubscribe()
	})
}
