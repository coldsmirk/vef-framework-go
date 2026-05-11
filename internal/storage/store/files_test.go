package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// fileModel mirrors a typical business struct that mixes a scalar key
// field and a richtext field carrying embedded resource URLs.
type fileModel struct {
	CoverKey string `meta:"uploaded_file"`
	Body     string `meta:"richtext"`
}

func setupFiles(t *testing.T) (context.Context, orm.DB, storage.ClaimStore, storage.DeleteQueue, storage.Files) {
	t.Helper()

	ctx, db, cs, dq := setupStores(t)
	return ctx, db, cs, dq, storage.NewFiles(cs, dq)
}

func TestFiles_OnCreate_ConsumesClaims(t *testing.T) {
	ctx, db, cs, _, files := setupFiles(t)

	claim := newClaim("priv/2026/05/10/cover.png", timex.Now().AddHours(1))
	require.NoError(t, cs.Create(ctx, claim))

	model := &fileModel{CoverKey: claim.Key}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return files.OnCreate(txCtx, tx, model)
	}))

	_, err := cs.GetByKey(ctx, claim.Key)
	assert.ErrorIs(t, err, storage.ErrClaimNotFound, "claim must be consumed")
}

func TestFiles_OnCreate_MissingClaimRollsBack(t *testing.T) {
	ctx, db, cs, _, files := setupFiles(t)

	existing := newClaim("priv/exists.bin", timex.Now().AddHours(1))
	require.NoError(t, cs.Create(ctx, existing))

	model := &fileModel{CoverKey: "priv/never-uploaded.bin"}

	err := db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return files.OnCreate(txCtx, tx, model)
	})
	assert.ErrorIs(t, err, storage.ErrClaimNotFound)

	got, err := cs.GetByKey(ctx, existing.Key)
	require.NoError(t, err)
	assert.Equal(t, existing.ID, got.ID, "unrelated claim survives rollback")
}

func TestFiles_OnUpdate_DiffsAcrossSnapshots(t *testing.T) {
	ctx, db, cs, dq, files := setupFiles(t)

	newCover := newClaim("priv/new-cover.png", timex.Now().AddHours(1))
	require.NoError(t, cs.Create(ctx, newCover))

	old := &fileModel{CoverKey: "priv/old-cover.png"}
	updated := &fileModel{CoverKey: newCover.Key}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return files.OnUpdate(txCtx, tx, old, updated)
	}))

	_, err := cs.GetByKey(ctx, newCover.Key)
	assert.ErrorIs(t, err, storage.ErrClaimNotFound, "new cover claim consumed")

	leased, err := dq.Lease(ctx, timex.Now(), 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, leased, 1)
	assert.Equal(t, old.CoverKey, leased[0].Key)
	assert.Equal(t, storage.DeleteReasonReplaced, leased[0].Reason)
}

func TestFiles_OnUpdate_NoChangeIsNoop(t *testing.T) {
	ctx, db, _, dq, files := setupFiles(t)

	model := &fileModel{CoverKey: "priv/same.png"}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return files.OnUpdate(txCtx, tx, model, model)
	}))

	leased, err := dq.Lease(ctx, timex.Now(), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, leased)
}

func TestFiles_OnDelete_SchedulesEveryRef(t *testing.T) {
	ctx, db, _, dq, files := setupFiles(t)

	model := &fileModel{
		CoverKey: "priv/cover.png",
		Body:     `<p><img src="priv/body-1.png"><img src="priv/body-2.png"></p>`,
	}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return files.OnDelete(txCtx, tx, model)
	}))

	leased, err := dq.Lease(ctx, timex.Now(), 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, leased, 3)

	for _, item := range leased {
		assert.Equal(t, storage.DeleteReasonDeleted, item.Reason)
	}
}

func TestFiles_NilModelIsNoop(t *testing.T) {
	ctx, db, _, dq, files := setupFiles(t)

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		if err := files.OnCreate(txCtx, tx, (*fileModel)(nil)); err != nil {
			return err
		}
		if err := files.OnDelete(txCtx, tx, (*fileModel)(nil)); err != nil {
			return err
		}
		return files.OnUpdate(txCtx, tx, (*fileModel)(nil), (*fileModel)(nil))
	}))

	leased, err := dq.Lease(ctx, timex.Now(), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, leased)
}
