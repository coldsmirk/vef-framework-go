package datasource

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/datasource"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

func newSQLiteCfg(t *testing.T, name string) config.DataSourceConfig {
	t.Helper()

	return config.DataSourceConfig{
		Kind: config.SQLite,
		Path: filepath.Join(t.TempDir(), name+".db"),
	}
}

func newTestRegistry(t *testing.T) *registry {
	t.Helper()

	ctx := context.Background()
	r, err := newRegistry(newSQLiteCfg(t, "primary"), logx.Discard())
	require.NoError(t, err, "Primary registry should construct")
	t.Cleanup(func() {
		require.NoError(t, r.Shutdown(ctx), "Registry should shut down cleanly")
	})

	return r
}

func TestRegistryTestConnection(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	t.Run("Success", func(t *testing.T) {
		info, err := r.TestConnection(ctx, newSQLiteCfg(t, "probe"))
		require.NoError(t, err, "Reachable SQLite source probe should succeed")
		require.NotEmpty(t, info.Version, "Successful probe should report the server version")

		require.Equal(t, []string{datasource.PrimaryName}, r.Names(),
			"TestConnection must not register the probed source")
	})

	t.Run("UnsupportedKind", func(t *testing.T) {
		info, err := r.TestConnection(ctx, config.DataSourceConfig{Kind: "no-such-dialect"})
		require.Error(t, err, "Unsupported dialect should fail the probe")
		require.Empty(t, info.Version, "Failed probe should return a zero ConnectionInfo")
	})

	t.Run("Unreachable", func(t *testing.T) {
		cfg := config.DataSourceConfig{
			Kind: config.SQLite,
			Path: filepath.Join(t.TempDir(), "missing-dir", "probe.db"),
		}

		info, err := r.TestConnection(ctx, cfg)
		require.Error(t, err, "Unreachable source should fail the probe")
		require.Empty(t, info.Version, "Failed probe should return a zero ConnectionInfo")

		require.Equal(t, []string{datasource.PrimaryName}, r.Names(),
			"Failed TestConnection should not leak a registry entry")
	})

	t.Run("RespectsCallerCancellation", func(t *testing.T) {
		canceled, cancel := context.WithCancel(ctx)
		cancel() // the internal default timeout must not swallow the caller's cancel

		_, err := r.TestConnection(canceled, newSQLiteCfg(t, "canceled"))
		require.Error(t, err, "Canceled caller context should abort the probe")
	})
}

func TestRegistryPrimary(t *testing.T) {
	r := newTestRegistry(t)

	require.NotNil(t, r.Primary(), "Primary orm.DB should be available")
	require.True(t, r.Has(datasource.PrimaryName), "Primary should be reported as present")
	require.Equal(t, []string{datasource.PrimaryName}, r.Names(), "Fresh registry should only know primary")

	kind, err := r.Kind(datasource.PrimaryName)
	require.NoError(t, err, "Kind lookup for primary should succeed")
	require.Equal(t, config.SQLite, kind, "Primary kind should match seed config")

	got, err := r.Get(datasource.PrimaryName)
	require.NoError(t, err, "Get(primary) should not error")
	require.Equal(t, r.Primary(), got, "Get(primary) returns the same DB as Primary()")
}

func TestRegistryRegisterAndGet(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	db, err := r.Register(ctx, "analytics", newSQLiteCfg(t, "analytics"))
	require.NoError(t, err, "First Register should succeed")
	require.NotNil(t, db, "Register should return a usable DB")

	again, err := r.Get("analytics")
	require.NoError(t, err, "Get after Register should succeed")
	require.Equal(t, db, again, "Get returns the same DB instance as Register")

	names := r.Names()
	require.Equal(t, []string{"analytics", datasource.PrimaryName}, names, "Names sorted lexically")

	_, err = r.Register(ctx, "analytics", newSQLiteCfg(t, "analytics2"))
	require.ErrorIs(t, err, datasource.ErrExists, "Duplicate Register should fail with ErrExists")
}

func TestRegistryRegisterRejectsInvalidName(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	_, err := r.Register(ctx, "", newSQLiteCfg(t, "x"))
	require.ErrorIs(t, err, datasource.ErrNameInvalid, "Empty name should be rejected")

	_, err = r.Register(ctx, datasource.PrimaryName, newSQLiteCfg(t, "x"))
	require.ErrorIs(t, err, datasource.ErrPrimaryReserved, "Primary name should be reserved")
}

