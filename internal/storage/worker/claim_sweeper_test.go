package worker_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/memory"
	"github.com/coldsmirk/vef-framework-go/internal/storage/migration"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/internal/storage/worker"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ── shared test infrastructure ──────────────────────────────────────────
//
// Lives at the top of claim_sweeper_test.go because that is where the
// shared bundle is first introduced. delete_worker_test.go reuses these
// directly via the shared package.

// TestEnv bundles the dependencies every worker test needs. Returned
// from setupWorker so individual cases can pull only the fields they
// touch without a long positional unpack.
type TestEnv struct {
	Ctx context.Context
	DB  orm.DB
	Svc storage.Service
	CS  store.ClaimStore
	DQ  store.DeleteQueue
	Pub *CapturePublisher
	Cfg *config.StorageConfig
}

// CapturePublisher records all published events for assertion.
type CapturePublisher struct {
	events []event.Event
}

func (p *CapturePublisher) Publish(evt event.Event) {
	p.events = append(p.events, evt)
}

// newTestStorageConfig returns a minimal StorageConfig for worker tests.
// All zero values fall back to package defaults via the Effective*
// accessors, so the workers behave as in production.
func newTestStorageConfig() *config.StorageConfig {
	return &config.StorageConfig{}
}

func setupWorker(t *testing.T) *TestEnv {
	t.Helper()

	ctx := context.Background()
	db := testx.NewTestDB(t)
	require.NoError(t, migration.Migrate(ctx, db, config.SQLite), "Storage migration should succeed")

	return &TestEnv{
		Ctx: ctx,
		DB:  db,
		Svc: memory.New(),
		CS:  store.NewClaimStore(db),
		DQ:  store.NewDeleteQueue(db),
		Pub: &CapturePublisher{},
		Cfg: newTestStorageConfig(),
	}
}

func putMemoryObject(t *testing.T, svc storage.Service, key string) {
	t.Helper()

	_, err := svc.PutObject(context.Background(), storage.PutObjectOptions{
		Key:    key,
		Reader: bytes.NewReader([]byte("payload")),
		Size:   7,
	})
	require.NoError(t, err, "Memory object setup should succeed")
}

// ── TestClaimSweeper ────────────────────────────────────────────────────

func TestClaimSweeper(t *testing.T) {
	t.Run("MovesExpiredClaimToDeleteQueue", func(t *testing.T) {
		env := setupWorker(t)

		claim := &store.UploadClaim{
			ID:        id.GenerateUUID(),
			Key:       "priv/expired.bin",
			CreatedBy: "tester",
			ExpiresAt: timex.Now().AddHours(-1),
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Expired claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.CS, env.DQ, env.Cfg).Run(env.Ctx)

		_, err := env.CS.Get(env.Ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Claim row should be removed by the sweeper")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		require.Len(t, leased, 1, "Sweeper should enqueue exactly one PendingDelete row")
		assert.Equal(t, claim.Key, leased[0].Key, "Enqueued row should reference the expired claim's object key")
		assert.Equal(t, storage.DeleteReasonClaimExpired, leased[0].Reason, "Enqueued row should carry the claim_expired reason")
		assert.Empty(t, leased[0].UploadID, "Direct claim should not carry a multipart upload ID forward")
	})

	t.Run("ForwardsUploadIDForMultipartClaim", func(t *testing.T) {
		env := setupWorker(t)

		claim := &store.UploadClaim{
			ID:        id.GenerateUUID(),
			Key:       "priv/mp.bin",
			UploadID:  "session-xyz",
			CreatedBy: "tester",
			ExpiresAt: timex.Now().AddHours(-1),
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Expired multipart claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.CS, env.DQ, env.Cfg).Run(env.Ctx)

		_, err := env.CS.Get(env.Ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Expired multipart claim row should be removed")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		require.Len(t, leased, 1, "Sweeper should enqueue exactly one PendingDelete row")
		assert.Equal(t, "session-xyz", leased[0].UploadID, "UploadID should be forwarded so the worker can abort the dangling session")
	})

	t.Run("LeavesLiveClaim", func(t *testing.T) {
		env := setupWorker(t)

		claim := &store.UploadClaim{
			ID:        id.GenerateUUID(),
			Key:       "priv/live.bin",
			CreatedBy: "tester",
			ExpiresAt: timex.Now().AddHours(1), // future
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Live claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.CS, env.DQ, env.Cfg).Run(env.Ctx)

		got, err := env.CS.Get(env.Ctx, claim.ID)
		require.NoError(t, err, "Live claim should remain queryable")
		assert.Equal(t, claim.Key, got.Key, "Non-expired claim must not be touched")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Live claim must not be enqueued for deletion")
	})

	t.Run("NoOpOnEmpty", func(t *testing.T) {
		env := setupWorker(t)

		// Nothing scheduled; sweeper should not error.
		worker.NewClaimSweeper(env.DB, env.CS, env.DQ, env.Cfg).Run(env.Ctx)

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Empty sweep should not enqueue anything")
	})
}
