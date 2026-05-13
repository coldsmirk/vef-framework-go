package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/storage/worker"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ── DeleteWorker-specific stubs ─────────────────────────────────────────

// AbortTrackingService wraps a real Service and counts AbortMultipart
// calls so tests can assert the worker invoked abort before delete. The
// extra InitMultipart / CompleteMultipart stubs exist solely to satisfy
// storage.Multipart so worker.NewDeleteWorker's type assertion picks
// this wrapper up as multipart-capable; they are never invoked by these
// tests.
type AbortTrackingService struct {
	storage.Service

	abortCount int
	abortErr   error
}

func (*AbortTrackingService) PartSize() int64   { return 0 }
func (*AbortTrackingService) MaxPartCount() int { return 0 }

func (*AbortTrackingService) InitMultipart(context.Context, storage.InitMultipartOptions) (*storage.MultipartSession, error) {
	return nil, errors.New("AbortTrackingService.InitMultipart: not expected in worker tests")
}

func (*AbortTrackingService) PutPart(context.Context, storage.PutPartOptions) (*storage.PartInfo, error) {
	return nil, errors.New("AbortTrackingService.PutPart: not expected in worker tests")
}

func (*AbortTrackingService) CompleteMultipart(context.Context, storage.CompleteMultipartOptions) (*storage.ObjectInfo, error) {
	return nil, errors.New("AbortTrackingService.CompleteMultipart: not expected in worker tests")
}

func (s *AbortTrackingService) AbortMultipart(context.Context, storage.AbortMultipartOptions) error {
	s.abortCount++

	return s.abortErr
}

// AlwaysFailService wraps a real Service but forces DeleteObject to fail,
// so we can drive the delete worker into its retry/dead-letter paths.
type AlwaysFailService struct {
	storage.Service

	err error
}

func (s *AlwaysFailService) DeleteObject(context.Context, storage.DeleteObjectOptions) error {
	return s.err
}

// ── TestDeleteWorker ────────────────────────────────────────────────────

