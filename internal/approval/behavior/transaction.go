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

// Order places TransactionBehavior at the outermost slot so every
// inner behavior (ActionLog / EventPublish) sees the same tx in context.
func (*TransactionBehavior) Order() int { return 0 }

// Handle wraps command actions in a database transaction. Query actions pass
// through unchanged. If a parent transaction is already attached to ctx
// (e.g. when an event subscriber re-dispatches a command from within an
// existing transaction), the inner pipeline reuses that transaction
// rather than opening a nested one — concurrent nested transactions on the
// same connection are driver-specific and the runtime cost of savepoints
// outweighs the rare benefit here. Only an actual transaction handle counts
// as a parent: the API middleware attaches a plain request-scoped DB to
// every request context, and mistaking it for a parent transaction would
// silently run the whole command pipeline without atomicity. That
// request-scoped handle is still preferred as the transaction opener — it
// carries the operator named arg that audit columns render.
func (b *TransactionBehavior) Handle(ctx context.Context, action cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	if action.Kind() == cqrs.Query {
		return next(ctx)
	}

	db := b.db

	if existing := contextx.DB(ctx); existing != nil {
		if existing.InTx() {
			return next(ctx)
		}

		db = existing
	}

	var result any

	err := db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) (err error) {
		ctx = contextx.SetDB(ctx, tx)
		result, err = next(ctx)

		return err
	})

	return result, err
}
