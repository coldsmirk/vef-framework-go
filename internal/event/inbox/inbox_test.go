package inbox_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/inbox"
	iinbox "github.com/coldsmirk/vef-framework-go/internal/event/inbox"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func setupInbox(t *testing.T) *iinbox.DefaultRepository {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)
	require.NoError(t, iinbox.Migrate(ctx, db, config.SQLite), "inbox migration should succeed")

	return iinbox.NewRepository(db)
}

func TestInboxAcquireFirstDeliveryAcquires(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	got, lockID, err := repo.Acquire(ctx, "consumer-a", "evt-1", timex.Now().Add(time.Minute))
	require.NoError(t, err, "First acquire should not error")
	require.Equal(t, inbox.AcquireResultAcquired, got, "First acquire should claim the delivery")
	require.NotEmpty(t, lockID, "Acquired delivery should return a lock id")
}

func TestInboxAcquireCompletedDuplicateIsSkipped(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	lockUntil := timex.Now().Add(time.Minute)
	got, lockID, err := repo.Acquire(ctx, "consumer-a", "evt-2", lockUntil)
	require.NoError(t, err, "First delivery acquire should not error")
	require.Equal(t, inbox.AcquireResultAcquired, got, "First delivery should acquire")
	require.NotEmpty(t, lockID, "Acquired delivery should return a lock id")
	require.NoError(t, repo.MarkCompleted(ctx, "consumer-a", "evt-2", lockID), "Completed delivery should be marked")

	got, lockID, err = repo.Acquire(ctx, "consumer-a", "evt-2", timex.Now().Add(time.Minute))
	require.NoError(t, err, "Duplicate acquire should not surface as an error")
	require.Equal(t, inbox.AcquireResultCompleted, got, "Completed duplicate should be skipped")
	require.Empty(t, lockID, "Completed duplicate should not return a lock id")
}

func TestInboxAcquireActiveDuplicateIsInProgress(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	got, lockID, err := repo.Acquire(ctx, "consumer-a", "evt-active", timex.Now().Add(time.Minute))
	require.NoError(t, err, "First active delivery acquire should not error")
	require.Equal(t, inbox.AcquireResultAcquired, got, "First delivery should acquire")
	require.NotEmpty(t, lockID, "Acquired delivery should return a lock id")

	got, lockID, err = repo.Acquire(ctx, "consumer-a", "evt-active", timex.Now().Add(time.Minute))
	require.NoError(t, err, "Active duplicate should not surface as a repository error")
	require.Equal(t, inbox.AcquireResultInProgress, got, "Active duplicate should remain retryable")
	require.Empty(t, lockID, "Active duplicate should not return a lock id")
}

func TestInboxReleaseAllowsRetry(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	lockUntil := timex.Now().Add(time.Minute)
	got, lockID, err := repo.Acquire(ctx, "consumer-a", "evt-retry", lockUntil)
	require.NoError(t, err, "First retry delivery acquire should not error")
	require.Equal(t, inbox.AcquireResultAcquired, got, "First delivery should acquire")
	require.NotEmpty(t, lockID, "Acquired delivery should return a lock id")
	require.NoError(t, repo.Release(ctx, "consumer-a", "evt-retry", lockID), "Failed delivery should release the claim")

	got, lockID, err = repo.Acquire(ctx, "consumer-a", "evt-retry", timex.Now().Add(time.Minute))
	require.NoError(t, err, "Retry acquire should not error")
	require.Equal(t, inbox.AcquireResultAcquired, got, "Released delivery should be acquired again")
	require.NotEmpty(t, lockID, "Retry acquire should return a fresh lock id")
}

func TestInboxAcquireDifferentGroupsAreIndependent(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	got, lockID, err := repo.Acquire(ctx, "consumer-a", "evt-3", timex.Now().Add(time.Minute))
	require.NoError(t, err, "First group acquire should not error")
	require.Equal(t, inbox.AcquireResultAcquired, got, "First group should acquire")
	require.NotEmpty(t, lockID, "First group should receive a lock id")

	got, lockID, err = repo.Acquire(ctx, "consumer-b", "evt-3", timex.Now().Add(time.Minute))
	require.NoError(t, err, "Second group acquire should not error")
	require.Equal(t, inbox.AcquireResultAcquired, got, "Same event id under a different group must acquire independently")
	require.NotEmpty(t, lockID, "Second group should receive a lock id")
}

