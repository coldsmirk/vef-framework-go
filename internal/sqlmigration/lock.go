package sqlmigration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

const (
	// postgresLockSQL takes a transaction-scoped advisory lock derived from
	// the lock name and the connection's database + schema, so co-hosted
	// deployments never contend across databases.
	postgresLockSQL = `SELECT pg_advisory_xact_lock(
    hashtextextended('vef:migration:' || ? || ':' || current_database() || ':' || current_schema(), 0)
)`
	// mysqlAcquireLockSQL blocks until the named user lock is granted. The
	// SHA2-256 hex digest is exactly 64 characters — GET_LOCK's name bound —
	// and, unlike MD5/SHA1, survives MySQL 9.6+, which moved the legacy
	// hash functions out of the server core.
	mysqlAcquireLockSQL = `SELECT GET_LOCK(SHA2(CONCAT('vef:migration:', ?, ':', DATABASE()), 256), -1)`
	mysqlReleaseLockSQL = `SELECT RELEASE_LOCK(SHA2(CONCAT('vef:migration:', ?, ':', DATABASE()), 256))`
	// lockCleanupTimeout bounds the release/rollback statements that must
	// run even after the caller's context died.
	lockCleanupTimeout = 5 * time.Second
	// sqliteBusyRetryInterval paces BEGIN IMMEDIATE attempts while another
	// connection holds the SQLite write lock, keeping acquisition responsive
	// to the caller's context.
	sqliteBusyRetryInterval = 25 * time.Millisecond
)

// errMySQLLockProtocol indicates GET_LOCK/RELEASE_LOCK answered with
// something other than success — a timeout, a lock owned elsewhere, or NULL.
var errMySQLLockProtocol = errors.New("sqlmigration: mysql migration lock protocol violation")

// WithLock runs fn while holding the named migration lock, serializing
// concurrently booting nodes through the database's own locking primitive:
// a transaction-scoped advisory lock on Postgres, GET_LOCK on a dedicated
// MySQL connection, and BEGIN IMMEDIATE on SQLite (whose single writer makes
// the database file itself the lock scope). fn receives the handle the DDL
// must execute on — a transaction on Postgres and SQLite, a dedicated
// connection on MySQL.
func WithLock(
	ctx context.Context,
	db orm.DB,
	kind config.DBKind,
	name string,
	fn func(context.Context, orm.DB) error,
) error {
	switch kind {
	case config.Postgres:
		return withPostgresLock(ctx, db, name, fn)
	case config.MySQL:
		return withMySQLLock(ctx, db, name, fn)
	case config.SQLite:
		return withSQLiteLock(ctx, db, name, fn)
	default:
		return fmt.Errorf("%w %q", ErrUnsupportedDBKind, kind)
	}
}

func withPostgresLock(
	ctx context.Context,
	db orm.DB,
	name string,
	fn func(context.Context, orm.DB) error,
) error {
	return db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		if _, err := tx.NewRaw(postgresLockSQL, name).Exec(ctx); err != nil {
			return fmt.Errorf("acquire migration lock %q: %w", name, err)
		}

		return fn(ctx, tx)
	})
}

func withMySQLLock(
	ctx context.Context,
	db orm.DB,
	name string,
	fn func(context.Context, orm.DB) error,
) error {
	return db.RunOnConnection(ctx, func(ctx context.Context, conn orm.DB) (resultErr error) {
		acquired, err := mysqlLockResult(ctx, conn, mysqlAcquireLockSQL, name)
		if err != nil {
			return fmt.Errorf("acquire migration lock %q: %w", name, err)
		}

		if !acquired.Valid || acquired.Int64 != 1 {
			return fmt.Errorf("%w: acquire returned %s", errMySQLLockProtocol, formatLockResult(acquired))
		}

		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lockCleanupTimeout)
			defer cancel()

			released, releaseErr := mysqlLockResult(cleanupCtx, conn, mysqlReleaseLockSQL, name)
			if releaseErr != nil {
				appendCleanupError(&resultErr, fmt.Errorf("release migration lock %q: %w", name, releaseErr))

				return
			}

			if !released.Valid || released.Int64 != 1 {
				appendCleanupError(
					&resultErr,
					fmt.Errorf("%w: release returned %s", errMySQLLockProtocol, formatLockResult(released)),
				)
			}
		}()

		return fn(ctx, conn)
	})
}