func TestRegistryGetUnknownReturnsNotFound(t *testing.T) {
	r := newTestRegistry(t)

	_, err := r.Get("does-not-exist")
	require.ErrorIs(t, err, datasource.ErrNotFound, "Get on unknown name returns ErrNotFound")

	_, err = r.Kind("does-not-exist")
	require.ErrorIs(t, err, datasource.ErrNotFound, "Kind on unknown name returns ErrNotFound")

	require.False(t, r.Has("does-not-exist"), "Has on unknown name reports false")
}

func TestRegistryUpdate(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	old, err := r.Register(ctx, "tenant1", newSQLiteCfg(t, "v1"))
	require.NoError(t, err, "Register should succeed")
	require.NotNil(t, old, "Register should return the old DB")

	newDB, err := r.Update(ctx, "tenant1", newSQLiteCfg(t, "v2"))
	require.NoError(t, err, "Update should succeed when name exists")
	require.NotNil(t, newDB, "Update should return the new DB")
	require.NotSame(t, old, newDB, "Update produces a new DB instance")

	got, err := r.Get("tenant1")
	require.NoError(t, err, "Get after Update should succeed")
	require.Equal(t, newDB, got, "Get returns the latest DB after Update")
}

func TestRegistryUpdateFailureKeepsOld(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	old, err := r.Register(ctx, "tenant1", newSQLiteCfg(t, "v1"))
	require.NoError(t, err, "Register should seed the original source")

	badCfg := config.DataSourceConfig{Kind: "no-such-dialect"}
	_, err = r.Update(ctx, "tenant1", badCfg)
	require.Error(t, err, "Update with unsupported dialect must fail")

	got, err := r.Get("tenant1")
	require.NoError(t, err, "Old entry should still be reachable after failed Update")
	require.Equal(t, old, got, "Old DB instance should be preserved on Update failure")
}

func TestRegistryUpdateUnknownReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	_, err := r.Update(ctx, "ghost", newSQLiteCfg(t, "ghost"))
	require.ErrorIs(t, err, datasource.ErrNotFound, "Update on missing name returns ErrNotFound")
}

func TestRegistryUnregister(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	_, err := r.Register(ctx, "to-remove", newSQLiteCfg(t, "rm"))
	require.NoError(t, err, "Register should seed the source to remove")

	require.NoError(t, r.Unregister(ctx, "to-remove"), "Unregister should succeed")

	_, err = r.Get("to-remove")
	require.ErrorIs(t, err, datasource.ErrNotFound, "Get after Unregister returns ErrNotFound")

	_, err = r.Kind("to-remove")
	require.ErrorIs(t, err, datasource.ErrNotFound, "Kind after Unregister returns ErrNotFound")

	require.False(t, r.Has("to-remove"), "Has after Unregister returns false")

	err = r.Unregister(ctx, "to-remove")
	require.ErrorIs(t, err, datasource.ErrNotFound, "Double Unregister should return ErrNotFound")
}

func TestRegistryReRegisterAfterUnregister(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	first, err := r.Register(ctx, "reopen", newSQLiteCfg(t, "first"))
	require.NoError(t, err, "Initial Register should succeed")

	require.NoError(t, r.Unregister(ctx, "reopen"), "Unregister frees the name")

	second, err := r.Register(ctx, "reopen", newSQLiteCfg(t, "second"))
	require.NoError(t, err, "Register should succeed once the name is free")
	require.NotSame(t, first, second, "Re-registered source should use a fresh DB instance")

	got, err := r.Get("reopen")
	require.NoError(t, err, "Get after re-registering should succeed")
	require.Equal(t, second, got, "Get should return the re-registered DB")
}

func TestRegistryPrimaryReserved(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	require.ErrorIs(t, r.Unregister(ctx, datasource.PrimaryName), datasource.ErrPrimaryReserved,
		"Unregister(primary) must return ErrPrimaryReserved")

	_, err := r.Update(ctx, datasource.PrimaryName, newSQLiteCfg(t, "p2"))
	require.ErrorIs(t, err, datasource.ErrPrimaryReserved, "Update(primary) must return ErrPrimaryReserved")
}

