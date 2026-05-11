package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/migration"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func setupStores(t *testing.T) (context.Context, orm.DB, storage.ClaimStore, storage.DeleteQueue) {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)

	require.NoError(t, migration.Migrate(ctx, db, config.SQLite))

	return ctx, db, store.NewClaimStore(db), store.NewDeleteQueue(db)
}

func newClaim(key string, expiresAt timex.DateTime) *storage.UploadClaim {
	return &storage.UploadClaim{
		ID:          id.GenerateUUID(),
		Key:         key,
		Bucket:      "test-bucket",
		Size:        1024,
		ContentType: "application/octet-stream",
		CreatedBy:   "tester",
		ExpiresAt:   expiresAt,
		CreatedAt:   timex.Now(),
	}
}

func TestClaimStore_CreateAndGet(t *testing.T) {
	ctx, _, cs, _ := setupStores(t)

	claim := newClaim("priv/2026/05/10/abc.bin", timex.Now().AddHours(1))
	require.NoError(t, cs.Create(ctx, claim))

	gotByID, err := cs.Get(ctx, claim.ID)
	require.NoError(t, err)
	assert.Equal(t, claim.Key, gotByID.Key)

	gotByKey, err := cs.GetByKey(ctx, claim.Key)
	require.NoError(t, err)
	assert.Equal(t, claim.ID, gotByKey.ID)
}

func TestClaimStore_GetMissing(t *testing.T) {
	ctx, _, cs, _ := setupStores(t)

	_, err := cs.Get(ctx, "non-existent")
	assert.ErrorIs(t, err, storage.ErrClaimNotFound)

	_, err = cs.GetByKey(ctx, "non-existent")
	assert.ErrorIs(t, err, storage.ErrClaimNotFound)
}

func TestClaimStore_ConsumeInTx(t *testing.T) {
	ctx, db, cs, _ := setupStores(t)

	claim := newClaim("priv/k1", timex.Now().AddHours(1))
	require.NoError(t, cs.Create(ctx, claim))

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return cs.Consume(txCtx, tx, claim.Key)
	}))

	_, err := cs.Get(ctx, claim.ID)
	assert.ErrorIs(t, err, storage.ErrClaimNotFound, "claim should be gone after Consume")
}

func TestClaimStore_ConsumeMissingFailsAndRollsBack(t *testing.T) {
	ctx, db, cs, _ := setupStores(t)

	claim := newClaim("priv/exists", timex.Now().AddHours(1))
	require.NoError(t, cs.Create(ctx, claim))

	// Try to consume both an existing and a non-existing key in one tx.
	err := db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return cs.ConsumeMany(txCtx, tx, []string{claim.Key, "priv/missing"})
	})
	assert.ErrorIs(t, err, storage.ErrClaimNotFound)

	// Rollback should leave the existing claim intact.
	got, err := cs.GetByKey(ctx, claim.Key)
	require.NoError(t, err)
	assert.Equal(t, claim.ID, got.ID)
}

func TestClaimStore_ConsumeManyEmpty(t *testing.T) {
	ctx, db, cs, _ := setupStores(t)

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return cs.ConsumeMany(txCtx, tx, nil)
	}))
}

func TestClaimStore_ScanExpired(t *testing.T) {
	ctx, _, cs, _ := setupStores(t)

	now := timex.Now()
	expired := newClaim("priv/expired", now.AddHours(-1))
	live := newClaim("priv/live", now.AddHours(1))

	require.NoError(t, cs.Create(ctx, expired))
	require.NoError(t, cs.Create(ctx, live))

	got, err := cs.ScanExpired(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, expired.ID, got[0].ID)

	require.NoError(t, cs.DeleteByID(ctx, expired.ID))

	got, err = cs.ScanExpired(ctx, now, 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func newPending(key string, nextAt timex.DateTime) storage.PendingDelete {
	return storage.PendingDelete{
		ID:            id.GenerateUUID(),
		Key:           key,
		Reason:        storage.DeleteReasonReplaced,
		NextAttemptAt: nextAt,
		CreatedAt:     timex.Now(),
	}
}

func TestDeleteQueue_ScheduleAndLease(t *testing.T) {
	ctx, db, _, dq := setupStores(t)

	now := timex.Now()
	items := []storage.PendingDelete{
		newPending("priv/a", now),
		newPending("priv/b", now),
	}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, items)
	}))

	leased, err := dq.Lease(ctx, now, 10, time.Minute)
	require.NoError(t, err)
	assert.Len(t, leased, 2)

	// Re-leasing immediately should yield nothing because the visibility
	// timeout pushed the rows into the future.
	again, err := dq.Lease(ctx, now, 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, again, "leased rows must not be visible until lease expires")
}

func TestDeleteQueue_Done(t *testing.T) {
	ctx, db, _, dq := setupStores(t)

	now := timex.Now()
	item := newPending("priv/done", now)

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, []storage.PendingDelete{item})
	}))

	require.NoError(t, dq.Done(ctx, []string{item.ID}))

	leased, err := dq.Lease(ctx, now.AddHours(24), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, leased, "Done should remove the row entirely")
}

func TestDeleteQueue_DeferIncrementsAttempts(t *testing.T) {
	ctx, db, _, dq := setupStores(t)

	now := timex.Now()
	item := newPending("priv/defer", now)

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, []storage.PendingDelete{item})
	}))

	leased, err := dq.Lease(ctx, now, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, leased, 1)

	nextAt := now.AddHours(1)
	require.NoError(t, dq.Defer(ctx, item.ID, nextAt))

	// Move now past nextAt and confirm Lease returns it with attempts=1.
	leased, err = dq.Lease(ctx, nextAt.AddHours(1), 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, leased, 1)
	assert.Equal(t, 1, leased[0].Attempts)
}

func TestDeleteQueue_ScheduleEmpty(t *testing.T) {
	ctx, db, _, dq := setupStores(t)

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, nil)
	}))

	leased, err := dq.Lease(ctx, timex.Now(), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, leased)
}

// Sanity: ensure ErrClaimNotFound wraps cleanly via errors.Is even when
// formatted with key context.
func TestErrClaimNotFoundWraps(t *testing.T) {
	ctx, db, cs, _ := setupStores(t)

	err := db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return cs.Consume(txCtx, tx, "missing")
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrClaimNotFound))
}
