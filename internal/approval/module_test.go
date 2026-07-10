package approval

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// StubRouteInspector lets verifyEventRouting exercise OnStart against a
// deterministic routing table without spinning up the real bus.
type StubRouteInspector struct {
	transactional map[string]bool
}

func (s *StubRouteInspector) HasTransactionalRoute(et string) bool {
	return s.transactional[et]
}

func (*StubRouteInspector) HasSubscribableTransport(string) bool { return false }

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
		assert.Contains(t, err.Error(), "[\"outbox\"]",
			"Error should show the minimal transactional route")
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
		// binding_failed is emitted by the projection worker and should not
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

	t.Run("DoesNotRequireSubscribableRoute", func(t *testing.T) {
		// Business projection no longer consumes lifecycle events. A
		// publish-only transactional outbox route is sufficient for approval
		// itself; host subscribers choose their own sink requirements.
		inspector := &StubRouteInspector{transactional: allRequiredTransactional()}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()),
			"Missing subscribable transports should not block projection-independent event routing")

		lc.RequireStop()
	})
}

// TestTransactionalEventTypesCoverAllEvents is the drift guard: it asserts the
// transactional route-check list is exactly the canonical event set minus the
// reviewed non-transactional exclusions. A new approval event constant that is
// added to approval.AllEventTypes() but neither marked transactional nor
// explicitly excluded fails here, instead of silently bypassing the fail-fast
// startup routing check and failing at runtime with event.ErrTxRequired.
func TestTransactionalEventTypesCoverAllEvents(t *testing.T) {
	t.Run("TransactionalListEqualsAllEventsMinusExclusions", func(t *testing.T) {
		var expected []string

		for _, et := range approval.AllEventTypes() {
			if _, excluded := nonTransactionalEventTypes[et]; !excluded {
				expected = append(expected, et)
			}
		}

		got := slices.Clone(transactionalEventTypes)

		slices.Sort(expected)
		slices.Sort(got)

		assert.Equal(t, expected, got,
			"transactionalEventTypes must equal approval.AllEventTypes() minus nonTransactionalEventTypes; "+
				"a new event constant must be added to the transactional set or the documented exclusion set")
	})

	t.Run("ExcludedEventsAreRealEventTypes", func(t *testing.T) {
		all := approval.AllEventTypes()
		for et := range nonTransactionalEventTypes {
			assert.True(t, slices.Contains(all, et),
				"excluded event %q must be a member of approval.AllEventTypes()", et)
		}
	})

	t.Run("BindingFailedIsExcluded", func(t *testing.T) {
		_, excluded := nonTransactionalEventTypes[approval.EventTypeInstanceBindingFailed]
		assert.True(t, excluded,
			"binding_failed is emitted outside a business transaction and must remain excluded")
		assert.False(t, slices.Contains(transactionalEventTypes, approval.EventTypeInstanceBindingFailed),
			"binding_failed must not be in the transactional route-check list")
	})
}
