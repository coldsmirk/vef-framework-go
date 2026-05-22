package approval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// stubRouteInspector lets verifyEventRouting exercise OnStart against a
// deterministic routing table without spinning up the real bus.
type stubRouteInspector struct {
	transactional map[string]bool
}

func (s *stubRouteInspector) HasTransactionalRoute(et string) bool {
	return s.transactional[et]
}

func allRequiredTransactional() map[string]bool {
	return map[string]bool{
		approval.EventTypeInstanceCreated:     true,
		approval.EventTypeInstanceCompleted:   true,
		approval.EventTypeInstanceWithdrawn:   true,
		approval.EventTypeInstanceRolledBack:  true,
		approval.EventTypeInstanceReturned:    true,
		approval.EventTypeInstanceResubmitted: true,
		approval.EventTypeNodeEntered:         true,
		approval.EventTypeNodeAutoPassed:      true,
		approval.EventTypeTaskCreated:         true,
		approval.EventTypeTaskApproved:        true,
		approval.EventTypeTaskHandled:         true,
		approval.EventTypeTaskRejected:        true,
		approval.EventTypeTaskTransferred:     true,
		approval.EventTypeTaskReassigned:      true,
		approval.EventTypeTaskTimedOut:        true,
		approval.EventTypeAssigneesAdded:      true,
		approval.EventTypeAssigneesRemoved:    true,
		approval.EventTypeTaskDeadlineWarning: true,
		approval.EventTypeTaskUrged:           true,
		approval.EventTypeCCNotified:          true,
		approval.EventTypeFlowCreated:         true,
		approval.EventTypeFlowUpdated:         true,
		approval.EventTypeFlowDeployed:        true,
		approval.EventTypeFlowToggled:         true,
		approval.EventTypeFlowPublished:       true,
	}
}

func TestVerifyEventRouting(t *testing.T) {
	t.Run("PassesWhenAllRequiredEventsHaveTransactionalRoute", func(t *testing.T) {
		inspector := &stubRouteInspector{transactional: allRequiredTransactional()}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()), "All required events should have transactional routes")

		lc.RequireStop()
	})

	t.Run("FailsWhenTaskCreatedMissesTransactionalRoute", func(t *testing.T) {
		ts := allRequiredTransactional()
		delete(ts, approval.EventTypeTaskCreated)
		inspector := &stubRouteInspector{transactional: ts}

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
		inspector := &stubRouteInspector{}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		err := lc.Start(context.Background())
		require.Error(t, err, "Missing all transactional routes should return an error")
		assert.ErrorIs(t, err, ErrEventRouteNotTransactional)
		assert.Contains(t, err.Error(), approval.EventTypeInstanceCreated,
			"Error should report the first missing transactional route")
	})

	t.Run("DoesNotRequireBindingFailedTxRoute", func(t *testing.T) {
		// binding_failed is emitted by the async listener and should not
		// be part of the approval business-event transaction route check.
		ts := allRequiredTransactional()
		_, exists := ts[approval.EventTypeInstanceBindingFailed]
		require.False(t, exists, "binding_failed should not be in the required transaction-route list")

		inspector := &stubRouteInspector{transactional: ts}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()),
			"Missing binding_failed transactional route should not fail module startup")

		lc.RequireStop()
	})
}
