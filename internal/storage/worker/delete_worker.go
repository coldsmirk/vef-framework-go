package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Tunables for the delete worker. Hard-coded for now; lift into config if
// operations want runtime tuning.
const (
	deleteBatchSize   = 100
	deleteConcurrency = 8
	deleteLeaseWindow = 5 * time.Minute
	deleteBaseBackoff = 30 * time.Second
	deleteMaxBackoff  = 1 * time.Hour
	deleteMaxAttempts = 12

	// deadLetterPark pushes a parked row far enough into the future that the
	// next Lease will not pick it up. Operations remove or fix it manually.
	deadLetterPark = 100 * 365 * 24 * time.Hour
)

// DeleteWorker drains storage_pending_deletes: it leases due rows, deletes
// the underlying objects, and either marks the rows done or defers them
// with exponential backoff. Rows that exceed deleteMaxAttempts are parked
// and a dead-letter event is published.
type DeleteWorker struct {
	service     storage.Service
	deleteQueue storage.DeleteQueue
	publisher   event.Publisher
}

// NewDeleteWorker constructs a DeleteWorker.
func NewDeleteWorker(service storage.Service, deleteQueue storage.DeleteQueue, publisher event.Publisher) *DeleteWorker {
	return &DeleteWorker{
		service:     service,
		deleteQueue: deleteQueue,
		publisher:   publisher,
	}
}

// Run executes one drain cycle. Safe to invoke from a cron task.
func (w *DeleteWorker) Run(ctx context.Context) {
	leased, err := w.deleteQueue.Lease(ctx, timex.Now(), deleteBatchSize, deleteLeaseWindow)
	if err != nil {
		logger.Errorf("Failed to lease pending deletes: %v", err)

		return
	}

	if len(leased) == 0 {
		return
	}

	logger.Infof("Processing %d pending delete(s)", len(leased))

	sem := make(chan struct{}, deleteConcurrency)

	var wg sync.WaitGroup

	for i := range leased {
		item := &leased[i]

		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			w.processOne(ctx, item)
		}()
	}

	wg.Wait()
}

func (w *DeleteWorker) processOne(ctx context.Context, item *storage.PendingDelete) {
	err := w.service.DeleteObject(ctx, storage.DeleteObjectOptions{Key: item.Key})
	if err == nil || errors.Is(err, storage.ErrObjectNotFound) {
		if doneErr := w.deleteQueue.Done(ctx, []string{item.ID}); doneErr != nil {
			logger.Errorf("Mark delete row %s done failed: %v", item.ID, doneErr)
		}

		return
	}

	nextAttempt := item.Attempts + 1

	if nextAttempt >= deleteMaxAttempts {
		w.parkDeadLetter(ctx, item, err)

		return
	}

	backoff := computeBackoff(nextAttempt)
	nextAt := timex.DateTime(time.Now().Add(backoff))

	if deferErr := w.deleteQueue.Defer(ctx, item.ID, nextAt); deferErr != nil {
		logger.Errorf("Defer delete row %s failed: %v", item.ID, deferErr)

		return
	}

	logger.Warnf("Delete object %s failed (attempt %d/%d), retry in %s: %v",
		item.Key, nextAttempt, deleteMaxAttempts, backoff, err)
}

func (w *DeleteWorker) parkDeadLetter(ctx context.Context, item *storage.PendingDelete, lastErr error) {
	parkUntil := timex.DateTime(time.Now().Add(deadLetterPark))

	if deferErr := w.deleteQueue.Defer(ctx, item.ID, parkUntil); deferErr != nil {
		logger.Errorf("Park dead-letter row %s failed: %v", item.ID, deferErr)
	}

	logger.Errorf("Delete object %s reached max attempts (%d), parked as dead-letter: %v",
		item.Key, deleteMaxAttempts, lastErr)

	w.publisher.Publish(storage.NewDeleteDeadLetterEvent(
		item.ID,
		item.Key,
		item.Reason,
		item.Attempts+1,
		lastErr.Error(),
	))
}

// computeBackoff returns 2^attempt * base, capped at deleteMaxBackoff.
func computeBackoff(attempt int) time.Duration {
	shift := attempt
	if shift > 30 {
		shift = 30 // prevent int overflow
	}

	d := deleteBaseBackoff << shift //nolint:gosec // shift bounded above
	if d <= 0 || d > deleteMaxBackoff {
		return deleteMaxBackoff
	}

	return d
}
