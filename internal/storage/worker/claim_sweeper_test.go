package worker_test

import (
	"bytes"
	"context"
	"errors"
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
	PS  store.UploadPartStore
	DQ  store.DeleteQueue
	FS  store.FileStore
	Pub *CaptureBus
	Cfg *config.StorageConfig
}

// CapturedPublish is one Publish invocation: the event payload and the
// raw PublishOption count. The option count lets tests assert that
// callers forwarded transactional options (event.WithTx) — PublishConfig.Tx
// itself folds nil-WithTx and stripped-WithTx to the same value, so
// counting raw options is the only signal a unit test can use to tell
// "Publish carried WithTx" from "Publish carried nothing".
type CapturedPublish struct {
	Event   event.Event
	OptsLen int
}

// CaptureBus records all published events for assertion. It
// implements event.Bus so worker constructors that expect a Bus
// can accept it directly.
type CaptureBus struct {
	events []event.Event
	calls  []CapturedPublish
}

type StatHookService struct {
	storage.Service

	afterStat func(context.Context, storage.StatObjectOptions, *storage.ObjectInfo, error)
}

func (s *StatHookService) StatObject(ctx context.Context, opts storage.StatObjectOptions) (*storage.ObjectInfo, error) {
	info, err := s.Service.StatObject(ctx, opts)
	if s.afterStat != nil {
		s.afterStat(ctx, opts, info, err)
	}

	return info, err
}

// Publish implements event.Bus.
func (p *CaptureBus) Publish(_ context.Context, evt event.Event, opts ...event.PublishOption) error {
	p.events = append(p.events, evt)
	p.calls = append(p.calls, CapturedPublish{Event: evt, OptsLen: len(opts)})

	return nil
}

// PublishBatch implements event.Bus.
func (p *CaptureBus) PublishBatch(_ context.Context, evts []event.Event, opts ...event.PublishOption) error {
	for _, e := range evts {
		p.events = append(p.events, e)
		p.calls = append(p.calls, CapturedPublish{Event: e, OptsLen: len(opts)})
	}

	return nil
}

