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
func (s *ClaimSweeper) Run(ctx context.Context) {
	limit := s.cfg.EffectiveSweepBatchSize()

	claims, err := s.claimStore.ScanExpired(ctx, timex.Now(), limit)
	if err != nil {
		logger.Errorf("Failed to scan expired claims: %v", err)

		return
	}

	if len(claims) == 0 {
		return
	}

	logger.Infof("Sweeping %d expired upload claim(s) into delete queue", len(claims))

	if err := s.transferToDeleteQueue(ctx, claims); err != nil {
		logger.Errorf("Failed to transfer expired claims to delete queue: %v", err)
	}
}

// transferToDeleteQueue atomically enqueues a PendingDelete for every
// expired claim and removes the matching claim rows. Multipart claims
// carry their UploadID forward so DeleteWorker can abort the dangling
// session before deleting the (possibly partial) object.
//
// Enqueue (rather than the public Schedule(keys, reason) facade) is
// used here so the per-row UploadID survives into the queue.
func (s *ClaimSweeper) transferToDeleteQueue(ctx context.Context, claims []store.UploadClaim) error {
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

	return s.db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		if err := s.deleteQueue.Enqueue(txCtx, tx, items); err != nil {
			return err
		}

		return s.claimStore.DeleteByIDs(txCtx, tx, ids)
	})
}
