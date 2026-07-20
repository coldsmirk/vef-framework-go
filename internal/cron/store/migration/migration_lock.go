package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

const (
	postgresMigrationLockSQL = `SELECT pg_advisory_xact_lock(
    hashtextextended('vef:cron:migration:' || current_database() || ':' || current_schema(), 0)
)`
	mysqlAcquireMigrationLockSQL = `SELECT GET_LOCK(
    CONCAT('vef:cron:migration:', MD5(DATABASE())), -1
)`
	mysqlReleaseMigrationLockSQL = `SELECT RELEASE_LOCK(
    CONCAT('vef:cron:migration:', MD5(DATABASE()))
)`
	migrationCleanupTimeout = 5 * time.Second
	// sqliteBusyRetryInterval paces BEGIN IMMEDIATE attempts while another
	// connection holds the SQLite write lock, keeping acquisition responsive
	// to the caller's context.
	sqliteBusyRetryInterval = 25 * time.Millisecond
)

// errMySQLLockProtocol indicates GET_LOCK/RELEASE_LOCK answered with
// something other than success — a timeout, a lock owned elsewhere, or NULL.
var errMySQLLockProtocol = errors.New("cron store: mysql migration lock protocol violation")

func withMigrationLock(
	ctx context.Context,
	db orm.DB,
	kind config.DBKind,
	fn func(context.Context, orm.DB) error,
) error {
	switch kind {
	case config.Postgres:
		return withPostgresMigrationLock(ctx, db, fn)
	case config.MySQL:
		return withMySQLMigrationLock(ctx, db, fn)
	case config.SQLite:
		return withSQLiteMigrationLock(ctx, db, fn)
	default:
		return fmt.Errorf("cron store: %w %q", sqlmigration.ErrUnsupportedDBKind, kind)
	}
}

func withPostgresMigrationLock(
	ctx context.Context,
	db orm.DB,
	fn func(context.Context, orm.DB) error,
) error {
	return db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		if _, err := tx.NewRaw(postgresMigrationLockSQL).Exec(ctx); err != nil {
			return fmt.Errorf("cron store: acquire migration lock: %w", err)
		}

		return fn(ctx, tx)
	})
}

func withMySQLMigrationLock(
	ctx context.Context,
	db orm.DB,
	fn func(context.Context, orm.DB) error,
) error {
	return db.RunOnConnection(ctx, func(ctx context.Context, conn orm.DB) (resultErr error) {
		acquired, err := mysqlLockResult(ctx, conn, mysqlAcquireMigrationLockSQL)
		if err != nil {
			return fmt.Errorf("cron store: acquire migration lock: %w", err)
		}

		if !acquired.Valid || acquired.Int64 != 1 {
			return fmt.Errorf("%w: acquire returned %s", errMySQLLockProtocol, formatLockResult(acquired))
		}

		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), migrationCleanupTimeout)
			defer cancel()

			released, releaseErr := mysqlLockResult(cleanupCtx, conn, mysqlReleaseMigrationLockSQL)
			if releaseErr != nil {
				appendCleanupError(&resultErr, fmt.Errorf("cron store: release migration lock: %w", releaseErr))

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

// withSQLiteMigrationLock serializes migration through one BEGIN IMMEDIATE
// transaction. SQLITE_BUSY is retried from Go on the whole attempt — the
// connection's busy timeout blocks inside the driver where context
// cancellation cannot interrupt it, and contention can also surface while a
// fresh pooled connection runs its DSN pragmas against the lock holder — so
// each attempt runs with the busy timeout zeroed and the loop owns the
// waiting. provisionFresh re-probes the tables, making a retried attempt safe.
func withSQLiteMigrationLock(
	ctx context.Context,
	db orm.DB,
	fn func(context.Context, orm.DB) error,
) error {
	for {
		err := attemptSQLiteMigration(ctx, db, fn)
		if err == nil || !isSQLiteBusy(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sqliteBusyRetryInterval):
		}
	}
}

func attemptSQLiteMigration(
	ctx context.Context,
	db orm.DB,
	fn func(context.Context, orm.DB) error,
) error {
	return db.RunOnConnection(ctx, func(ctx context.Context, conn orm.DB) (resultErr error) {
		restoreBusyTimeout, err := suspendSQLiteBusyTimeout(ctx, conn)
		if err != nil {
			return err
		}
		defer restoreBusyTimeout(&resultErr)

		if _, err := conn.NewRaw("BEGIN IMMEDIATE").Exec(ctx); err != nil {
			return fmt.Errorf("cron store: acquire migration lock: %w", err)
		}

		transactionOpen := true
		defer func() {
			if !transactionOpen {
				return
			}

			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), migrationCleanupTimeout)
			defer cancel()

			if _, rollbackErr := conn.NewRaw("ROLLBACK").Exec(cleanupCtx); rollbackErr != nil {
				appendCleanupError(&resultErr, fmt.Errorf("cron store: release migration lock: %w", rollbackErr))
			}
		}()

		if err := fn(ctx, conn); err != nil {
			return err
		}

		if _, err := conn.NewRaw("COMMIT").Exec(ctx); err != nil {
			return fmt.Errorf("cron store: commit migration: %w", err)
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
		return nil, fmt.Errorf("cron store: read busy timeout: %w", err)
	}

	if _, err := conn.NewRaw("PRAGMA busy_timeout = 0").Exec(ctx); err != nil {
		return nil, fmt.Errorf("cron store: suspend busy timeout: %w", err)
	}

	return func(resultErr *error) {
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), migrationCleanupTimeout)
		defer cancel()

		restore := fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout)
		if _, err := conn.NewRaw(restore).Exec(restoreCtx); err != nil {
			appendCleanupError(resultErr, fmt.Errorf("cron store: restore busy timeout: %w", err))
		}
	}, nil
}

// isSQLiteBusy reports the driver's lock contention error anywhere in the
// attempt: acquiring the pooled connection (a fresh connection's DSN pragmas
// can contend with the lock holder), BEGIN IMMEDIATE, or the commit.
func isSQLiteBusy(err error) bool {
	message := err.Error()

	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "SQLITE_BUSY")
}

func mysqlLockResult(ctx context.Context, db orm.DB, query string) (sql.NullInt64, error) {
	var result sql.NullInt64

	err := db.NewRaw(query).Scan(ctx, &result)

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
