package store

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NewDeleteQueue returns the default DeleteQueue implementation backed
// by the orm.DB abstraction. The concrete SQL dialect is determined by
// the underlying orm provider; this package depends only on orm.DB.
//
// The Lease implementation issues SELECT ... FOR UPDATE SKIP LOCKED
// inside a transaction so multi-instance worker deployments can run
// without leader election: each worker leases a disjoint slice of due
// rows, and the visibility-timeout UPDATE pushes them out of sight for
// the lease window. SQLite, which lacks row-level locking, transparently
// drops the FOR UPDATE clause via the ORM (single-writer DB → no race
// to begin with). MySQL and PostgreSQL execute the lock as expected.
//
// The returned value also satisfies the public storage.DeleteScheduler
// interface; the fx graph exposes both surfaces.
func NewDeleteQueue(db orm.DB) DeleteQueue {
	return &deleteQueue{db: db}
}

type deleteQueue struct {
	db orm.DB
}

// Schedule implements storage.DeleteScheduler. It builds a PendingDelete
// row per (deduplicated) key with the supplied reason and forwards them
// to Enqueue inside the same business transaction.
func (q *deleteQueue) Schedule(ctx context.Context, tx orm.DB, keys []string, reason storage.DeleteReason) error {
	if len(keys) == 0 {
		return nil
	}

	uniq := dedupeStrings(keys)
	now := timex.Now()
	items := make([]PendingDelete, len(uniq))

	for i, key := range uniq {
		items[i] = PendingDelete{
			ID:            id.GenerateUUID(),
			Key:           key,
			Reason:        reason,
			NextAttemptAt: now,
			CreatedAt:     now,
		}
	}

	return q.Enqueue(ctx, tx, items)
}

func (*deleteQueue) Enqueue(ctx context.Context, tx orm.DB, items []PendingDelete) error {
	if len(items) == 0 {
		return nil
	}

	_, err := tx.NewInsert().Model(&items).Exec(ctx)

	return err
}

func (q *deleteQueue) Lease(ctx context.Context, now timex.DateTime, limit int, leaseDuration time.Duration) ([]PendingDelete, error) {
	if limit <= 0 {
		return nil, nil
	}

	var leased []PendingDelete

	err := q.db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		var candidates []PendingDelete

		// SKIP LOCKED is essential for the multi-worker deployment story:
		// without it, two workers calling Lease at the same tick would
		// both pick the same earliest rows and then race on the UPDATE,
		// effectively serializing on the queue head. The ORM transparently
		// degrades the clause on SQLite.
		err := tx.NewSelect().Model(&candidates).Where(func(cb orm.ConditionBuilder) {
			cb.LessThanOrEqual("next_attempt_at", now)
		}).OrderBy("next_attempt_at").Limit(limit).ForUpdateSkipLocked().Scan(txCtx)
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

		_, err = tx.NewUpdate().Model((*PendingDelete)(nil)).
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

func (q *deleteQueue) Done(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	_, err := q.db.NewDelete().Model((*PendingDelete)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.In("id", ids)
	}).Exec(ctx)

	return err
}

func (q *deleteQueue) Defer(ctx context.Context, id string, nextAt timex.DateTime) error {
	res, err := q.db.NewUpdate().Model((*PendingDelete)(nil)).
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
		return result.ErrRecordNotFound
	}

	return nil
}
