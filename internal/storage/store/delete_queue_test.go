package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func newPending(key string, nextAt timex.DateTime) store.PendingDelete {
	return store.PendingDelete{
		ID:            id.GenerateUUID(),
		Key:           key,
		Reason:        storage.DeleteReasonReplaced,
		NextAttemptAt: nextAt,
		CreatedAt:     timex.Now(),
	}
}

func TestDeleteQueue(t *testing.T) {
	t.Run("EnqueueAndLease", func(t *testing.T) {
		ctx, db, _, dq := setupStores(t)

		now := timex.Now()
		items := []store.PendingDelete{
			newPending("priv/a", now),
			newPending("priv/b", now),
		}

		require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
			return dq.Enqueue(txCtx, tx, items)
		}), "Pending deletes should be enqueued inside the transaction")

		leased, err := dq.Lease(ctx, now, 10, time.Minute)
		require.NoError(t, err, "Pending delete lease should succeed")
		assert.Len(t, leased, 2, "Lease should return all due pending deletes")

		// Re-leasing immediately should yield nothing because the visibility
		// timeout pushed the rows into the future.
		again, err := dq.Lease(ctx, now, 10, time.Minute)
		require.NoError(t, err, "Immediate re-lease should succeed")
		assert.Empty(t, again, "Leased rows must not be visible until lease expires")
	})

	t.Run("ScheduleByKeysWritesOneRowPerKey", func(t *testing.T) {
		// Public DeleteScheduler.Schedule(keys, reason): the queue must
		// build one PendingDelete per deduplicated key with the supplied
		// reason, all sharing the current timestamp as NextAttemptAt.
		ctx, db, _, dq := setupStores(t)

		require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
			return dq.Schedule(txCtx, tx, []string{"priv/x", "priv/y", "priv/x"}, storage.DeleteReasonReplaced)
		}), "Schedule(keys, reason) should succeed inside the transaction")

		leased, err := dq.Lease(ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease after schedule should succeed")
		require.Len(t, leased, 2, "Schedule should dedupe before insert (priv/x appears once)")

		keys := []string{leased[0].Key, leased[1].Key}
		assert.ElementsMatch(t, []string{"priv/x", "priv/y"}, keys, "Scheduled keys should match the deduplicated input")

		for _, row := range leased {
			assert.Equal(t, storage.DeleteReasonReplaced, row.Reason, "Schedule should propagate the reason verbatim")
			assert.Empty(t, row.UploadID, "Schedule(keys, reason) must never set a multipart UploadID")
		}
	})

	t.Run("Done", func(t *testing.T) {
		ctx, db, _, dq := setupStores(t)

		now := timex.Now()
		item := newPending("priv/done", now)

		require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
			return dq.Enqueue(txCtx, tx, []store.PendingDelete{item})
		}), "Pending delete should be enqueued inside the transaction")

		require.NoError(t, dq.Done(ctx, []string{item.ID}), "Done should remove the pending delete row")

		leased, err := dq.Lease(ctx, now.AddHours(24), 10, time.Minute)
		require.NoError(t, err, "Lease after Done should succeed")
		assert.Empty(t, leased, "Done should remove the row entirely")
	})

	t.Run("DeferIncrementsAttempts", func(t *testing.T) {
		ctx, db, _, dq := setupStores(t)

		now := timex.Now()
		item := newPending("priv/defer", now)

		require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
			return dq.Enqueue(txCtx, tx, []store.PendingDelete{item})
		}), "Pending delete should be enqueued inside the transaction")

		leased, err := dq.Lease(ctx, now, 10, time.Minute)
		require.NoError(t, err, "Initial lease should succeed")
		require.Len(t, leased, 1, "Initial lease should return the scheduled row")

		nextAt := now.AddHours(1)
		require.NoError(t, dq.Defer(ctx, item.ID, nextAt), "Deferring a leased row should succeed")

		// Move now past nextAt and confirm Lease returns it with attempts=1.
		leased, err = dq.Lease(ctx, nextAt.AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease after defer timeout should succeed")
		require.Len(t, leased, 1, "Deferred row should become visible after next attempt time")
		assert.Equal(t, 1, leased[0].Attempts, "Deferred row should increment attempts")
	})

	t.Run("EnqueueEmpty", func(t *testing.T) {
		ctx, db, _, dq := setupStores(t)

		require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
			return dq.Enqueue(txCtx, tx, nil)
		}), "Enqueueing an empty delete list should succeed")

		leased, err := dq.Lease(ctx, timex.Now(), 10, time.Minute)
		require.NoError(t, err, "Lease after empty enqueue should succeed")
		assert.Empty(t, leased, "Empty enqueue should not create pending delete rows")
	})

	t.Run("ScheduleEmpty", func(t *testing.T) {
		ctx, db, _, dq := setupStores(t)

		require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
			return dq.Schedule(txCtx, tx, nil, storage.DeleteReasonReplaced)
		}), "Schedule with no keys should succeed")

		leased, err := dq.Lease(ctx, timex.Now(), 10, time.Minute)
		require.NoError(t, err, "Lease after empty schedule should succeed")
		assert.Empty(t, leased, "Empty schedule must not create pending delete rows")
	})
}
