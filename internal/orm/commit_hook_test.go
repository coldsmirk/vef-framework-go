package orm

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/sqliteshim"

	"github.com/coldsmirk/vef-framework-go/config"
)

// commitHookRow is the minimal model used to prove that a hook observes
// state the transaction committed, not merely that it was invoked.
type commitHookRow struct {
	bun.BaseModel `bun:"table:test_commit_hook"`

	ID int64 `bun:"id,pk"`
}

type commitHookCtxKey struct{}

func newCommitHookDB(t *testing.T) DB {
	t.Helper()

	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	require.NoError(t, err, "open in-memory sqlite")
	sqldb.SetMaxOpenConns(1) // keep a single connection so the in-memory schema persists
	t.Cleanup(func() { require.NoError(t, sqldb.Close(), "close sqlite") })

	db, err := Open(sqldb, config.SQLite)
	require.NoError(t, err, "wrap sqlite into orm.DB")

	db.RegisterModel((*commitHookRow)(nil))
	require.NoError(t, db.ResetModel(context.Background(), (*commitHookRow)(nil)), "create table")

	return db
}

func insertCommitHookRow(ctx context.Context, tx DB, id int64) error {
	_, err := tx.NewInsert().Model(&commitHookRow{ID: id}).Exec(ctx)

	return err
}

func countCommitHookRows(ctx context.Context, t *testing.T, db DB) int {
	t.Helper()

	count, err := db.NewSelect().Model((*commitHookRow)(nil)).Count(ctx)
	require.NoError(t, err, "count rows")

	return int(count)
}

