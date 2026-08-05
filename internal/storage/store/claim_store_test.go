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
// Tests use it as the default Consume caller; cases that exercise
// the ownership boundary build their own principal.
var owner = &security.Principal{ID: "tester"}

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

	return ctx, db, store.NewClaimStore(db, store.NewFileStore(db)), store.NewDeleteQueue(db)
}

func newClaim(key string, expiresAt timex.DateTime) *store.UploadClaim {
	return &store.UploadClaim{
		ID:               id.GenerateUUID(),
		Key:              key,
		Size:             1024,
		ContentType:      "application/octet-stream",
		OriginalFilename: "test-file.bin",
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

		got, err := cs.Get(ctx, claim.ID)
		require.NoError(t, err, "Claim lookup by ID should succeed")
		assert.Equal(t, claim.Key, got.Key, "Lookup by ID should return matching key")
		assert.Equal(t, claim.OriginalFilename, got.OriginalFilename, "OriginalFilename must round-trip through the claim row")
	})

	t.Run("GetMissing", func(t *testing.T) {
		ctx, _, cs, _ := setupStores(t)

		_, err := cs.Get(ctx, "non-existent")
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Missing claim lookup by ID should return ErrClaimNotFound")
	})

	t.Run("ConsumeInTx", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		claim := newClaim("priv/k1", timex.Now().AddHours(1))
		claim.Status = store.ClaimStatusUploaded
		require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, owner, []string{claim.Key})
		}), "Claim consumption transaction should succeed")

		_, err := cs.Get(ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Claim should be gone after Consume")
	})

	// Adoption is what deletes the claim row and takes the original
	// filename with it, so the registry record must pick the fact up in
	// the same transaction.
	t.Run("ConsumeMarksTheRecordClaimed", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)
		fs := store.NewFileStore(db)

		claim := newClaim("priv/consume-marks.bin", timex.Now().AddHours(1))
		recordClaim(t, ctx, db, cs, claim)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, owner, []string{claim.Key})
		}), "Claim consumption transaction should succeed")

		found, err := fs.Lookup(ctx, []string{claim.Key})
		require.NoError(t, err, "Registry lookup should succeed")
		require.Len(t, found, 1, "The record must outlive the claim row Consume deletes")
		assert.Equal(t, storage.FileStatusClaimed, found[claim.Key].Status, "Adoption should flip the record to claimed")
		assert.Equal(t, claim.OriginalFilename, found[claim.Key].OriginalFilename,
			"The original filename must survive the adoption that deletes the claim")
	})

	// Files uploaded before the registry existed have a claim but no
	// record. Adopting one must still succeed — the claim table, not the
	// registry, is the authority on what a caller may consume.
	t.Run("ConsumeSucceedsWithoutARecord", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)
		fs := store.NewFileStore(db)

		claim := newClaim("priv/legacy.bin", timex.Now().AddHours(1))
		claim.Status = store.ClaimStatusUploaded
		require.NoError(t, cs.Create(ctx, claim), "Legacy claim creation should succeed")

		found, err := fs.Lookup(ctx, []string{claim.Key})
		require.NoError(t, err, "Registry lookup should succeed")
		require.Empty(t, found, "The legacy fixture must have no record for this test to mean anything")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, owner, []string{claim.Key})
		}), "Adopting a file with no registry record must not fail the business write")

		_, err = cs.Get(ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "The claim should still have been consumed")
	})

	t.Run("ConsumeMissingFailsAndRollsBack", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		claim := newClaim("priv/exists", timex.Now().AddHours(1))
		require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")

		// Flip to uploaded so Consume's status='uploaded' filter does not
		// silently drop this row — without this the test would pass even
		// if Consume never tried to delete anything.
		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.MarkUploaded(txCtx, tx, *claim)
		}), "MarkUploaded should succeed so the claim is consumable")

		// Try to consume both an existing and a non-existing key in one tx.
		err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, owner, []string{claim.Key, "priv/missing"})
		})
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Missing claim should fail the transaction")

		// Rollback must leave the existing uploaded claim intact, proving
		// the partial DELETE was undone (not just that Consume short-
		// circuited before any write).
		got, err := cs.Get(ctx, claim.ID)
		require.NoError(t, err, "Existing claim lookup should succeed after rollback")
		assert.Equal(t, claim.Key, got.Key, "Rollback should leave the existing claim intact")
		assert.Equal(t, store.ClaimStatusUploaded, got.Status, "Rollback should preserve the uploaded status")
	})

	t.Run("ConsumeEmpty", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, owner, nil)
		}), "Consuming an empty claim list should succeed")
	})

	t.Run("ConsumeRejectsOtherOwner", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		claim := newClaim("priv/owned-by-tester", timex.Now().AddHours(1))
		require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.MarkUploaded(txCtx, tx, *claim)
		}), "MarkUploaded should succeed so the claim is consumable")

		intruder := &security.Principal{ID: "someone-else"}

		err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.Consume(txCtx, tx, intruder, []string{claim.Key})
		})
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Consuming another principal's claim must be rejected with the same sentinel as missing keys")

		// And the claim must survive the rollback intact.
		got, err := cs.Get(ctx, claim.ID)
		require.NoError(t, err, "Original owner's claim must remain after the rejected consume")
		assert.Equal(t, claim.Key, got.Key, "Claim row should be untouched")
	})

	t.Run("ConsumeRejectsAnonymousPrincipal", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		anonCases := []struct {
			name      string
			principal *security.Principal
		}{
			{"Nil", nil},
			{"EmptyID", &security.Principal{ID: ""}},
			{"AnonymousSentinel", security.PrincipalAnonymous},
		}

		for _, tc := range anonCases {
			t.Run(tc.name, func(t *testing.T) {
				err := db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
					return cs.Consume(txCtx, tx, tc.principal, []string{"priv/whatever"})
				})
				assert.ErrorIs(t, err, storage.ErrAccessDenied,
					"Anonymous principal must be rejected upfront with access-denied "+
						"(not claim-not-found, which would mislead debugging)")
			})
		}
	})

	t.Run("ListExpired", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		now := timex.Now()
		expired := newClaim("priv/expired", now.AddHours(-1))
		live := newClaim("priv/live", now.AddHours(1))

		require.NoError(t, cs.Create(ctx, expired), "Expired claim creation should succeed")
		require.NoError(t, cs.Create(ctx, live), "Live claim creation should succeed")

		got, err := cs.ListExpired(ctx, now, 10)
		require.NoError(t, err, "Expired claim listing should succeed")
		require.Len(t, got, 1, "Only the expired claim should be returned")
		assert.Equal(t, expired.ID, got[0].ID, "Expired listing should return the expired claim")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			deleted, delErr := cs.DeleteIfPending(txCtx, tx, expired.ID)
			require.True(t, deleted, "Pending claim deletion should win the compare-and-set")

			return delErr
		}), "Expired claim deletion should succeed")

		got, err = cs.ListExpired(ctx, now, 10)
		require.NoError(t, err, "Expired claim relisting should succeed")
		assert.Empty(t, got, "Deleted expired claim should not appear in later listings")
	})

	t.Run("ListExpiredSkipsUploaded", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		now := timex.Now()
		uploaded := newClaim("priv/uploaded-past-ttl", now.AddHours(-1))
		require.NoError(t, cs.Create(ctx, uploaded), "Uploaded claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.MarkUploaded(txCtx, tx, *uploaded)
		}), "MarkUploaded should succeed")

		got, err := cs.ListExpired(ctx, now, 10)
		require.NoError(t, err, "Expired claim listing should succeed")
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

	// DeleteIfPending is the arbitration between abort_upload and a
	// concurrent complete_upload: losing the compare-and-set must leave
	// the finalized claim — and therefore its object — untouched.
	t.Run("DeleteIfPending", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)

		now := timex.Now()
		pending := newClaim("priv/pending-abort", now.AddHours(1))
		uploaded := newClaim("priv/uploaded-abort", now.AddHours(1))

		require.NoError(t, cs.Create(ctx, pending), "Pending claim creation should succeed")
		require.NoError(t, cs.Create(ctx, uploaded), "Uploaded claim creation should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return cs.MarkUploaded(txCtx, tx, *uploaded)
		}), "MarkUploaded should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			deleted, err := cs.DeleteIfPending(txCtx, tx, uploaded.ID)
			require.NoError(t, err, "Conditional delete should not fail on an uploaded claim")
			assert.False(t, deleted, "An uploaded claim must never lose its row to an abort")

			deleted, err = cs.DeleteIfPending(txCtx, tx, pending.ID)
			require.NoError(t, err, "Conditional delete should not fail on a pending claim")
			assert.True(t, deleted, "A pending claim should be deleted by abort")

			return nil
		}), "Conditional delete transaction should succeed")

		_, err := cs.Get(ctx, uploaded.ID)
		require.NoError(t, err, "The uploaded claim must survive the losing abort")

		_, err = cs.Get(ctx, pending.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "The aborted pending claim should be gone")
	})
}
