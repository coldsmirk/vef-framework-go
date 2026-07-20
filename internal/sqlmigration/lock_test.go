package sqlmigration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func TestWithLockSerializesOneName(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		const workerCount = 4

		var (
			inside  atomic.Int32
			overlap atomic.Bool
		)

		results := make(chan error, workerCount)
		for range workerCount {
			go func() {
				results <- WithLock(env.Ctx, env.DB, env.DS.Kind, "serialize",
					func(context.Context, orm.DB) error {
						if inside.Add(1) > 1 {
							overlap.Store(true)
						}
						defer inside.Add(-1)

						time.Sleep(30 * time.Millisecond)

						return nil
					})
			}()
		}

		for range workerCount {
			require.NoError(t, <-results, "Every lock holder should complete for %s", env.DS.Kind)
		}

		assert.False(t, overlap.Load(), "Holders of one lock name must never overlap for %s", env.DS.Kind)
	})
}

func TestWithLockHonorsTheCallerDeadlineWhileBlocked(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		acquired := make(chan struct{})
		release := make(chan struct{})
		holderResult := make(chan error, 1)

		go func() {
			holderResult <- WithLock(env.Ctx, env.DB, env.DS.Kind, "deadline",
				func(context.Context, orm.DB) error {
					close(acquired)
					<-release

					return nil
				})
		}()

		t.Cleanup(func() {
			close(release)
			require.NoError(t, <-holderResult, "The lock holder should release cleanly for %s", env.DS.Kind)
		})

		select {
		case <-acquired:
		case <-time.After(10 * time.Second):
			t.Fatal("The lock holder must acquire the lock")
		}

		blockedCtx, cancel := context.WithTimeout(env.Ctx, 250*time.Millisecond)
		defer cancel()

		startedAt := time.Now()
		err := WithLock(blockedCtx, env.DB, env.DS.Kind, "deadline",
			func(context.Context, orm.DB) error { return nil })

		require.Error(t, err, "A blocked acquisition must fail when its deadline expires for %s", env.DS.Kind)
		assert.Less(t, time.Since(startedAt), 2*time.Second,
			"A blocked acquisition must respect the short deadline for %s", env.DS.Kind)
	})
}

func TestWithLockDistinctNamesDoNotContend(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind == config.SQLite {
			t.Skip("SQLite locks the whole database file; distinct names share one write lock by design")
		}

		acquired := make(chan struct{})
		release := make(chan struct{})
		holderResult := make(chan error, 1)

		go func() {
			holderResult <- WithLock(env.Ctx, env.DB, env.DS.Kind, "module-a",
				func(context.Context, orm.DB) error {
					close(acquired)
					<-release

					return nil
				})
		}()

		t.Cleanup(func() {
			close(release)
			require.NoError(t, <-holderResult, "The first module's holder should release cleanly for %s", env.DS.Kind)
		})

		select {
		case <-acquired:
		case <-time.After(10 * time.Second):
			t.Fatal("The first module's holder must acquire its lock")
		}

		otherCtx, cancel := context.WithTimeout(env.Ctx, 5*time.Second)
		defer cancel()

		err := WithLock(otherCtx, env.DB, env.DS.Kind, "module-b",
			func(context.Context, orm.DB) error { return nil })
		require.NoError(t, err,
			"A different module's migration must proceed while another module holds its own lock for %s", env.DS.Kind)
	})
}

func TestWithLockRejectsUnknownKind(t *testing.T) {
	db := testx.NewTestDB(t)

	err := WithLock(t.Context(), db, "oracle-ish", "any", func(context.Context, orm.DB) error { return nil })
	assert.ErrorIs(t, err, ErrUnsupportedDBKind, "An unknown dialect must fail with the sentinel")
}