// withSQLiteLock serializes migration through one BEGIN IMMEDIATE
// transaction. SQLITE_BUSY is retried from Go on the whole attempt — the
// connection's busy timeout blocks inside the driver where context
// cancellation cannot interrupt it, and contention can also surface while a
// fresh pooled connection runs its DSN pragmas against the lock holder — so
// each attempt runs with the busy timeout zeroed and the loop owns the
// waiting. Callers must keep fn idempotent per attempt; Run's presence probe
// re-runs naturally.
func withSQLiteLock(
	ctx context.Context,
	db orm.DB,
	name string,
	fn func(context.Context, orm.DB) error,
) error {
	for {
		err := attemptSQLiteLocked(ctx, db, name, fn)
		if err == nil || !IsBusyContention(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sqliteBusyRetryInterval):
		}
	}
}

func attemptSQLiteLocked(
	ctx context.Context,
	db orm.DB,
	name string,
	fn func(context.Context, orm.DB) error,
) error {
	return db.RunOnConnection(ctx, func(ctx context.Context, conn orm.DB) (resultErr error) {
		restoreBusyTimeout, err := suspendSQLiteBusyTimeout(ctx, conn)
		if err != nil {
			return err
		}
		defer restoreBusyTimeout(&resultErr)

		if _, err := conn.NewRaw("BEGIN IMMEDIATE").Exec(ctx); err != nil {
			return fmt.Errorf("acquire migration lock %q: %w", name, err)
		}

		transactionOpen := true
		defer func() {
			if !transactionOpen {
				return
			}

			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lockCleanupTimeout)
			defer cancel()

			if _, rollbackErr := conn.NewRaw("ROLLBACK").Exec(cleanupCtx); rollbackErr != nil {
				appendCleanupError(&resultErr, fmt.Errorf("release migration lock %q: %w", name, rollbackErr))
			}
		}()

		if err := fn(ctx, conn); err != nil {
			return err
		}

		if _, err := conn.NewRaw("COMMIT").Exec(ctx); err != nil {
			return fmt.Errorf("commit migration %q: %w", name, err)
		}

		transactionOpen = false

		return nil
	})
}

// suspendSQLiteBusyTimeout zeroes the connection's busy timeout so lock
// contention returns SQLITE_BUSY immediately instead of stalling inside the
// driver. The returned restore puts the previous timeout back before the
// pooled connection serves anyone else.
func suspendSQLiteBusyTimeout(ctx context.Context, conn orm.DB) (func(*error), error) {
	var busyTimeout int
	if err := conn.NewRaw("PRAGMA busy_timeout").Scan(ctx, &busyTimeout); err != nil {
		return nil, fmt.Errorf("read busy timeout: %w", err)
	}

	if _, err := conn.NewRaw("PRAGMA busy_timeout = 0").Exec(ctx); err != nil {
		return nil, fmt.Errorf("suspend busy timeout: %w", err)
	}

	return func(resultErr *error) {
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lockCleanupTimeout)
		defer cancel()

		restore := fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout)
		if _, err := conn.NewRaw(restore).Exec(restoreCtx); err != nil {
			appendCleanupError(resultErr, fmt.Errorf("restore busy timeout: %w", err))
		}
	}, nil
}

func mysqlLockResult(ctx context.Context, db orm.DB, query, name string) (sql.NullInt64, error) {
	var result sql.NullInt64

	err := db.NewRaw(query, name).Scan(ctx, &result)

	return result, err
}

func formatLockResult(result sql.NullInt64) string {
	if !result.Valid {
		return "NULL"
	}

	return fmt.Sprintf("%d", result.Int64)
}

func appendCleanupError(resultErr *error, cleanupErr error) {
	if *resultErr == nil {
		*resultErr = cleanupErr

		return
	}

	*resultErr = errors.Join(*resultErr, cleanupErr)
}
