package worker

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ClaimSweeper reaps expired upload claims by translating each into a
// PendingDelete row and removing the claim entry — both atomically in
// one transaction. The actual backend work (abort multipart, delete
// object) is performed asynchronously by DeleteWorker, which inherits
// retry/backoff/dead-letter from the queue. ClaimSweeper itself is a
// pure metadata cleaner; if a single batch fails to commit, no rows
// are dropped and the next tick re-attempts the same set.
type ClaimSweeper struct {
	db          orm.DB
	claimStore  store.ClaimStore
	deleteQueue store.DeleteQueue
	cfg         *config.StorageConfig
}

// NewClaimSweeper constructs a ClaimSweeper. db is required to wrap the
// schedule-and-delete pair in a single transaction.
func NewClaimSweeper(
	db orm.DB,
	claimStore store.ClaimStore,
	deleteQueue store.DeleteQueue,
	cfg *config.StorageConfig,
) *ClaimSweeper {
	return &ClaimSweeper{
		db:          db,
		claimStore:  claimStore,
		deleteQueue: deleteQueue,
		cfg:         cfg,
	}
}

// Run executes one sweep cycle. Safe to invoke from a cron task. Logs and
// returns on any error; the next tick will pick up the same expired set.
//
// Everything happens inside a single transaction: LockExpiredInTx takes
// FOR UPDATE SKIP LOCKED on the candidate rows, then Enqueue + DeleteByIDs
// commit against that locked view. This eliminates the previous
// TOCTOU window where a concurrent complete_upload could flip a claim
// from pending to uploaded between scan and delete — under the old
// flow Enqueue would queue a deletion for a now-live object and
// DeleteByIDs would remove its uploaded claim row, orphaning the
// object and breaking the eventual ConsumeMany. With the lock, the
// concurrent MarkUploaded blocks until commit and then sees the row
// gone (its own UPDATE matches 0 rows, returning ErrClaimNotFound,
// which complete_upload surfaces as a normal failure).
func (s *ClaimSweeper) Run(ctx context.Context) {
	limit := s.cfg.EffectiveSweepBatchSize()

	err := s.db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		claims, err := s.claimStore.LockExpiredInTx(txCtx, tx, timex.Now(), limit)
		if err != nil {
			return err
		}

		if len(claims) == 0 {
			return nil
		}

		logger.Infof("Sweeping %d expired upload claim(s) into delete queue", len(claims))

		now := timex.Now()
		items := make([]store.PendingDelete, len(claims))
		ids := make([]string, len(claims))

		for i := range claims {
			claim := &claims[i]

			items[i] = store.PendingDelete{
				ID:            id.GenerateUUID(),
				Key:           claim.Key,
				UploadID:      claim.UploadID,
				Reason:        storage.DeleteReasonClaimExpired,
				NextAttemptAt: now,
				CreatedAt:     now,
			}
			ids[i] = claim.ID
		}

		if err := s.deleteQueue.Enqueue(txCtx, tx, items); err != nil {
			return err
		}

		return s.claimStore.DeleteByIDs(txCtx, tx, ids)
	})
	if err != nil {
		logger.Errorf("Failed to sweep expired claims: %v", err)
	}
}
