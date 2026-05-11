package store

import (
	"context"
	"errors"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NewDeleteQueue returns the default bun-backed DeleteQueue implementation.
//
// The Lease implementation assumes a single active worker per process and
// relies on leader election (e.g. via advisory locks) for multi-instance
// deployments. Cross-DB SELECT FOR UPDATE SKIP LOCKED is not used; instead,
// Lease wraps a SELECT + bumping UPDATE in a transaction that the visibility
// timeout (NextAttemptAt = now + leaseDuration) protects against
// double-processing across runs.
func NewDeleteQueue(db orm.DB) storage.DeleteQueue {
	return &bunDeleteQueue{db: db}
}

type bunDeleteQueue struct {
	db orm.DB
}

func (q *bunDeleteQueue) Schedule(ctx context.Context, tx orm.DB, items []storage.PendingDelete) error {
	if len(items) == 0 {
		return nil
	}

	_, err := tx.NewInsert().Model(&items).Exec(ctx)

	return err
}

func (q *bunDeleteQueue) Lease(ctx context.Context, now timex.DateTime, limit int, leaseDuration time.Duration) ([]storage.PendingDelete, error) {
	if limit <= 0 {
		return nil, nil
	}

	var leased []storage.PendingDelete

	err := q.db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		var candidates []storage.PendingDelete

		err := tx.NewSelect().Model(&candidates).Where(func(cb orm.ConditionBuilder) {
			cb.LessThanOrEqual("next_attempt_at", now)
		}).OrderBy("next_attempt_at").Limit(limit).Scan(txCtx)
		if err != nil {
			return err
		}

		if len(candidates) == 0 {
			return nil
		}

		ids := make([]string, len(candidates))
		for i, c := range candidates {
			ids[i] = c.ID
		}

		newDeadline := timex.DateTime(time.Time(now).Add(leaseDuration))

		_, err = tx.NewUpdate().Model((*storage.PendingDelete)(nil)).
			Set("next_attempt_at", newDeadline).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", ids)
			}).
			Exec(txCtx)
		if err != nil {
			return err
		}

		for i := range candidates {
			candidates[i].NextAttemptAt = newDeadline
		}

		leased = candidates

		return nil
	})

	return leased, err
}

func (q *bunDeleteQueue) Done(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	_, err := q.db.NewDelete().Model((*storage.PendingDelete)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.In("id", ids)
	}).Exec(ctx)

	return err
}

func (q *bunDeleteQueue) Defer(ctx context.Context, id string, nextAt timex.DateTime) error {
	res, err := q.db.NewUpdate().Model((*storage.PendingDelete)(nil)).
		Set("next_attempt_at", nextAt).
		SetExpr("attempts", func(eb orm.ExprBuilder) any {
			return eb.Add(eb.Column("attempts"), 1)
		}).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", id)
		}).
		Exec(ctx)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		// Defer is best-effort; a missing row likely means another path
		// already handled it (Done by parallel branch, or row was reaped).
		return errors.Join(result.ErrRecordNotFound)
	}

	return nil
}
