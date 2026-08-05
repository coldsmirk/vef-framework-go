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

// ClaimSweeper reaps expired upload claims after a grace window.
// Backend object checks run before the database transaction so remote
// storage latency never extends claim-row locks. The final transaction
// uses conditional updates/deletes so concurrent complete_upload calls
// win or lose cleanly without orphaning objects.
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

type sweepPlan struct {
	recover []store.UploadClaim
	remove  []store.UploadClaim
}

// Run executes one sweep cycle. Safe to invoke from a cron task.
// Errors at each stage (listing expired rows, per-claim backend probe,
// transactional cleanup) are logged; the next tick re-reads the
// expired set, so transient failures self-heal.
//
// Two independent passes over two disjoint claim populations: expired
// PENDING claims (uploads that never finished) and, when reclamation is
// configured, finalized claims nobody ever adopted.
func (s *ClaimSweeper) Run(ctx context.Context) {
	// Polling bookkeeping logs at Debug; failures keep their level.
	ctx = orm.WithQuietSQLLog(ctx)

	s.sweepExpired(ctx)
	s.reclaimOrphans(ctx)
}

// reclaimOrphans deletes uploads that materialized an object and were
// never adopted by a business transaction — the file a user uploaded and
// abandoned by closing the form. Nothing else reclaims them: the expiry
// pass deliberately skips finalized claims so a slow business save keeps
// its file, which leaves this population growing forever.
//
// Off unless StorageConfig.OrphanRetention is configured; see the field
// documentation for why deleting user data is opt-in.
//
// Every delete is gated on a compare-and-set against the same claim row
// Consume deletes, so a business save landing in this exact moment wins
// or loses cleanly — it can never end up referencing a reaped object.
func (s *ClaimSweeper) reclaimOrphans(ctx context.Context) {
	retention := s.cfg.OrphanRetention
	if retention <= 0 {
		return
	}

	cutoff := timex.Now().Add(-retention)

	claims, err := s.claimStore.ListUnadopted(ctx, cutoff, s.cfg.EffectiveSweepBatchSize())
	if err != nil {
		logger.Errorf("Failed to scan unadopted claims: %v", err)

		return
	}

	if len(claims) == 0 {
		return
	}

	logger.Infof("Reclaiming %d unadopted upload(s)", len(claims))

	err = s.db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
		now := timex.Now()
		items := make([]store.PendingDelete, 0, len(claims))

		for i := range claims {
			claim := claims[i]

			reclaimed, err := s.claimStore.DeleteIfUploadedBefore(txCtx, tx, claim, cutoff)
			if err != nil {
				return err
			}

			if !reclaimed {
				continue
			}

			items = append(items, store.PendingDelete{
				ID:            id.GenerateUUID(),
				Key:           claim.Key,
				Reason:        storage.DeleteReasonOrphaned,
				NextAttemptAt: now,
				CreatedAt:     now,
			})
		}

		return s.deleteQueue.Insert(txCtx, tx, items)
	})
	if err != nil {
		logger.Errorf("Failed to reclaim unadopted uploads: %v", err)
	}
}

func (s *ClaimSweeper) sweepExpired(ctx context.Context) {
	limit := s.cfg.EffectiveSweepBatchSize()
	cutoff := timex.Now().Add(-s.cfg.EffectiveSweepInterval())

	claims, err := s.claimStore.ListExpired(ctx, cutoff, limit)
	if err != nil {
		logger.Errorf("Failed to scan expired claims: %v", err)

		return
	}

	if len(claims) == 0 {
		return
	}

	logger.Infof("Processing %d expired upload claim(s)", len(claims))

	plan := s.planClaims(ctx, claims)
	if len(plan.recover) == 0 && len(plan.remove) == 0 {
		return
	}

	err = s.db.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
		now := timex.Now()
		items := make([]store.PendingDelete, 0, len(plan.remove))

		for i := range plan.recover {
			claim := plan.recover[i]

			recovered, err := s.claimStore.MarkUploadedIfPendingExpired(txCtx, tx, claim, cutoff)
			if err != nil {
				return err
			}

			if !recovered {
				continue
			}

			if err := s.partStore.DeleteByClaim(txCtx, tx, claim.ID); err != nil {
				return err
			}

			logger.Infof("Recovered completed upload claim %s for object %s", claim.ID, claim.Key)
		}

		for i := range plan.remove {
			claim := plan.remove[i]

			deleted, err := s.claimStore.DeleteIfPendingExpired(txCtx, tx, claim, cutoff)
			if err != nil {
				return err
			}

			if !deleted {
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
		}

		return s.deleteQueue.Insert(txCtx, tx, items)
	})
	if err != nil {
		logger.Errorf("Failed to sweep expired claims: %v", err)
	}
}

func (s *ClaimSweeper) planClaims(ctx context.Context, claims []store.UploadClaim) sweepPlan {
	plan := sweepPlan{
		recover: make([]store.UploadClaim, 0, len(claims)),
		remove:  make([]store.UploadClaim, 0, len(claims)),
	}

	for i := range claims {
		claim := claims[i]

		shouldRecover, err := s.canRecoverCompletedClaim(ctx, &claim)
		if err != nil {
			logger.Warnf("Check expired claim %s object %s failed: %v", claim.ID, claim.Key, err)

			continue
		}

		if shouldRecover {
			plan.recover = append(plan.recover, claim)
		} else {
			plan.remove = append(plan.remove, claim)
		}
	}

	return plan
}

func (s *ClaimSweeper) canRecoverCompletedClaim(ctx context.Context, claim *store.UploadClaim) (bool, error) {
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

	return true, nil
}
