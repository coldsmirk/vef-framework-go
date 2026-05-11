package worker

import (
	"context"
	"errors"

	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// claimSweepBatchSize bounds how many expired claims one tick processes.
const claimSweepBatchSize = 200

// ClaimSweeper reaps expired upload claims: it aborts any associated
// multipart session, deletes the partially-uploaded object (if present),
// and removes the claim row. Failures on either side are logged and left
// for the next tick to retry — the 24h claim TTL provides ample slack.
type ClaimSweeper struct {
	service    storage.Service
	claimStore storage.ClaimStore
}

// NewClaimSweeper constructs a ClaimSweeper bound to a backend Service and
// the claim store.
func NewClaimSweeper(service storage.Service, claimStore storage.ClaimStore) *ClaimSweeper {
	return &ClaimSweeper{
		service:    service,
		claimStore: claimStore,
	}
}

// Run executes one sweep cycle. Safe to invoke from a cron task.
func (s *ClaimSweeper) Run(ctx context.Context) {
	claims, err := s.claimStore.ScanExpired(ctx, timex.Now(), claimSweepBatchSize)
	if err != nil {
		logger.Errorf("Failed to scan expired claims: %v", err)

		return
	}

	if len(claims) == 0 {
		return
	}

	logger.Infof("Sweeping %d expired upload claim(s)", len(claims))

	for i := range claims {
		s.cleanupClaim(ctx, &claims[i])
	}
}

func (s *ClaimSweeper) cleanupClaim(ctx context.Context, claim *storage.UploadClaim) {
	if claim.IsMultipart() {
		if err := s.service.AbortMultipart(ctx, storage.AbortMultipartOptions{
			Key:      claim.Key,
			UploadID: claim.UploadID,
		}); err != nil && !errors.Is(err, storage.ErrCapabilityNotSupported) {
			// Log but proceed — the object delete below may still succeed.
			logger.Warnf("Abort multipart for claim %s (key=%s) failed: %v", claim.ID, claim.Key, err)
		}
	}

	if err := s.service.DeleteObject(ctx, storage.DeleteObjectOptions{Key: claim.Key}); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		// Leave the claim row in place; next tick will retry.
		logger.Warnf("Delete object %s for claim %s failed: %v", claim.Key, claim.ID, err)

		return
	}

	if err := s.claimStore.DeleteByID(ctx, claim.ID); err != nil {
		logger.Errorf("Delete claim row %s failed: %v", claim.ID, err)
	}
}
