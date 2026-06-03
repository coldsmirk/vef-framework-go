package approval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// StubRouteInspector lets verifyEventRouting exercise OnStart against a
// deterministic routing table without spinning up the real bus.
//
// subscribable defaults to "every event is subscribable" via the nil
// map fallback in HasSubscribableTransport, so existing scenarios that
// only care about the transactional axis stay terse. Scenarios that
// exercise the subscribable check set an explicit map.
type StubRouteInspector struct {
	transactional map[string]bool
	subscribable  map[string]bool
}

func (s *StubRouteInspector) HasTransactionalRoute(et string) bool {
	return s.transactional[et]
}

func (s *StubRouteInspector) HasSubscribableTransport(et string) bool {
	if s.subscribable == nil {
		// Treat every event as subscribable when the field is unset so
		// callers that only care about HasTransactionalRoute don't have
		// to enumerate the full event list twice.
		return true
	}

	return s.subscribable[et]
}

func allRequiredTransactional() map[string]bool {
	m := make(map[string]bool, len(transactionalEventTypes))
	for _, et := range transactionalEventTypes {
		m[et] = true
	}

	return m
}

func TestVerifyEventRouting(t *testing.T) {
	t.Run("PassesWhenAllRequiredEventsHaveTransactionalRoute", func(t *testing.T) {
		inspector := &StubRouteInspector{transactional: allRequiredTransactional()}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()), "All required events should have transactional routes")

		lc.RequireStop()
	})

	t.Run("FailsWhenTaskCreatedMissesTransactionalRoute", func(t *testing.T) {
		ts := allRequiredTransactional()
		delete(ts, approval.EventTypeTaskCreated)
		inspector := &StubRouteInspector{transactional: ts}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		err := lc.Start(context.Background())
		require.Error(t, err, "Missing transactional route should return an error")
		assert.ErrorIs(t, err, ErrEventRouteNotTransactional, "Error should wrap ErrEventRouteNotTransactional")
		assert.Contains(t, err.Error(), approval.EventTypeTaskCreated, "Error should name the missing event type")
		assert.Contains(t, err.Error(), "outbox", "Error should guide operators toward outbox configuration")
		assert.Contains(t, err.Error(), "[\"outbox\", \"memory\"]",
			"Error should include a subscribable sink and not only publish-only outbox")
	})

	t.Run("FailsOnFirstMissingEventInDeclaredOrder", func(t *testing.T) {
		// An empty table should report the first required event type.
		inspector := &StubRouteInspector{}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		err := lc.Start(context.Background())
		require.Error(t, err, "Missing all transactional routes should return an error")
		assert.ErrorIs(t, err, ErrEventRouteNotTransactional, "Error should wrap ErrEventRouteNotTransactional")
		assert.Contains(t, err.Error(), approval.EventTypeInstanceCreated,
			"Error should report the first missing transactional route")
	})

	t.Run("DoesNotRequireBindingFailedTxRoute", func(t *testing.T) {
		// binding_failed is emitted by the async listener and should not
		// be part of the approval business-event transaction route check.
		ts := allRequiredTransactional()
		_, exists := ts[approval.EventTypeInstanceBindingFailed]
		require.False(t, exists, "Binding failed event should not be in the required transaction-route list")

		inspector := &StubRouteInspector{transactional: ts}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()),
			"Missing binding_failed transactional route should not fail module startup")

		lc.RequireStop()
	})

	t.Run("FailsWhenInstanceCompletedHasNoSubscribableTransport", func(t *testing.T) {
		// Mirror the production misconfiguration: route resolves to a
		// publish-only outbox alone. HasTransactionalRoute=true on all
		// events, HasSubscribableTransport=false on InstanceCompleted.
		inspector := &StubRouteInspector{
			transactional: allRequiredTransactional(),
			subscribable: map[string]bool{
				// Every other event remains subscribable so the test
				// pinpoints InstanceCompleted as the unique offender.
				approval.EventTypeInstanceCompleted: false,
			},
		}
		// Mark all other events as subscribable explicitly.
		for et := range allRequiredTransactional() {
			if et == approval.EventTypeInstanceCompleted {
				continue
			}

			inspector.subscribable[et] = true
		}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		err := lc.Start(context.Background())
		require.Error(t, err, "Binding listener cannot subscribe to a publish-only route")
		assert.ErrorIs(t, err, ErrEventRouteNotSubscribable,
			"Error should wrap ErrEventRouteNotSubscribable")
		assert.Contains(t, err.Error(), approval.EventTypeInstanceCompleted,
			"Error should name the offending event type")
		assert.Contains(t, err.Error(), "binding listener",
			"Error should explain why the route needs a sink")
	})

	t.Run("PassesWhenSubscribableRouteIsAvailable", func(t *testing.T) {
		// Default stub treats every event as subscribable. Same result
		// as a route containing [\"outbox\", \"memory\"].
		inspector := &StubRouteInspector{transactional: allRequiredTransactional()}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()),
			"Subscribable transport on InstanceCompleted should let the module start")

		lc.RequireStop()
	})
}
