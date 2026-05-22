package worker

import (
	"context"
	"errors"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ClaimSweeper reaps expired upload claims after a grace window. Most
// expired claims are translated into PendingDelete rows and removed
// atomically in one transaction; if a multipart claim already has a
// finalized object, the sweeper recovers it to uploaded instead of
// deleting it. The actual backend delete work is performed
// asynchronously by DeleteWorker, which inherits retry/backoff/dead-
// letter from the queue.
type ClaimSweeper struct {
	db          orm.DB
	service     storage.Service
	claimStore  store.ClaimStore
	partStore   store.UploadPartStore
	deleteQueue store.DeleteQueue
	cfg         *config.StorageConfig
}

// NewClaimSweeper constructs a ClaimSweeper. db is required to wrap the
// schedule-and-delete pair in a single transaction.
func NewClaimSweeper(
	db orm.DB,
	service storage.Service,
	claimStore store.ClaimStore,
	partStore store.UploadPartStore,
	deleteQueue store.DeleteQueue,
	cfg *config.StorageConfig,
) *ClaimSweeper {
	return &ClaimSweeper{
		db:          db,
		service:     service,
		claimStore:  claimStore,
		partStore:   partStore,
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
	cutoff := timex.Now().Add(-s.cfg.EffectiveSweepInterval())

	err := s.db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		claims, err := s.claimStore.LockExpiredInTx(txCtx, tx, cutoff, limit)
		if err != nil {
			return err
		}

		if len(claims) == 0 {
			return nil
		}

		logger.Infof("Processing %d expired upload claim(s)", len(claims))

		now := timex.Now()
		items := make([]store.PendingDelete, 0, len(claims))
		ids := make([]string, 0, len(claims))

		for i := range claims {
			claim := &claims[i]

			recovered, err := s.recoverCompletedClaim(txCtx, tx, claim)
			if err != nil {
				return err
			}

			if recovered {
				continue
			}

			items = append(items, store.PendingDelete{
				ID:            id.GenerateUUID(),
				Key:           claim.Key,
				UploadID:      claim.UploadID,
				Reason:        storage.DeleteReasonClaimExpired,
				NextAttemptAt: now,
				CreatedAt:     now,
			})
			ids = append(ids, claim.ID)
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

func (s *ClaimSweeper) recoverCompletedClaim(ctx context.Context, tx orm.DB, claim *store.UploadClaim) (bool, error) {
	if !claim.IsMultipart() {
		return false, nil
	}

	info, err := s.service.StatObject(ctx, storage.StatObjectOptions{Key: claim.Key})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return false, nil
		}

		return false, err
	}

	if claim.Size > 0 && info.Size != claim.Size {
		return false, nil
	}

	if err := s.claimStore.MarkUploaded(ctx, tx, claim.ID); err != nil {
		return false, err
	}

	if err := s.partStore.DeleteByClaim(ctx, tx, claim.ID); err != nil {
		return false, err
	}

	logger.Infof("Recovered completed upload claim %s for object %s", claim.ID, claim.Key)

	return true, nil
}
