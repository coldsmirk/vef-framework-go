package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	publicstorage "github.com/coldsmirk/vef-framework-go/storage"
)

type StubRouteInspector struct {
	transactional map[string]bool
}

func (s *StubRouteInspector) HasTransactionalRoute(et string) bool {
	return s.transactional[et]
}

func (*StubRouteInspector) HasSubscribableTransport(string) bool {
	return true
}

func allStorageTransactionalRoutes() map[string]bool {
	return map[string]bool{
		publicstorage.EventTypeFileClaimed:      true,
		publicstorage.EventTypeFileDeleted:      true,
		publicstorage.EventTypeDeleteDeadLetter: true,
	}
}

func TestVerifyEventRouting(t *testing.T) {
	t.Run("PassesWhenAllRequiredEventsHaveTransactionalRoute", func(t *testing.T) {
		inspector := &StubRouteInspector{transactional: allStorageTransactionalRoutes()}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()), "All storage events should have transactional routes")

		lc.RequireStop()
	})

	t.Run("FailsWhenFileDeletedMissesTransactionalRoute", func(t *testing.T) {
		routes := allStorageTransactionalRoutes()
		delete(routes, publicstorage.EventTypeFileDeleted)
		inspector := &StubRouteInspector{transactional: routes}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		err := lc.Start(context.Background())
		require.Error(t, err, "Missing transactional route should return an error")
		assert.ErrorIs(t, err, ErrEventRouteNotTransactional, "Error should wrap ErrEventRouteNotTransactional")
		assert.Contains(t, err.Error(), publicstorage.EventTypeFileDeleted, "Error should name the missing event type")
		assert.Contains(t, err.Error(), "vef.storage.*", "Error should guide operators toward storage routing")
	})
}
