package inbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/event/inbox"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func setupInbox(t *testing.T) *inbox.DefaultRepository {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)
	require.NoError(t, inbox.Migrate(ctx, db, config.SQLite), "inbox migration should succeed")

	return inbox.NewRepository(db)
}

func TestInboxTryInsertFirstDeliveryAcquires(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	ok, err := repo.TryInsert(ctx, "consumer-a", "evt-1")
	require.NoError(t, err, "First insert should not error")
	require.True(t, ok, "First insert should acquire the slot")
}

func TestInboxTryInsertDuplicateIsSkipped(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	ok1, err := repo.TryInsert(ctx, "consumer-a", "evt-2")
	require.NoError(t, err)
	require.True(t, ok1, "First delivery should acquire")

	ok2, err := repo.TryInsert(ctx, "consumer-a", "evt-2")
	require.NoError(t, err, "Duplicate insert should not surface as an error")
	require.False(t, ok2, "Second delivery should report duplicate")
}

func TestInboxTryInsertDifferentGroupsAreIndependent(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	ok1, err := repo.TryInsert(ctx, "consumer-a", "evt-3")
	require.NoError(t, err)
	require.True(t, ok1)

	ok2, err := repo.TryInsert(ctx, "consumer-b", "evt-3")
	require.NoError(t, err)
	require.True(t, ok2, "Same event id under a different group must acquire independently")
}

func TestInboxDeleteOlderThanRemovesStaleRows(t *testing.T) {
	ctx := context.Background()
	repo := setupInbox(t)

	_, err := repo.TryInsert(ctx, "consumer-a", "evt-old")
	require.NoError(t, err)
	_, err = repo.TryInsert(ctx, "consumer-a", "evt-fresh")
	require.NoError(t, err)

	future := timex.Now().Add(time.Hour)
	deleted, err := repo.DeleteOlderThan(ctx, future)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted, "Cutoff in the future should delete every record")

	past := timex.Now().Add(-time.Hour)
	deleted, err = repo.DeleteOlderThan(ctx, past)
	require.NoError(t, err)
	require.EqualValues(t, 0, deleted, "Cutoff in the past should retain every record")
}