func TestDeleteWorker(t *testing.T) {
	t.Run("DeletesRowAndEmitsFileDeletedEvent", func(t *testing.T) {
		env := setupWorker(t)

		putMemoryObject(t, env.Svc, "priv/del.bin")

		item := store.PendingDelete{
			ID:            id.GenerateUUID(),
			Key:           "priv/del.bin",
			Reason:        storage.DeleteReasonReplaced,
			NextAttemptAt: timex.Now(),
			CreatedAt:     timex.Now(),
		}

		require.NoError(t, env.DB.RunInTX(env.Ctx, func(txCtx context.Context, tx orm.DB) error {
			return env.DQ.Enqueue(txCtx, tx, []store.PendingDelete{item})
		}), "Pending delete should be scheduled inside the transaction")

		worker.NewDeleteWorker(env.Svc, env.DQ, env.Pub, env.Cfg).Run(env.Ctx)

		_, err := env.Svc.GetObject(env.Ctx, storage.GetObjectOptions{Key: item.Key})
		assert.ErrorIs(t, err, storage.ErrObjectNotFound, "Deleted object should no longer exist")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Pending delete lease should succeed")
		assert.Empty(t, leased, "Processed row should be removed")

		require.Len(t, env.Pub.events, 1, "Successful delete should emit one event")
		fd, ok := env.Pub.events[0].(*storage.FileDeletedEvent)
		require.True(t, ok, "Event should be FileDeletedEvent")
		assert.Equal(t, item.Key, fd.FileKey, "FileDeletedEvent should carry the deleted key")
		assert.Equal(t, storage.DeleteReasonReplaced, fd.Reason, "FileDeletedEvent should preserve the schedule reason")
	})

	t.Run("AbortsMultipartBeforeDelete", func(t *testing.T) {
		env := setupWorker(t)

		tracker := &AbortTrackingService{Service: env.Svc}
		putMemoryObject(t, env.Svc, "priv/mp-row.bin")

		item := store.PendingDelete{
			ID:            id.GenerateUUID(),
			Key:           "priv/mp-row.bin",
			UploadID:      "session-abc",
			Reason:        storage.DeleteReasonClaimExpired,
			NextAttemptAt: timex.Now(),
			CreatedAt:     timex.Now(),
		}

		require.NoError(t, env.DB.RunInTX(env.Ctx, func(txCtx context.Context, tx orm.DB) error {
			return env.DQ.Enqueue(txCtx, tx, []store.PendingDelete{item})
		}), "Multipart pending delete should be scheduled")

		worker.NewDeleteWorker(tracker, env.DQ, env.Pub, env.Cfg).Run(env.Ctx)

		assert.Equal(t, 1, tracker.abortCount, "Worker should abort the multipart session before deleting")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Pending delete lease should succeed")
		assert.Empty(t, leased, "Row should be marked done after abort + delete succeed")
	})

	t.Run("TreatsMissingObjectAsSuccess", func(t *testing.T) {
		env := setupWorker(t)

		// No PutObject — object never existed but row scheduled (e.g. someone
		// else deleted it concurrently).
		item := store.PendingDelete{
			ID:            id.GenerateUUID(),
			Key:           "priv/missing.bin",
			Reason:        storage.DeleteReasonDeleted,
			NextAttemptAt: timex.Now(),
			CreatedAt:     timex.Now(),
		}

		require.NoError(t, env.DB.RunInTX(env.Ctx, func(txCtx context.Context, tx orm.DB) error {
			return env.DQ.Enqueue(txCtx, tx, []store.PendingDelete{item})
		}), "Pending delete should be scheduled inside the transaction")

		worker.NewDeleteWorker(env.Svc, env.DQ, env.Pub, env.Cfg).Run(env.Ctx)

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Pending delete lease should succeed")
		assert.Empty(t, leased, "Row should be removed (Done) when object was already gone")
	})

	t.Run("DefersOnTransientFailure", func(t *testing.T) {
		env := setupWorker(t)

		failingSvc := &AlwaysFailService{Service: env.Svc, err: errors.New("simulated transient failure")}

		item := store.PendingDelete{
			ID:            id.GenerateUUID(),
			Key:           "priv/fail.bin",
			Reason:        storage.DeleteReasonReplaced,
			NextAttemptAt: timex.Now(),
			CreatedAt:     timex.Now(),
		}

		require.NoError(t, env.DB.RunInTX(env.Ctx, func(txCtx context.Context, tx orm.DB) error {
			return env.DQ.Enqueue(txCtx, tx, []store.PendingDelete{item})
		}), "Pending delete should be scheduled inside the transaction")

		worker.NewDeleteWorker(failingSvc, env.DQ, env.Pub, env.Cfg).Run(env.Ctx)

		// Row still exists but NextAttemptAt should be pushed into the future.
		// Leasing with a far-future "now" should return it with attempts=1.
		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(2), 10, time.Minute)
		require.NoError(t, err, "Pending delete lease after retry delay should succeed")
		require.Len(t, leased, 1, "Deferred row should become visible after retry delay")
		assert.Equal(t, 1, leased[0].Attempts, "Attempts should be incremented")
		assert.Empty(t, env.Pub.events, "Transient failure must not emit any event yet")
	})

	t.Run("DeadLettersAfterMaxAttempts", func(t *testing.T) {
		env := setupWorker(t)

		failingSvc := &AlwaysFailService{Service: env.Svc, err: errors.New("permanent failure")}

		// Seed row with attempts already at the threshold so a single Run pushes it over.
		item := store.PendingDelete{
			ID:            id.GenerateUUID(),
			Key:           "priv/dead.bin",
			Reason:        storage.DeleteReasonDeleted,
			Attempts:      config.DefaultDeleteMaxAttempts - 1,
			NextAttemptAt: timex.Now(),
			CreatedAt:     timex.Now(),
		}

		require.NoError(t, env.DB.RunInTX(env.Ctx, func(txCtx context.Context, tx orm.DB) error {
			return env.DQ.Enqueue(txCtx, tx, []store.PendingDelete{item})
		}), "Pending delete should be scheduled inside the transaction")

		worker.NewDeleteWorker(failingSvc, env.DQ, env.Pub, env.Cfg).Run(env.Ctx)

		// Row should still exist but parked far in the future.
		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(24*365), 10, time.Minute)
		require.NoError(t, err, "Pending delete lease should succeed")
		assert.Empty(t, leased, "Dead-lettered row must not be visible within a year")

		require.Len(t, env.Pub.events, 1, "Max attempts should emit one dead-letter event")
		dl, ok := env.Pub.events[0].(*storage.DeleteDeadLetterEvent)
		require.True(t, ok, "Event should be DeleteDeadLetterEvent")
		assert.Equal(t, item.ID, dl.PendingDeleteID, "Dead-letter event should reference the pending delete ID")
		assert.Equal(t, item.Key, dl.FileKey, "Dead-letter event should reference the object key")
		assert.Equal(t, storage.DeleteReasonDeleted, dl.Reason, "Dead-letter event should preserve the delete reason")
		assert.GreaterOrEqual(t, dl.Attempts, config.DefaultDeleteMaxAttempts, "Dead-letter event should report exhausted attempts")
		assert.Equal(t, "transient", dl.LastError, "Dead-letter event should carry the classified error category")
	})
}
