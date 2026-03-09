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

// Handle wraps command actions in a database transaction. Query actions pass through unchanged.
func (b *TransactionBehavior) Handle(ctx context.Context, action cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	if action.Kind() == cqrs.Query {
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