// Subscribe implements event.Bus with a no-op unsubscribe.
func (*CaptureBus) Subscribe(string, event.Handler, ...event.SubscribeOption) (event.Unsubscribe, error) {
	return func() {}, nil
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

	files := store.NewFileStore(db)

	return &TestEnv{
		Ctx: ctx,
		DB:  db,
		Svc: memory.New(),
		CS:  store.NewClaimStore(db, files),
		PS:  store.NewUploadPartStore(db),
		DQ:  store.NewDeleteQueue(db),
		FS:  files,
		Pub: &CaptureBus{},
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
			Status:    store.ClaimStatusPending,
			ExpiresAt: timex.Now().Add(-2 * config.DefaultSweepInterval),
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Expired claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

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
			Status:    store.ClaimStatusPending,
			ExpiresAt: timex.Now().Add(-2 * config.DefaultSweepInterval),
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Expired multipart claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		_, err := env.CS.Get(env.Ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound, "Expired multipart claim row should be removed")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		require.Len(t, leased, 1, "Sweeper should enqueue exactly one PendingDelete row")
		assert.Equal(t, "session-xyz", leased[0].UploadID, "UploadID should be forwarded so the worker can abort the dangling session")
	})

	t.Run("RecoversCompletedMultipartClaim", func(t *testing.T) {
		env := setupWorker(t)

		claim := &store.UploadClaim{
			ID:               id.GenerateUUID(),
			Key:              "priv/recovered.bin",
			UploadID:         "session-recovered",
			Size:             7,
			OriginalFilename: "恢复的报告.bin",
			CreatedBy:        "tester",
			Status:           store.ClaimStatusPending,
			ExpiresAt:        timex.Now().Add(-2 * config.DefaultSweepInterval),
			CreatedAt:        timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Expired multipart claim creation should succeed")
		putMemoryObject(t, env.Svc, claim.Key)

		require.NoError(t, env.DB.RunInTx(env.Ctx, func(txCtx context.Context, tx orm.DB) error {
			return env.PS.Upsert(txCtx, tx, &store.UploadPart{
				ID:         id.GenerateUUID(),
				ClaimID:    claim.ID,
				PartNumber: 1,
				ETag:       "stale-etag",
				Size:       7,
				CreatedAt:  timex.Now(),
			})
		}), "Upload part row should be seeded")

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		got, err := env.CS.Get(env.Ctx, claim.ID)
		require.NoError(t, err, "Recovered claim should remain queryable")
		assert.Equal(t, store.ClaimStatusUploaded, got.Status, "Sweeper should mark the already-materialized object as uploaded")

		parts, err := env.PS.ListByClaim(env.Ctx, claim.ID)
		require.NoError(t, err, "Part lookup should succeed")
		assert.Empty(t, parts, "Recovered claim should no longer retain stale part rows")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Recovered claim must not be enqueued for deletion")

		// Recovery is the second of the two paths that finalize an
		// upload, so it owes the registry the same record complete_upload
		// writes — otherwise a file recovered here would have no filename.
		recorded, err := env.FS.Lookup(env.Ctx, []string{claim.Key})
		require.NoError(t, err, "Registry lookup should succeed")
		require.Len(t, recorded, 1, "Sweeper recovery must record the file it adopts")
		assert.Equal(t, claim.OriginalFilename, recorded[claim.Key].OriginalFilename,
			"The recovered record should carry the claim's original filename")
		assert.Equal(t, claim.CreatedBy, recorded[claim.Key].CreatedBy,
			"Recovery runs without a principal, so the uploader must come from the claim, not the audit handler")
	})

	t.Run("LeavesRecentlyExpiredClaim", func(t *testing.T) {
		env := setupWorker(t)

		claim := &store.UploadClaim{
			ID:        id.GenerateUUID(),
			Key:       "priv/recently-expired.bin",
			UploadID:  "session-recent",
			CreatedBy: "tester",
			Status:    store.ClaimStatusPending,
			ExpiresAt: timex.Now().Add(-config.DefaultSweepInterval / 2),
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Recently expired claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		got, err := env.CS.Get(env.Ctx, claim.ID)
		require.NoError(t, err, "Recently expired claim should remain queryable during grace period")
		assert.Equal(t, claim.Key, got.Key, "Sweeper must not touch claims still inside the grace window")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Recently expired claim must not be enqueued for deletion")
	})

	t.Run("DoesNotEnqueueDeleteWhenClaimCompletesAfterStat", func(t *testing.T) {
		env := setupWorker(t)

		claim := &store.UploadClaim{
			ID:        id.GenerateUUID(),
			Key:       "priv/race-completed-after-stat.bin",
			UploadID:  "session-race",
			Size:      7,
			CreatedBy: "tester",
			Status:    store.ClaimStatusPending,
			ExpiresAt: timex.Now().Add(-2 * config.DefaultSweepInterval),
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Expired multipart claim creation should succeed")

		completedDuringStat := false
		svc := &StatHookService{
			Service: env.Svc,
			afterStat: func(ctx context.Context, _ storage.StatObjectOptions, _ *storage.ObjectInfo, err error) {
				if completedDuringStat || !errors.Is(err, storage.ErrObjectNotFound) {
					return
				}

				completedDuringStat = true

				putMemoryObject(t, env.Svc, claim.Key)
				require.NoError(t, env.DB.RunInTx(ctx, func(txCtx context.Context, tx orm.DB) error {
					return env.CS.MarkUploaded(txCtx, tx, *claim)
				}), "Concurrent complete_upload bookkeeping should succeed")
			},
		}

		worker.NewClaimSweeper(env.DB, svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		got, err := env.CS.Get(env.Ctx, claim.ID)
		require.NoError(t, err, "Concurrent completed claim should remain queryable")
		assert.Equal(t, store.ClaimStatusUploaded, got.Status, "Concurrent complete_upload should win over the stale delete plan")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Stale delete plan must not enqueue a delete for a claim completed after StatObject")

		// The two finalize paths raced over one claim. Both write the
		// registry record, and both are gated on the same compare-and-set,
		// so exactly one of them may have won.
		recorded, err := env.FS.Lookup(env.Ctx, []string{claim.Key})
		require.NoError(t, err, "Registry lookup should succeed")
		assert.Len(t, recorded, 1, "The racing finalize paths must produce exactly one record")
	})

	t.Run("LeavesLiveClaim", func(t *testing.T) {
		env := setupWorker(t)

		claim := &store.UploadClaim{
			ID:        id.GenerateUUID(),
			Key:       "priv/live.bin",
			CreatedBy: "tester",
			Status:    store.ClaimStatusPending,
			ExpiresAt: timex.Now().AddHours(1), // future
			CreatedAt: timex.Now(),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Live claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

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
		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Empty sweep should not enqueue anything")
	})
}

// newUnadoptedClaim seeds a finalized upload nobody adopted, whose claim
// TTL elapsed expiredFor ago.
func newUnadoptedClaim(t *testing.T, env *TestEnv, key string, expiredFor time.Duration) *store.UploadClaim {
	t.Helper()

	claim := &store.UploadClaim{
		ID:               id.GenerateUUID(),
		Key:              key,
		Size:             7,
		OriginalFilename: "abandoned.bin",
		CreatedBy:        "tester",
		Status:           store.ClaimStatusPending,
		ExpiresAt:        timex.Now().Add(-expiredFor),
		CreatedAt:        timex.Now().Add(-expiredFor),
	}

	require.NoError(t, env.CS.Create(env.Ctx, claim), "Unadopted claim creation should succeed")
	require.NoError(t, env.DB.RunInTx(env.Ctx, func(txCtx context.Context, tx orm.DB) error {
		return env.CS.MarkUploaded(txCtx, tx, *claim)
	}), "MarkUploaded should finalize the abandoned upload")

	putMemoryObject(t, env.Svc, key)

	return claim
}

// TestClaimSweeperOrphanReclaim covers the second sweep pass: uploads
// that finalized and were never adopted. Nothing else reclaims them, so
// without this pass they accumulate forever — but reclaiming too eagerly
// would delete a file whose business save merely ran late, hence the
// opt-in retention window.
func TestClaimSweeperOrphanReclaim(t *testing.T) {
	t.Run("DisabledByDefault", func(t *testing.T) {
		env := setupWorker(t)
		claim := newUnadoptedClaim(t, env, "priv/orphan-default.bin", 30*24*time.Hour)

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		_, err := env.CS.Get(env.Ctx, claim.ID)
		require.NoError(t, err, "An unconfigured retention must never delete user data")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Reclamation is opt-in; nothing should be scheduled for deletion")
	})

	t.Run("ReclaimsAbandonedUpload", func(t *testing.T) {
		env := setupWorker(t)
		env.Cfg.OrphanRetention = 7 * 24 * time.Hour

		claim := newUnadoptedClaim(t, env, "priv/orphan-old.bin", 30*24*time.Hour)

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		_, err := env.CS.Get(env.Ctx, claim.ID)
		assert.ErrorIs(t, err, storage.ErrClaimNotFound,
			"The claim must go, or the file would stay adoptable after its object is deleted")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		require.Len(t, leased, 1, "The abandoned object should be scheduled for deletion")
		assert.Equal(t, claim.Key, leased[0].Key, "The queue row should target the abandoned object")
		assert.Equal(t, storage.DeleteReasonOrphaned, leased[0].Reason, "The queue row should carry the orphaned reason")
	})

	t.Run("LeavesUploadsInsideTheRetentionWindow", func(t *testing.T) {
		env := setupWorker(t)
		env.Cfg.OrphanRetention = 7 * 24 * time.Hour

		// Expired as a claim, but well inside the reclamation window — a
		// business save running late must keep its file.
		claim := newUnadoptedClaim(t, env, "priv/orphan-young.bin", time.Hour)

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		got, err := env.CS.Get(env.Ctx, claim.ID)
		require.NoError(t, err, "A recently expired unadopted claim should survive")
		assert.Equal(t, store.ClaimStatusUploaded, got.Status, "It should still be adoptable")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Nothing inside the retention window may be scheduled for deletion")
	})

	// The two passes must stay disjoint: an unfinished upload belongs to
	// the expiry sweep, which probes the backend before destroying
	// anything, and must never be reclaimed by the orphan pass — which
	// probes nothing because a finalized object needs no probing.
	//
	// The fixture sits in the one window where only the orphan pass could
	// act: past the reclamation cutoff, but still inside the expiry
	// sweep's grace period, so a single leased row would prove the orphan
	// pass reached a claim it must not see.
	t.Run("IgnoresPendingClaims", func(t *testing.T) {
		env := setupWorker(t)
		env.Cfg.OrphanRetention = time.Minute

		claim := &store.UploadClaim{
			ID:        id.GenerateUUID(),
			Key:       "priv/still-pending.bin",
			Size:      7,
			CreatedBy: "tester",
			Status:    store.ClaimStatusPending,
			ExpiresAt: timex.Now().Add(-2 * time.Minute),
			CreatedAt: timex.Now().Add(-2 * time.Minute),
		}
		require.NoError(t, env.CS.Create(env.Ctx, claim), "Pending claim creation should succeed")

		worker.NewClaimSweeper(env.DB, env.Svc, env.CS, env.PS, env.DQ, env.Cfg).Run(env.Ctx)

		got, err := env.CS.Get(env.Ctx, claim.ID)
		require.NoError(t, err, "An unfinished upload must survive the orphan pass")
		assert.Equal(t, store.ClaimStatusPending, got.Status, "Its status should be untouched")

		leased, err := env.DQ.Lease(env.Ctx, timex.Now().AddHours(1), 10, time.Minute)
		require.NoError(t, err, "Lease should succeed")
		assert.Empty(t, leased, "Only finalized uploads are reclaimable as orphans")
	})
}
