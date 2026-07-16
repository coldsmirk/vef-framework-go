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
	"github.com/coldsmirk/vef-framework-go/orm"
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

		require.NoError(t, err, "TestTransactionBehavior should complete without error")
		assert.Equal(t, "ok", result, "Should return handler result")
	})

	t.Run("WrapsCommandDespiteRequestScopedDB", func(t *testing.T) {
		// The API middleware attaches a plain (non-transactional) DB to every
		// request context; it must not be mistaken for a parent transaction.
		requestDB := db.WithNamedArg("Operator", "user-1")
		ctx := contextx.SetDB(context.Background(), requestDB)

		_, err := behavior.Handle(ctx, TestCmd{}, func(ctx context.Context) (any, error) {
			tx := contextx.DB(ctx)
			require.NotNil(t, tx, "Should inject tx DB into context")
			assert.True(t, tx.InTx(), "Handler must run inside a transaction")

			return nil, nil
		})

		require.NoError(t, err, "Command with request-scoped DB should complete without error")
	})

	t.Run("ReusesParentTransaction", func(t *testing.T) {
		err := db.RunInTx(context.Background(), func(ctx context.Context, tx orm.DB) error {
			ctx = contextx.SetDB(ctx, tx)

			_, err := behavior.Handle(ctx, TestCmd{}, func(ctx context.Context) (any, error) {
				assert.Same(t, tx, contextx.DB(ctx), "Should reuse the parent transaction handle")

				return nil, nil
			})

			return err
		})

		require.NoError(t, err, "Nested dispatch inside a transaction should complete without error")
	})

	t.Run("BypassesTransactionForQuery", func(t *testing.T) {
		called := false

		result, err := behavior.Handle(context.Background(), TestQuery{}, func(ctx context.Context) (any, error) {
			called = true

			assert.Nil(t, contextx.DB(ctx), "Should not inject tx DB for queries")

			return "query-result", nil
		})

		require.NoError(t, err, "TestTransactionBehavior should complete without error")
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