func TestInboxAcquireExpiredClaimUsesNewLockID(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	got, oldLockID, err := repo.Acquire(ctx, "consumer-a", "evt-expired", timex.Now().Add(-time.Minute))
	require.NoError(t, err, "Expired first claim should still acquire")
	require.Equal(t, inbox.AcquireResultAcquired, got, "First delivery should acquire")
	require.NotEmpty(t, oldLockID, "First delivery should receive a lock id")

	got, newLockID, err := repo.Acquire(ctx, "consumer-a", "evt-expired", timex.Now().Add(time.Minute))
	require.NoError(t, err, "Expired duplicate should be claimable")
	require.Equal(t, inbox.AcquireResultAcquired, got, "Expired claim should be re-acquired")
	require.NotEmpty(t, newLockID, "Re-acquired delivery should receive a lock id")
	require.NotEqual(t, oldLockID, newLockID, "Re-acquired delivery should use a different lock id")

	err = repo.MarkCompleted(ctx, "consumer-a", "evt-expired", oldLockID)
	require.ErrorIs(t, err, inbox.ErrLockLost, "Old lock owner should not complete a re-acquired delivery")

	require.NoError(t, repo.MarkCompleted(ctx, "consumer-a", "evt-expired", newLockID), "Current lock owner should complete")
}

func TestInboxAcquireConcurrentDuplicateHasSingleOwner(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	const workers = 2

	results := make(chan inbox.AcquireResult, workers)
	lockIDs := make(chan string, workers)
	errs := make(chan error, workers)
	start := make(chan struct{})

	var wg sync.WaitGroup

	for range workers {
		wg.Go(func() {
			<-start

			got, lockID, err := repo.Acquire(ctx, "consumer-a", "evt-concurrent", timex.Now().Add(time.Minute))
			results <- got

			lockIDs <- lockID

			errs <- err
		})
	}

	close(start)
	wg.Wait()
	close(results)
	close(lockIDs)
	close(errs)

	for err := range errs {
		require.NoError(t, err, "Concurrent acquire should not return repository errors")
	}

	acquired := 0

	inProgress := 0
	for got := range results {
		switch got {
		case inbox.AcquireResultAcquired:
			acquired++
		case inbox.AcquireResultInProgress:
			inProgress++
		default:
			t.Fatalf("unexpected acquire result: %s", got)
		}
	}

	nonEmptyLocks := 0
	for lockID := range lockIDs {
		if lockID != "" {
			nonEmptyLocks++
		}
	}

	require.Equal(t, 1, acquired, "Exactly one concurrent acquire should own the delivery")
	require.Equal(t, 1, inProgress, "Exactly one concurrent acquire should observe active processing")
	require.Equal(t, 1, nonEmptyLocks, "Only the owner should receive a lock id")
}

func TestInboxDeleteOlderThanRemovesStaleRows(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	oldLockUntil := timex.Now().Add(time.Minute)
	_, oldLockID, err := repo.Acquire(ctx, "consumer-a", "evt-old", oldLockUntil)
	require.NoError(t, err, "Old delivery acquire should not error")
	require.NoError(t, repo.MarkCompleted(ctx, "consumer-a", "evt-old", oldLockID), "Old delivery should be completed")

	freshLockUntil := timex.Now().Add(time.Minute)
	_, freshLockID, err := repo.Acquire(ctx, "consumer-a", "evt-fresh", freshLockUntil)
	require.NoError(t, err, "Fresh delivery acquire should not error")
	require.NoError(t, repo.MarkCompleted(ctx, "consumer-a", "evt-fresh", freshLockID), "Fresh delivery should be completed")

	future := timex.Now().Add(time.Hour)
	deleted, err := repo.DeleteOlderThan(ctx, future)
	require.NoError(t, err, "Future cleanup cutoff should not error")
	require.EqualValues(t, 2, deleted, "Cutoff in the future should delete every record")

	past := timex.Now().Add(-time.Hour)
	deleted, err = repo.DeleteOlderThan(ctx, past)
	require.NoError(t, err, "Past cleanup cutoff should not error")
	require.EqualValues(t, 0, deleted, "Cutoff in the past should retain every record")
}
