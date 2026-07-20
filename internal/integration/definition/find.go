package definition

import (
	"context"
	"errors"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FindOne loads the single definition row of T matched by where, mapping a
// missing row to notFound. Enabled-state handling stays at the call sites —
// what a disabled definition means is per-flow policy.
func FindOne[T any](ctx context.Context, db orm.DB, notFound error, where func(orm.ConditionBuilder)) (*T, error) {
	model := new(T)

	err := db.NewSelect().
		Model(model).
		Where(where).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return nil, notFound
		}

		return nil, err
	}

	return model, nil
}
