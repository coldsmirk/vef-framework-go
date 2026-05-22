package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/migration"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// owner is the principal that owns every claim newClaim builds.
// Tests use it as the default ConsumeMany caller; cases that exercise
// the ownership boundary build their own principal.
var owner = &security.Principal{ID: "tester"}

// Type aliases keep the test code readable while making the dependency
// on the internal store package explicit.
type (
	uploadClaim = store.UploadClaim
)

// ── shared test infrastructure ──────────────────────────────────────────
//
// setupStores lives at the top of claim_store_test.go because that is
// where the shared bundle is first introduced. delete_queue_test.go
// reuses it directly via the shared package.

func setupStores(t *testing.T) (context.Context, orm.DB, store.ClaimStore, store.DeleteQueue) {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)

	require.NoError(t, migration.Migrate(ctx, db, config.SQLite), "Storage migration should succeed")

	return ctx, db, store.NewClaimStore(db), store.NewDeleteQueue(db)
}

func newClaim(key string, expiresAt timex.DateTime) *uploadClaim {
	return &uploadClaim{
		ID:               id.GenerateUUID(),
		Key:              key,
		Size:             1024,
		ContentType:      "application/octet-stream",
		OriginalFilename: "测试文件.bin",
		CreatedBy:        "tester",
		Status:           store.ClaimStatusPending,
		ExpiresAt:        expiresAt,
		CreatedAt:        timex.Now(),
	}
}

// ── TestClaimStore ──────────────────────────────────────────────────────