func TestOnCommit(t *testing.T) {
	ctx := context.Background()

	t.Run("RunsAfterCommitNotDuringTx", func(t *testing.T) {
		db := newCommitHookDB(t)

		var (
			ranInsideTx bool
			rowsSeen    = -1
		)

		err := db.RunInTx(ctx, func(txCtx context.Context, tx DB) error {
			if err := insertCommitHookRow(txCtx, tx, 1); err != nil {
				return err
			}

			if err := OnCommit(txCtx, func(hookCtx context.Context) {
				rowsSeen = countCommitHookRows(hookCtx, t, db)
			}); err != nil {
				return err
			}

			ranInsideTx = rowsSeen >= 0

			return nil
		})

		require.NoError(t, err, "RunInTx should succeed")
		assert.False(t, ranInsideTx, "hook must not run while the transaction is still open")
		assert.Equal(t, 1, rowsSeen, "hook must observe the committed row through the pool handle")
	})

	t.Run("DiscardedOnRollback", func(t *testing.T) {
		db := newCommitHookDB(t)
		failure := errors.New("business failure")

		ran := false

		err := db.RunInTx(ctx, func(txCtx context.Context, tx DB) error {
			require.NoError(t, insertCommitHookRow(txCtx, tx, 1), "insert should succeed")
			require.NoError(t, OnCommit(txCtx, func(context.Context) { ran = true }), "OnCommit should register")

			return failure
		})

		require.ErrorIs(t, err, failure, "RunInTx should surface the business failure")
		assert.False(t, ran, "a rolled-back transaction must not fire its commit hooks")
		assert.Equal(t, 0, countCommitHookRows(ctx, t, db), "the rolled-back insert must not persist")
	})

	t.Run("RunsInRegistrationOrder", func(t *testing.T) {
		db := newCommitHookDB(t)

		var order []int

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, _ DB) error {
			for i := range 3 {
				require.NoError(t, OnCommit(txCtx, func(context.Context) {
					order = append(order, i)
				}), "OnCommit should register")
			}

			return nil
		}), "RunInTx should succeed")

		assert.Equal(t, []int{0, 1, 2}, order, "hooks must fire in registration order")
	})

	t.Run("OutsideTransactionReportsNoScope", func(t *testing.T) {
		ran := false

		err := OnCommit(ctx, func(context.Context) { ran = true })

		require.ErrorIs(t, err, ErrNoCommitScope, "a bare context has no commit to hang the callback on")
		assert.False(t, ran, "nothing may run when registration failed")
	})

	t.Run("ReadOnlyTxIsACommitScope", func(t *testing.T) {
		db := newCommitHookDB(t)

		ran := false

		require.NoError(t, db.RunInReadOnlyTx(ctx, func(txCtx context.Context, _ DB) error {
			return OnCommit(txCtx, func(context.Context) { ran = true })
		}), "RunInReadOnlyTx should succeed")

		assert.True(t, ran, "a read-only transaction still commits, so its hooks fire")
	})

	t.Run("ManualBeginTxIsNotACommitScope", func(t *testing.T) {
		db := newCommitHookDB(t)

		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err, "BeginTx should succeed")

		defer func() { require.NoError(t, tx.Rollback(), "rollback should succeed") }()

		var registerErr error

		require.NoError(t, tx.RunInTx(ctx, func(txCtx context.Context, _ DB) error {
			registerErr = OnCommit(txCtx, func(context.Context) {})

			return nil
		}), "nested RunInTx on a manual transaction should succeed")

		require.ErrorIs(t, registerErr, ErrNoCommitScope,
			"a manual BeginTx scope carries no collector, so OnCommit must report rather than fire at savepoint release")
	})

	t.Run("ConnectionScopeIsACommitScope", func(t *testing.T) {
		db := newCommitHookDB(t)

		ran := false

		require.NoError(t, db.RunOnConnection(ctx, func(connCtx context.Context, conn DB) error {
			return conn.RunInTx(connCtx, func(txCtx context.Context, _ DB) error {
				return OnCommit(txCtx, func(context.Context) { ran = true })
			})
		}), "RunInTx on a connection-scoped handle should succeed")

		assert.True(t, ran,
			"a connection-scoped RunInTx is a real transaction, not a savepoint, so its hooks must fire on its own commit")
	})

	t.Run("RegistrationAfterScopeClosedReports", func(t *testing.T) {
		db := newCommitHookDB(t)

		var (
			escaped context.Context
			ran     bool
		)

		require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, _ DB) error {
			escaped = txCtx

			return nil
		}), "RunInTx should succeed")

		// A context captured by a goroutine outlives its transaction; a late
		// registration must be reported rather than accepted and dropped.
		err := OnCommit(escaped, func(context.Context) { ran = true })

		require.ErrorIs(t, err, ErrNoCommitScope, "the scope's commit has already happened, so there is nothing to defer to")
		assert.False(t, ran, "nothing may run when registration failed")
	})

	t.Run("PanicInOneHookIsIsolated", func(t *testing.T) {
		db := newCommitHookDB(t)

		ran := false

		require.NotPanics(t, func() {
			require.NoError(t, db.RunInTx(ctx, func(txCtx context.Context, _ DB) error {
				require.NoError(t, OnCommit(txCtx, func(context.Context) {
					panic("hook blew up")
				}), "OnCommit should register the panicking hook")

				return OnCommit(txCtx, func(context.Context) { ran = true })
			}), "RunInTx should succeed")
		}, "a panicking hook must not surface at the RunInTx call site after a successful commit")

		assert.True(t, ran, "a later hook must still run after an earlier one panicked")
	})
}

