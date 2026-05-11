package worker_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/memory"
	"github.com/coldsmirk/vef-framework-go/internal/storage/migration"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/storage/worker"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// capturePublisher records all published events for assertion.
type capturePublisher struct {
	events []event.Event
}

func (p *capturePublisher) Publish(evt event.Event) {
	p.events = append(p.events, evt)
}

// alwaysFailService wraps a real Service but forces DeleteObject to fail,
// so we can drive the delete worker into its retry/dead-letter paths.
type alwaysFailService struct {
	storage.Service
	err error
}

func (s *alwaysFailService) DeleteObject(_ context.Context, _ storage.DeleteObjectOptions) error {
	return s.err
}

func setupWorker(t *testing.T) (context.Context, orm.DB, storage.Service, storage.ClaimStore, storage.DeleteQueue, *capturePublisher) {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)
	require.NoError(t, migration.Migrate(ctx, db, config.SQLite))

	svc := memory.New()
	cs := store.NewClaimStore(db)
	dq := store.NewDeleteQueue(db)
	pub := &capturePublisher{}

	return ctx, db, svc, cs, dq, pub
}

func putMemoryObject(t *testing.T, svc storage.Service, key string) {
	t.Helper()

	_, err := svc.PutObject(context.Background(), storage.PutObjectOptions{
		Key:    key,
		Reader: bytes.NewReader([]byte("payload")),
		Size:   7,
	})
	require.NoError(t, err)
}

func TestClaimSweeper_DeletesExpiredClaim(t *testing.T) {
	ctx, _, svc, cs, _, _ := setupWorker(t)

	putMemoryObject(t, svc, "priv/expired.bin")

	claim := &storage.UploadClaim{
		ID:        id.GenerateUUID(),
		Key:       "priv/expired.bin",
		Bucket:    "memory",
		CreatedBy: "tester",
		ExpiresAt: timex.Now().AddHours(-1),
		CreatedAt: timex.Now(),
	}
	require.NoError(t, cs.Create(ctx, claim))

	worker.NewClaimSweeper(svc, cs).Run(ctx)

	_, err := cs.Get(ctx, claim.ID)
	assert.ErrorIs(t, err, storage.ErrClaimNotFound, "claim row should be gone")

	_, err = svc.GetObject(ctx, storage.GetObjectOptions{Key: claim.Key})
	assert.ErrorIs(t, err, storage.ErrObjectNotFound, "object should be deleted")
}

func TestClaimSweeper_LeavesLiveClaim(t *testing.T) {
	ctx, _, svc, cs, _, _ := setupWorker(t)

	claim := &storage.UploadClaim{
		ID:        id.GenerateUUID(),
		Key:       "priv/live.bin",
		Bucket:    "memory",
		CreatedBy: "tester",
		ExpiresAt: timex.Now().AddHours(1), // future
		CreatedAt: timex.Now(),
	}
	require.NoError(t, cs.Create(ctx, claim))

	worker.NewClaimSweeper(svc, cs).Run(ctx)

	got, err := cs.Get(ctx, claim.ID)
	require.NoError(t, err)
	assert.Equal(t, claim.Key, got.Key, "non-expired claim must not be touched")
}

func TestClaimSweeper_ToleratesMissingObject(t *testing.T) {
	ctx, _, svc, cs, _, _ := setupWorker(t)

	// Note: no PutObject — the object never existed.
	claim := &storage.UploadClaim{
		ID:        id.GenerateUUID(),
		Key:       "priv/never-uploaded.bin",
		Bucket:    "memory",
		CreatedBy: "tester",
		ExpiresAt: timex.Now().AddHours(-1),
		CreatedAt: timex.Now(),
	}
	require.NoError(t, cs.Create(ctx, claim))

	worker.NewClaimSweeper(svc, cs).Run(ctx)

	_, err := cs.Get(ctx, claim.ID)
	assert.ErrorIs(t, err, storage.ErrClaimNotFound, "claim row should be removed even if object never existed")
}

func TestClaimSweeper_HandlesMultipartClaim(t *testing.T) {
	// Memory backend returns ErrCapabilityNotSupported for AbortMultipart;
	// the sweeper must tolerate that and still clean up.
	ctx, _, svc, cs, _, _ := setupWorker(t)

	putMemoryObject(t, svc, "priv/mp.bin")

	claim := &storage.UploadClaim{
		ID:        id.GenerateUUID(),
		Key:       "priv/mp.bin",
		UploadID:  "some-upload-id",
		Bucket:    "memory",
		CreatedBy: "tester",
		ExpiresAt: timex.Now().AddHours(-1),
		CreatedAt: timex.Now(),
	}
	require.NoError(t, cs.Create(ctx, claim))

	worker.NewClaimSweeper(svc, cs).Run(ctx)

	_, err := cs.Get(ctx, claim.ID)
	assert.ErrorIs(t, err, storage.ErrClaimNotFound)
}