func TestClaimStore(t *testing.T) {
	t.Run("CreateAndGet", func(t *testing.T) {
		ctx, _, cs, _ := setupStores(t)

		claim := newClaim("priv/2026/05/10/abc.bin", timex.Now().AddHours(1))
		require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")

		gotByID, err := cs.Get(ctx, claim.ID)
		require.NoError(t, err, "Claim lookup by ID should succeed")
		assert.Equal(t, claim.Key, gotByID.Key, "Lookup by ID should return matching key")
		assert.Equal(t, claim.OriginalFilename, gotByID.OriginalFilename, "OriginalFilename must round-trip through the claim row")

		gotByKey, err := cs.GetByKey(ctx, claim.Key)
		require.NoError(t, err, "Claim lookup by key should succeed")
		assert.Equal(t, claim.ID, gotByKey.ID, "Lookup by key should return matching ID")
	})

	t.Run("GetMissing", func(t *testing.T) {
		ctx, _, cs, _ := setupStores(t)

		_, err := cs.Get(ctx, "non-existent")
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Missing claim lookup by ID should return ErrClaimNotFound")

		_, err = cs.GetByKey(ctx, "non-existent")
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Missing claim lookup by key should return ErrClaimNotFound")
	})

	t.Run("ConsumeInTx", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		claim := newClaim("priv/k1", timex.Now().AddHours(1))
		claim.Status = store.ClaimStatusUploaded
		require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, claim.Key)
		}), "Claim consumption transaction should succeed")

		_, err := cs.Get(ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Claim should be gone after Consume")
	})

	t.Run("ConsumeMissingFailsAndRollsBack", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		claim := newClaim("priv/exists", timex.Now().AddHours(1))
		require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")

		// Try to consume both an existing and a non-existing key in one tx.
		err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.ConsumeMany(txCtx, tx, owner, []string{claim.Key, "priv/missing"})
		})
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Missing claim should fail the transaction")

		// Rollback should leave the existing claim intact.
		got, err := cs.GetByKey(ctx, claim.Key)
		require.NoError(t, err, "Existing claim lookup should succeed after rollback")
		assert.Equal(t, claim.ID, got.ID, "Rollback should leave the existing claim intact")
	})

	t.Run("ConsumeManyEmpty", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.ConsumeMany(txCtx, tx, owner, nil)
		}), "Consuming an empty claim list should succeed")
	})

	t.Run("ConsumeManyRejectsOtherOwner", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		claim := newClaim("priv/owned-by-tester", timex.Now().AddHours(1))
		require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.MarkUploaded(txCtx, tx, claim.ID)
		}), "MarkUploaded should succeed so the claim is consumable")

		intruder := &security.Principal{ID: "someone-else"}

		err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.ConsumeMany(txCtx, tx, intruder, []string{claim.Key})
		})
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Consuming another principal's claim must be rejected with the same sentinel as missing keys")

		// And the claim must survive the rollback intact.
		got, err := cs.GetByKey(ctx, claim.Key)
		require.NoError(t, err, "Original owner's claim must remain after the rejected consume")
		assert.Equal(t, claim.ID, got.ID, "Claim row should be untouched")
	})

	t.Run("ConsumeManyRejectsAnonymousPrincipal", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.ConsumeMany(txCtx, tx, nil, []string{"priv/whatever"})
		})
		assert.ErrorIs(t, err, storage.ErrAccessDenied, "Anonymous principal must be rejected upfront with access-denied (not claim-not-found, which would mislead debugging)")
	})

	t.Run("ScanExpired", func(t *testing.T) {
		ctx, _, cs, _ := setupStores(t)

		now := timex.Now()
		expired := newClaim("priv/expired", now.AddHours(-1))
		live := newClaim("priv/live", now.AddHours(1))

		require.NoError(t, cs.Create(ctx, expired), "Expired claim creation should succeed")
		require.NoError(t, cs.Create(ctx, live), "Live claim creation should succeed")

		got, err := cs.ScanExpired(ctx, now, 10)
		require.NoError(t, err, "Expired claim scan should succeed")
		require.Len(t, got, 1, "Only the expired claim should be returned")
		assert.Equal(t, expired.ID, got[0].ID, "Expired scan should return the expired claim")

		require.NoError(t, cs.DeleteByID(ctx, expired.ID), "Expired claim deletion should succeed")

		got, err = cs.ScanExpired(ctx, now, 10)
		require.NoError(t, err, "Expired claim rescan should succeed")
		assert.Empty(t, got, "Deleted expired claim should not appear in later scans")
	})

	t.Run("ScanExpiredSkipsUploaded", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		now := timex.Now()
		uploaded := newClaim("priv/uploaded-past-ttl", now.AddHours(-1))
		require.NoError(t, cs.Create(ctx, uploaded), "Uploaded claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.MarkUploaded(txCtx, tx, uploaded.ID)
		}), "MarkUploaded should succeed")

		got, err := cs.ScanExpired(ctx, now, 10)
		require.NoError(t, err, "Expired claim scan should succeed")
		assert.Empty(t, got, "Uploaded claims must never appear in the expired sweep set")
	})

	t.Run("MarkUploadedIfPendingExpired", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		now := timex.Now()
		claim := newClaim("priv/conditional-uploaded", now.AddHours(-1))
		claim.UploadID = "session-1"
		require.NoError(t, cs.Create(ctx, claim), "Expired claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			updated, err := cs.MarkUploadedIfPendingExpired(txCtx, tx, *claim, now)
			require.NoError(t, err, "Conditional upload mark should not fail")
			assert.True(t, updated, "Expired pending claim should be updated")

			return nil
		}), "Conditional upload mark transaction should succeed")

		got, err := cs.Get(ctx, claim.ID)
		require.NoError(t, err, "Claim lookup should succeed")
		assert.Equal(t, store.ClaimStatusUploaded, got.Status, "Conditional update should mark the claim uploaded")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			updated, err := cs.MarkUploadedIfPendingExpired(txCtx, tx, *claim, now)
			require.NoError(t, err, "Conditional upload mark should not fail for stale snapshot")
			assert.False(t, updated, "Already uploaded claim should not be updated again")

			return nil
		}), "Stale conditional upload mark transaction should succeed")
	})

	t.Run("DeleteIfPendingExpired", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		now := timex.Now()
		claim := newClaim("priv/conditional-delete", now.AddHours(-1))
		claim.UploadID = "session-delete"
		require.NoError(t, cs.Create(ctx, claim), "Expired claim creation should succeed")

		staleSnapshot := *claim
		staleSnapshot.UploadID = "stale-session"

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			deleted, err := cs.DeleteIfPendingExpired(txCtx, tx, staleSnapshot, now)
			require.NoError(t, err, "Conditional delete should not fail for stale snapshot")
			assert.False(t, deleted, "Changed UploadID should make the stale snapshot lose")

			return nil
		}), "Stale conditional delete transaction should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			deleted, err := cs.DeleteIfPendingExpired(txCtx, tx, *claim, now)
			require.NoError(t, err, "Conditional delete should not fail")
			assert.True(t, deleted, "Matching expired pending claim should be deleted")

			return nil
		}), "Conditional delete transaction should succeed")

		_, err := cs.Get(ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Deleted claim should no longer be queryable")
	})

	t.Run("ErrClaimNotFoundWraps", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, "missing")
		})
		require.Error(t, err, "Missing claim consumption should fail")
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Consume error should wrap ErrClaimNotFound")
	})
}