func TestOnCommitNesting(t *testing.T) {
	ctx := context.Background()

	t.Run("NestedHookDeferredUntilOuterCommit", func(t *testing.T) {
		db := newCommitHookDB(t)

		var (
			ran               bool
			ranBeforeOuterEnd bool
		)

		require.NoError(t, db.RunInTx(ctx, func(outerCtx context.Context, outerTx DB) error {
			if err := outerTx.RunInTx(outerCtx, func(innerCtx context.Context, _ DB) error {
				return OnCommit(innerCtx, func(context.Context) { ran = true })
			}); err != nil {
				return err
			}

			ranBeforeOuterEnd = ran

			return nil
		}), "RunInTx should succeed")

		assert.False(t, ranBeforeOuterEnd, "releasing a savepoint is not a commit, so the hook must still be pending")
		assert.True(t, ran, "the nested hook fires once the outermost transaction commits")
	})

	t.Run("NestedRollbackDiscardsOnlyItsOwnHooks", func(t *testing.T) {
		db := newCommitHookDB(t)
		innerFailure := errors.New("inner failure")

		var (
			outerBeforeRan bool
			innerRan       bool
			outerAfterRan  bool
		)

		require.NoError(t, db.RunInTx(ctx, func(outerCtx context.Context, outerTx DB) error {
			if err := OnCommit(outerCtx, func(context.Context) { outerBeforeRan = true }); err != nil {
				return err
			}

			// The caller deliberately swallows the savepoint failure and
			// commits anyway — the case that makes hook unwinding load-bearing.
			err := outerTx.RunInTx(outerCtx, func(innerCtx context.Context, innerTx DB) error {
				if err := insertCommitHookRow(innerCtx, innerTx, 1); err != nil {
					return err
				}

				if err := OnCommit(innerCtx, func(context.Context) { innerRan = true }); err != nil {
					return err
				}

				return innerFailure
			})
			require.ErrorIs(t, err, innerFailure, "the nested scope should surface its failure")

			return OnCommit(outerCtx, func(context.Context) { outerAfterRan = true })
		}), "the outer transaction should still commit")

		assert.True(t, outerBeforeRan, "hooks registered before the savepoint survive its rollback")
		assert.True(t, outerAfterRan, "hooks registered after the savepoint are unaffected")
		assert.False(t, innerRan, "hooks registered inside a rolled-back savepoint must be discarded")
		assert.Equal(t, 0, countCommitHookRows(ctx, t, db), "the savepoint's insert must not persist")
	})
}

// TestOnCommitNestedPanicDiscardsHooks covers the savepoint that is
// abandoned by a panic rather than by an error: the caller recovers and
// commits the enclosing transaction anyway, so the hooks of the rolled-back
// savepoint must not survive to fire.
func TestOnCommitNestedPanicDiscardsHooks(t *testing.T) {
	db := newCommitHookDB(t)

	var (
		innerRan  bool
		outerRan  bool
		recovered any
	)

	require.NoError(t, db.RunInTx(context.Background(), func(outerCtx context.Context, outerTx DB) error {
		func() {
			defer func() { recovered = recover() }()

			_ = outerTx.RunInTx(outerCtx, func(innerCtx context.Context, _ DB) error {
				require.NoError(t, OnCommit(innerCtx, func(context.Context) { innerRan = true }),
					"OnCommit should register inside the savepoint")

				panic("savepoint blew up")
			})
		}()

		return OnCommit(outerCtx, func(context.Context) { outerRan = true })
	}), "the outer transaction should still commit")

	require.NotNil(t, recovered, "the test must actually have recovered a panic")
	assert.False(t, innerRan, "hooks of a savepoint abandoned by a panic must be discarded")
	assert.True(t, outerRan, "the enclosing transaction's own hooks are unaffected")
}

// TestCommitHooksRunDetachesCancellation pins the contract directly on the
// collector: the commit has already happened durably, so a caller whose
// context was canceled in the meantime must not lose the follow-up work,
// while context values stay reachable for logging and tracing.
func TestCommitHooksRunDetachesCancellation(t *testing.T) {
	canceled, cancel := context.WithCancel(context.WithValue(context.Background(), commitHookCtxKey{}, "trace-1"))
	cancel()

	var (
		hookErr   error
		hookValue any
	)

	hooks := new(commitHooks)
	hooks.add(func(ctx context.Context) {
		hookErr = ctx.Err()
		hookValue = ctx.Value(commitHookCtxKey{})
	})

	hooks.run(canceled)

	assert.NoError(t, hookErr, "a canceled caller context must not cancel work whose transaction already committed")
	assert.Equal(t, "trace-1", hookValue, "context values must survive so hooks keep the originating request's identity")
}

// TestCommitHooksRunDrains asserts the collector empties itself, so a
// second run cannot double-fire hooks that already executed.
func TestCommitHooksRunDrains(t *testing.T) {
	runs := 0

	hooks := new(commitHooks)
	hooks.add(func(context.Context) { runs++ })

	hooks.run(context.Background())
	hooks.run(context.Background())

	assert.Equal(t, 1, runs, "hooks must be drained by the run that executed them")
}
