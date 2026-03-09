package behavior

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

type TestCmd struct{ cqrs.BaseCommand }

type TestQuery struct{ cqrs.BaseQuery }

func TestTransactionBehavior(t *testing.T) {
	db := testx.NewTestDB(t)
	behavior := NewTransactionBehavior(db)

	t.Run("WrapsCommandInTransaction", func(t *testing.T) {
		result, err := behavior.Handle(context.Background(), TestCmd{}, func(ctx context.Context) (any, error) {
			tx := contextx.DB(ctx)
			assert.NotNil(t, tx, "Should inject tx DB into context")
			assert.NotEqual(t, db, tx, "Should be transaction DB, not original DB")

			return "ok", nil
		})

		require.NoError(t, err, "Should not return error")
		assert.Equal(t, "ok", result, "Should return handler result")
	})

	t.Run("BypassesTransactionForQuery", func(t *testing.T) {
		called := false

		result, err := behavior.Handle(context.Background(), TestQuery{}, func(ctx context.Context) (any, error) {
			called = true

			assert.Nil(t, contextx.DB(ctx), "Should not inject tx DB for queries")

			return "query-result", nil
		})

		require.NoError(t, err, "Should not return error")
		assert.True(t, called, "Should call next handler")
		assert.Equal(t, "query-result", result, "Should return handler result")
	})

	t.Run("PropagatesHandlerError", func(t *testing.T) {
		handlerErr := errors.New("handler failed")

		_, err := behavior.Handle(context.Background(), TestCmd{}, func(context.Context) (any, error) {
			return nil, handlerErr
		})

		require.ErrorIs(t, err, handlerErr, "Should propagate handler error")
	})
}