func TestDeleteWorker_DeletesAndRemovesRow(t *testing.T) {
	ctx, db, svc, _, dq, pub := setupWorker(t)

	putMemoryObject(t, svc, "priv/del.bin")

	item := storage.PendingDelete{
		ID:            id.GenerateUUID(),
		Key:           "priv/del.bin",
		Reason:        storage.DeleteReasonReplaced,
		NextAttemptAt: timex.Now(),
		CreatedAt:     timex.Now(),
	}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, []storage.PendingDelete{item})
	}))

	worker.NewDeleteWorker(svc, dq, pub).Run(ctx)

	_, err := svc.GetObject(ctx, storage.GetObjectOptions{Key: item.Key})
	assert.ErrorIs(t, err, storage.ErrObjectNotFound)

	// Row should be gone (Done).
	leased, err := dq.Lease(ctx, timex.Now().AddHours(1), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, leased)
	assert.Empty(t, pub.events, "happy path should not emit dead-letter events")
}

func TestDeleteWorker_TreatsMissingObjectAsSuccess(t *testing.T) {
	ctx, db, svc, _, dq, pub := setupWorker(t)

	// No PutObject — object never existed but row scheduled (e.g. someone
	// else deleted it concurrently).
	item := storage.PendingDelete{
		ID:            id.GenerateUUID(),
		Key:           "priv/missing.bin",
		Reason:        storage.DeleteReasonDeleted,
		NextAttemptAt: timex.Now(),
		CreatedAt:     timex.Now(),
	}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, []storage.PendingDelete{item})
	}))

	worker.NewDeleteWorker(svc, dq, pub).Run(ctx)

	leased, err := dq.Lease(ctx, timex.Now().AddHours(1), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, leased, "row should be removed (Done) when object was already gone")
	assert.Empty(t, pub.events)
}

func TestDeleteWorker_DefersOnTransientFailure(t *testing.T) {
	ctx, db, baseSvc, _, dq, pub := setupWorker(t)

	failingSvc := &alwaysFailService{Service: baseSvc, err: errors.New("simulated transient failure")}

	item := storage.PendingDelete{
		ID:            id.GenerateUUID(),
		Key:           "priv/fail.bin",
		Reason:        storage.DeleteReasonReplaced,
		NextAttemptAt: timex.Now(),
		CreatedAt:     timex.Now(),
	}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, []storage.PendingDelete{item})
	}))

	worker.NewDeleteWorker(failingSvc, dq, pub).Run(ctx)

	// Row still exists but NextAttemptAt should be pushed into the future.
	// Leasing with a far-future "now" should return it with attempts=1.
	leased, err := dq.Lease(ctx, timex.Now().AddHours(2), 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, leased, 1)
	assert.Equal(t, 1, leased[0].Attempts, "attempts should be incremented")
	assert.Empty(t, pub.events, "transient failure must not emit dead-letter yet")
}

func TestDeleteWorker_DeadLettersAfterMaxAttempts(t *testing.T) {
	ctx, db, baseSvc, _, dq, pub := setupWorker(t)

	failingSvc := &alwaysFailService{Service: baseSvc, err: errors.New("permanent failure")}

	// Seed row with attempts already at the threshold so a single Run pushes it over.
	item := storage.PendingDelete{
		ID:            id.GenerateUUID(),
		Key:           "priv/dead.bin",
		Reason:        storage.DeleteReasonDeleted,
		Attempts:      11, // one more attempt and we hit max=12
		NextAttemptAt: timex.Now(),
		CreatedAt:     timex.Now(),
	}

	require.NoError(t, db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		return dq.Schedule(txCtx, tx, []storage.PendingDelete{item})
	}))

	worker.NewDeleteWorker(failingSvc, dq, pub).Run(ctx)

	// Row should still exist but parked far in the future.
	leased, err := dq.Lease(ctx, timex.Now().AddHours(24*365), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, leased, "dead-lettered row must not be visible within a year")

	// Confirm a dead-letter event was published.
	require.Len(t, pub.events, 1)
	dl, ok := pub.events[0].(*storage.DeleteDeadLetterEvent)
	require.True(t, ok, "event should be DeleteDeadLetterEvent")
	assert.Equal(t, item.ID, dl.PendingDeleteID)
	assert.Equal(t, item.Key, dl.FileKey)
	assert.Equal(t, storage.DeleteReasonDeleted, dl.Reason)
	assert.GreaterOrEqual(t, dl.Attempts, 12)
	assert.Contains(t, dl.LastError, "permanent failure")
}

// Compile-time assertion that the test stub satisfies storage.Service.
var _ storage.Service = (*alwaysFailService)(nil)
