package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Backoff bounds for transient delete failures. These are intentionally
// not exposed in StorageConfig — tuning them requires understanding the
// exponential schedule, and operators can already cap retries via
// DeleteMaxAttempts.
const (
	deleteBaseBackoff = 30 * time.Second
	deleteMaxBackoff  = 1 * time.Hour

	// deadLetterPark pushes a parked row far enough into the future that
	// the next Lease will not pick it up. Operations remove or fix it
	// manually after consuming the dead-letter event.
	deadLetterPark = 100 * 365 * 24 * time.Hour
)

// DeleteWorker drains sys_storage_pending_delete: for each leased row it
// optionally aborts a multipart session (UploadID != "" and the backend
// implements storage.Multipart), deletes the underlying object, and
// either marks the row done or defers it with exponential backoff. Rows
// that exceed StorageConfig.DeleteMaxAttempts are parked indefinitely
// and a dead-letter event is published.
type DeleteWorker struct {
	service     storage.Service
	multipart   storage.Multipart // nil when the backend does not implement chunked uploads
	deleteQueue store.DeleteQueue
	publisher   event.Publisher
	cfg         *config.StorageConfig
}

// NewDeleteWorker constructs a DeleteWorker. The optional multipart
// capability is resolved once via a type assertion against the backend;
// processOne consults the resulting handle instead of probing the
// backend on every iteration.
func NewDeleteWorker(
	service storage.Service,
	deleteQueue store.DeleteQueue,
	publisher event.Publisher,
	cfg *config.StorageConfig,
) *DeleteWorker {
	w := &DeleteWorker{
		service:     service,
		deleteQueue: deleteQueue,
		publisher:   publisher,
		cfg:         cfg,
	}

	if mp, ok := service.(storage.Multipart); ok {
		w.multipart = mp
	}

	return w
}

// Run executes one drain cycle. Safe to invoke from a cron task.
func (w *DeleteWorker) Run(ctx context.Context) {
	batchSize := w.cfg.EffectiveDeleteBatchSize()
	leaseWindow := w.cfg.EffectiveDeleteLeaseWindow()

	leased, err := w.deleteQueue.Lease(ctx, timex.Now(), batchSize, leaseWindow)
	if err != nil {
		logger.Errorf("Failed to lease pending deletes: %v", err)

		return
	}

	if len(leased) == 0 {
		return
	}

	logger.Infof("Processing %d pending delete(s)", len(leased))

	concurrency := w.cfg.EffectiveDeleteConcurrency()
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup

	for i := range leased {
		item := &leased[i]

		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			w.processOne(ctx, item)
		})
	}

	wg.Wait()
}

// processOne handles a single leased pending-delete row. The work is
// always: abort the multipart session if the row carries one AND the
// backend implements storage.Multipart, then delete the object, then
// mark done. A row that claims to be multipart against a backend that
// no longer implements it (typically only possible after a backend
// swap) silently skips the abort and proceeds to object deletion — the
// session is unreachable through this service anyway. Transient errors
// trigger Defer with exponential backoff; the DeleteMaxAttempts budget
// gates dead-lettering.
func (w *DeleteWorker) processOne(ctx context.Context, item *store.PendingDelete) {
	if item.IsMultipart() && w.multipart != nil {
		if err := w.multipart.AbortMultipart(ctx, storage.AbortMultipartOptions{
			Key:      item.Key,
			UploadID: item.UploadID,
		}); err != nil {
			w.handleFailure(ctx, item, err)

			return
		}
	}

	err := w.service.DeleteObject(ctx, storage.DeleteObjectOptions{Key: item.Key})
	if err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		w.handleFailure(ctx, item, err)

		return
	}

	if doneErr := w.deleteQueue.Done(ctx, []string{item.ID}); doneErr != nil {
		logger.Errorf("Mark delete row %s done failed: %v", item.ID, doneErr)

		return
	}

	w.publisher.Publish(storage.NewFileDeletedEvent(item.Key, item.Reason))
}

// handleFailure decides whether a transient error should trigger a
// backoff retry or, once attempts have exhausted DeleteMaxAttempts, a
// dead-letter park.
func (w *DeleteWorker) handleFailure(ctx context.Context, item *store.PendingDelete, lastErr error) {
	maxAttempts := w.cfg.EffectiveDeleteMaxAttempts()
	nextAttempt := item.Attempts + 1

	if nextAttempt >= maxAttempts {
		w.parkDeadLetter(ctx, item, lastErr)

		return
	}

	backoff := computeBackoff(nextAttempt)
	nextAt := timex.DateTime(time.Now().Add(backoff))

	if deferErr := w.deleteQueue.Defer(ctx, item.ID, nextAt); deferErr != nil {
		logger.Errorf("Defer delete row %s failed: %v", item.ID, deferErr)

		return
	}

	logger.Warnf("Delete object %s failed (attempt %d/%d), retry in %s: %v",
		item.Key, nextAttempt, maxAttempts, backoff, lastErr)
}

func (w *DeleteWorker) parkDeadLetter(ctx context.Context, item *store.PendingDelete, lastErr error) {
	maxAttempts := w.cfg.EffectiveDeleteMaxAttempts()
	parkUntil := timex.DateTime(time.Now().Add(deadLetterPark))

	if deferErr := w.deleteQueue.Defer(ctx, item.ID, parkUntil); deferErr != nil {
		logger.Errorf("Park dead-letter row %s failed: %v", item.ID, deferErr)
	}

	logger.Errorf("Delete object %s reached max attempts (%d), parked as dead-letter: %v",
		item.Key, maxAttempts, lastErr)

	w.publisher.Publish(storage.NewDeleteDeadLetterEvent(
		item.ID,
		item.Key,
		item.Reason,
		item.Attempts+1,
		classifyDeleteError(lastErr),
	))
}

// computeBackoff returns 2^attempt * base, capped at deleteMaxBackoff.
func computeBackoff(attempt int) time.Duration {
	// 30s << 7 = 64 min already exceeds the 1h cap; clamping shift here
	// keeps the multiplication well within int64 bounds.
	shift := min(attempt, 7)

	d := deleteBaseBackoff << shift //nolint:gosec // shift bounded above
	if d > deleteMaxBackoff {
		return deleteMaxBackoff
	}

	return d
}

// classifyDeleteError returns a sanitized error category string for
// dead-letter events. The full error detail stays in server logs;
// events only carry the classification to avoid leaking backend
// internals to external subscribers.
func classifyDeleteError(err error) string {
	switch {
	case errors.Is(err, storage.ErrAccessDenied):
		return "access_denied"
	case errors.Is(err, storage.ErrBucketNotFound):
		return "bucket_not_found"
	case errors.Is(err, storage.ErrUploadSessionNotFound):
		return "session_not_found"
	default:
		return "transient"
	}
}
