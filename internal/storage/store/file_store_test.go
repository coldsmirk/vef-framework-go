package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// recordClaim finalizes claim through the claim store so the registry
// row is written exactly the way production writes it — via the
// pending → uploaded compare-and-set rather than a direct insert.
func recordClaim(t *testing.T, ctx context.Context, db orm.DB, cs store.ClaimStore, claim *store.UploadClaim) {
	t.Helper()

	require.NoError(t, cs.Create(ctx, claim), "Claim creation should succeed")
	require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
		return cs.MarkUploaded(txCtx, tx, *claim)
	}), "MarkUploaded should succeed and record the file")
}

func TestFileStore(t *testing.T) {
	t.Run("RecordProjectsTheClaim", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)
		fs := store.NewFileStore(db)

		claim := newClaim("priv/2026/08/04/abc.pdf", timex.Now().AddHours(1))
		claim.OriginalFilename = "季度报告.pdf"
		claim.ContentType = "application/pdf"
		claim.Size = 4096
		claim.Public = true

		recordClaim(t, ctx, db, cs, claim)

		found, err := fs.Lookup(ctx, []string{claim.Key})
		require.NoError(t, err, "Registry lookup should succeed")
		require.Len(t, found, 1, "The finalized upload should have exactly one record")

		got := found[claim.Key]
		assert.Equal(t, claim.ID, got.ID, "The record should carry the originating claim's ID")
		assert.Equal(t, "季度报告.pdf", got.OriginalFilename, "The original filename is the whole point of the registry")
		assert.Equal(t, "application/pdf", got.ContentType, "Content type should be projected from the claim")
		assert.Equal(t, int64(4096), got.Size, "Size should be projected from the claim")
		assert.True(t, got.Public, "Visibility should be projected from the claim")
		assert.Equal(t, claim.CreatedBy, got.CreatedBy, "The uploader must survive as the record's creator")
		assert.Equal(t, storage.FileStatusUploaded, got.Status, "A freshly recorded file is uploaded, not yet claimed")
		assert.Equal(t, claim.CreatedAt.String(), got.StartedAt.String(),
			"StartedAt should be the upload session start, not the record write time")
		assert.Nil(t, got.ClaimedAt, "An unadopted file has no claimed timestamp")
		assert.Nil(t, got.DeletedAt, "A live file has no deleted timestamp")
	})

	t.Run("LookupOmitsUnknownKeys", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)
		fs := store.NewFileStore(db)

		claim := newClaim("priv/known.bin", timex.Now().AddHours(1))
		recordClaim(t, ctx, db, cs, claim)

		found, err := fs.Lookup(ctx, []string{claim.Key, claim.Key, "priv/never-uploaded.bin"})
		require.NoError(t, err, "Registry lookup should succeed for a partially unknown key set")
		assert.Len(t, found, 1, "Duplicate keys collapse and unknown keys are omitted rather than erroring")

		empty, err := fs.Lookup(ctx, nil)
		require.NoError(t, err, "Empty lookup should succeed")
		assert.Empty(t, empty, "Empty lookup should return an empty map")
	})

	t.Run("MarkClaimed", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)
		fs := store.NewFileStore(db)

		claim := newClaim("priv/adopted.bin", timex.Now().AddHours(1))
		recordClaim(t, ctx, db, cs, claim)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return fs.MarkClaimed(txCtx, tx, []string{claim.Key})
		}), "Marking a recorded file as claimed should succeed")

		found, err := fs.Lookup(ctx, []string{claim.Key})
		require.NoError(t, err, "Registry lookup should succeed")
		assert.Equal(t, storage.FileStatusClaimed, found[claim.Key].Status, "Adoption should flip the record to claimed")
		assert.NotNil(t, found[claim.Key].ClaimedAt, "Adoption should stamp the claimed timestamp")
	})

	// The registry is a projection, never an authority: a key with no
	// record — every file uploaded before this table existed — must not
	// fail the business write that adopts it.
	t.Run("MarkClaimedToleratesMissingRecord", func(t *testing.T) {
		ctx, db, _, _ := setupStores(t)
		fs := store.NewFileStore(db)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return fs.MarkClaimed(txCtx, tx, []string{"priv/uploaded-before-the-registry.bin"})
		}), "Marking an unrecorded key as claimed must be a silent no-op")
	})

	t.Run("MarkDeletedFirstWriterWins", func(t *testing.T) {
		ctx, db, cs, _ := setupStores(t)
		fs := store.NewFileStore(db)

		claim := newClaim("priv/doomed.bin", timex.Now().AddHours(1))
		recordClaim(t, ctx, db, cs, claim)

		// One key legitimately carries several queue rows — the delete
		// queue is unique on (object_key, reason) — so the reason that
		// actually removed the object must not be rewritten by the next
		// row to drain.
		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return fs.MarkDeleted(txCtx, tx, claim.Key, storage.DeleteReasonReplaced)
		}), "First delete marking should succeed")

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return fs.MarkDeleted(txCtx, tx, claim.Key, storage.DeleteReasonDeleted)
		}), "Second delete marking should succeed without error")

		found, err := fs.Lookup(ctx, []string{claim.Key})
		require.NoError(t, err, "Registry lookup should succeed")

		got := found[claim.Key]
		require.Len(t, found, 1, "A deleted file keeps its record — that is the audit trail")
		assert.Equal(t, storage.FileStatusDeleted, got.Status, "The record should report the object as deleted")
		assert.Equal(t, storage.DeleteReasonReplaced, got.DeleteReason, "The first writer's reason must survive")
		assert.NotNil(t, got.DeletedAt, "Deletion should stamp the deleted timestamp")
	})

	t.Run("MarkDeletedToleratesMissingRecord", func(t *testing.T) {
		ctx, db, _, _ := setupStores(t)
		fs := store.NewFileStore(db)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
			return fs.MarkDeleted(txCtx, tx, "priv/never-recorded.bin", storage.DeleteReasonClaimExpired)
		}), "Deleting an object that never had a record must be a silent no-op")
	})
}