func TestRegistryReconcileAddsUpdatesAndRemoves(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	keepCfg := newSQLiteCfg(t, "keep")
	updateOldCfg := newSQLiteCfg(t, "u-old")
	updateNewCfg := newSQLiteCfg(t, "u-new")
	removeCfg := newSQLiteCfg(t, "rm")

	_, err := r.Register(ctx, "keep", keepCfg)
	require.NoError(t, err, "Register should seed unchanged source")
	_, err = r.Register(ctx, "tenant", updateOldCfg)
	require.NoError(t, err, "Register should seed source to update")
	_, err = r.Register(ctx, "remove", removeCfg)
	require.NoError(t, err, "Register should seed source to remove")

	specs := []datasource.Spec{
		{Name: "keep", Config: keepCfg},                   // unchanged
		{Name: "tenant", Config: updateNewCfg},            // updated cfg
		{Name: "fresh", Config: newSQLiteCfg(t, "fresh")}, // added
		// "remove" omitted → removed
	}

	report, err := r.Reconcile(ctx, specs)
	require.NoError(t, err, "Reconcile should not return a top-level error")
	require.Equal(t, []string{"fresh"}, report.Added, "Added list matches expected diff")
	require.Equal(t, []string{"tenant"}, report.Updated, "Updated list matches expected diff")
	require.Equal(t, []string{"remove"}, report.Removed, "Removed list matches expected diff")
	require.Nil(t, report.Errors, "Errors should be nil when all actions succeed")

	require.True(t, r.Has("keep"), "Unchanged source should remain")
	require.True(t, r.Has("tenant"), "Updated source should remain")
	require.True(t, r.Has("fresh"), "Added source should be present")
	require.False(t, r.Has("remove"), "Removed source should be gone")
}

func TestRegistryReconcileDryRun(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	cfg := newSQLiteCfg(t, "candidate")
	specs := []datasource.Spec{{Name: "candidate", Config: cfg}}

	report, err := r.Reconcile(ctx, specs, datasource.WithReconcileDryRun())
	require.NoError(t, err, "Dry run Reconcile should not error")
	require.Equal(t, []string{"candidate"}, report.Added, "Dry run should still report diff")
	require.False(t, r.Has("candidate"), "Dry run should not register the source")
}

func TestRegistryReconcileIgnoresPrimaryAndEmpty(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	specs := []datasource.Spec{
		{Name: "", Config: newSQLiteCfg(t, "empty")},
		{Name: datasource.PrimaryName, Config: newSQLiteCfg(t, "shadow")},
	}

	report, err := r.Reconcile(ctx, specs)
	require.NoError(t, err, "Reconcile should ignore invalid desired entries")
	require.Empty(t, report.Added, "Empty and primary entries should be ignored")
	require.Empty(t, report.Updated, "Empty and primary entries should not update sources")
	require.Empty(t, report.Removed, "Empty desired set should not remove existing sources")
}

func TestRegistryReconcilePartialFailureAggregatesErrors(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	specs := []datasource.Spec{
		{Name: "good", Config: newSQLiteCfg(t, "good")},
		{Name: "bad", Config: config.DataSourceConfig{Kind: "no-such-dialect"}},
	}

	report, err := r.Reconcile(ctx, specs)
	require.NoError(t, err, "Partial failure should not surface as top-level error")
	require.Equal(t, []string{"good"}, report.Added, "Good source should still be added")
	require.NotNil(t, report.Errors, "Errors should be surfaced in the report")
	require.Contains(t, report.Errors, "bad", "Bad source name should be keyed in errors map")
}

func TestRegistryHealthCheck(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	_, err := r.Register(ctx, "extra", newSQLiteCfg(t, "extra"))
	require.NoError(t, err, "Register should seed extra source for health check")

	results := r.HealthCheck(ctx)
	require.Len(t, results, 2, "HealthCheck reports primary + one extra")
	require.NoError(t, results[datasource.PrimaryName], "Primary should be healthy")
	require.NoError(t, results["extra"], "Extra source should be healthy")
}

func TestRegistryConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	const (
		workers = 16
		ops     = 20
	)

	var (
		wg           sync.WaitGroup
		getSuccesses atomic.Int64
		getMisses    atomic.Int64
	)

	for id := range workers {
		wg.Go(func() {
			name := "ds-" + string(rune('a'+id))

			for range ops {
				if _, err := r.Register(ctx, name, newSQLiteCfg(t, name)); err == nil || errors.Is(err, datasource.ErrExists) {
					if _, gerr := r.Get(name); gerr == nil {
						getSuccesses.Add(1)
					} else {
						getMisses.Add(1)
					}
				}

				if err := r.Unregister(ctx, name); err != nil &&
					!errors.Is(err, datasource.ErrNotFound) {
					t.Errorf("Unregister %q unexpected error: %v", name, err)

					return
				}
			}
		})
	}

	wg.Wait()

	require.Positive(t, getSuccesses.Load(), "At least some Get calls should succeed")
	// Misses are tolerated under contention; the counter is only used to
	// exercise the path. Read it via Load to avoid copying the atomic value.
	_ = getMisses.Load()
}

func TestRegistryConcurrentSameName(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	const (
		workers = 16
		ops     = 25
	)

	// A shared in-memory SQLite config keeps every Open independent (no file
	// locking) while all workers contend on the SAME registry key, exercising
	// the Register/Update/Unregister atomicity that the per-name concurrency
	// test does not.
	cfg := config.DataSourceConfig{Kind: config.SQLite}

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range ops {
				if _, err := r.Register(ctx, "contended", cfg); err != nil &&
					!errors.Is(err, datasource.ErrExists) {
					t.Errorf("Register: unexpected error: %v", err)

					return
				}

				if _, err := r.Update(ctx, "contended", cfg); err != nil &&
					!errors.Is(err, datasource.ErrNotFound) {
					t.Errorf("Update: unexpected error: %v", err)

					return
				}

				if err := r.Unregister(ctx, "contended"); err != nil &&
					!errors.Is(err, datasource.ErrNotFound) {
					t.Errorf("Unregister: unexpected error: %v", err)

					return
				}
			}
		})
	}

	wg.Wait()

	// The final state must be consistent: the name is either absent or maps to
	// a usable connection — never a half-closed or leaked entry.
	db, err := r.Get("contended")
	if err != nil {
		require.ErrorIs(t, err, datasource.ErrNotFound, "Absent source should report NotFound")

		return
	}

	var v int
	require.NoError(t, db.NewRaw("SELECT 1").Scan(ctx, &v), "Present source should be usable")
	require.Equal(t, 1, v, "Present source should answer queries")
}

func TestRegistryUpdateWithCloseGrace(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	old, err := r.Register(ctx, "graceful", newSQLiteCfg(t, "v1"))
	require.NoError(t, err, "Register should succeed")

	_, err = r.Update(ctx, "graceful", newSQLiteCfg(t, "v2"), datasource.WithCloseGrace(2*time.Second))
	require.NoError(t, err, "Update with close grace should succeed")

	// The replaced connection is closed only after the grace window, so a
	// caller still holding the old orm.DB can keep querying right after Update.
	var v int
	require.NoError(t, old.NewRaw("SELECT 1").Scan(ctx, &v),
		"Old connection should still serve queries during the grace window")
	require.Equal(t, 1, v, "Drained query should return its value")
}

func TestRegistryUnregisterDrainsInFlight(t *testing.T) {
	ctx := context.Background()
	r := newTestRegistry(t)

	db, err := r.Register(ctx, "draining", newSQLiteCfg(t, "drain"))
	require.NoError(t, err, "Register should succeed")

	require.NoError(t, r.Unregister(ctx, "draining", datasource.WithCloseGrace(2*time.Second)),
		"Unregister with close grace should succeed")

	_, err = r.Get("draining")
	require.ErrorIs(t, err, datasource.ErrNotFound, "Get after Unregister returns ErrNotFound")

	// The connection is closed only after the grace window, so the previously
	// obtained orm.DB reference can still finish in-flight work.
	var v int
	require.NoError(t, db.NewRaw("SELECT 1").Scan(ctx, &v),
		"Held reference should drain during the grace window")
	require.Equal(t, 1, v, "Drained query should return its value")
}
