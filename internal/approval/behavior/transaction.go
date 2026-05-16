package behavior

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// TransactionBehavior wraps command handlers in a database transaction.
// Query handlers bypass the transaction.
type TransactionBehavior struct {
	db orm.DB
}

// NewTransactionBehavior creates a new TransactionBehavior.
func NewTransactionBehavior(db orm.DB) cqrs.Behavior {
	return &TransactionBehavior{db: db}
}

// Handle wraps command actions in a database transaction. Query actions pass
// through unchanged. If a parent transaction is already attached to ctx
// (e.g. when a Saga or event subscriber re-dispatches a command from within
// an existing transaction), the inner pipeline reuses that transaction
// rather than opening a nested one — concurrent nested transactions on the
// same connection are driver-specific and the runtime cost of savepoints
// outweighs the rare benefit here.
func (b *TransactionBehavior) Handle(ctx context.Context, action cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	if action.Kind() == cqrs.Query {
		return next(ctx)
	}

	if existing := contextx.DB(ctx); existing != nil {
		return next(ctx)
	}

	var result any

	err := b.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) (err error) {
		ctx = contextx.SetDB(ctx, tx)
		result, err = next(ctx)

		return err
	})

	return result, err
}
