package migration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// columnExists probes one column through the dialect metadata loader.
func columnExists(ctx context.Context, db orm.DB, kind config.DBKind, table, column string) (bool, error) {
	columns, err := loadTableColumns(ctx, db, kind, table)
	if err != nil {
		return false, err
	}

	_, exists := columns[column]

	return exists, nil
}

// indexCapabilityExists probes one index capability through the dialect
// metadata loader.
func indexCapabilityExists(ctx context.Context, db orm.DB, kind config.DBKind, expected schemaIndex) (bool, error) {
	indexes, err := loadTableIndexes(ctx, db, kind, expected.table)
	if err != nil {
		return false, err
	}

	return hasIndexCapability(indexes, expected), nil
}

func TestMigrateFreshSchemaIsIdempotent(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		const workerCount = 8

		ctx, cancel := context.WithTimeout(env.Ctx, 30*time.Second)
		defer cancel()

		type MigrationResult struct {
			worker int
			err    error
		}

		start := make(chan struct{})

		results := make(chan MigrationResult, workerCount)
		for worker := range workerCount {
			go func() {
				<-start

				results <- MigrationResult{
					worker: worker,
					err:    Migrate(ctx, env.DB, env.DS.Kind),
				}
			}()
		}

		close(start)

		errs := make([]error, workerCount)
		for range workerCount {
			result := <-results
			errs[result.worker] = result.err
		}

		for worker, err := range errs {
			require.NoError(t, err,
				"Concurrent cron migration worker %d should succeed for %s", worker, env.DS.Kind)
		}

		// Migrate ends in Verify, so a passing replay already proves every
		// required column and index capability plus the absence of obsolete
		// timeline columns; re-probing them here would only duplicate it.
		require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind),
			"Replaying the cron migration should be harmless for %s", env.DS.Kind)

		if env.DS.Kind == config.MySQL {
			_, err := env.DB.NewRaw(`ALTER TABLE crn_run
MODIFY scheduled_at_unix_ms BIGINT UNSIGNED NOT NULL`).Exec(env.Ctx)
			require.NoError(t, err, "The MySQL timeline fixture should become unsigned")

			err = Verify(env.Ctx, env.DB, env.DS.Kind)
			require.ErrorIs(t, err, ErrSchemaOutdated,
				"An unsigned MySQL timeline must not satisfy the signed Unix-millisecond contract")
			assert.ErrorContains(t, err, "crn_run.scheduled_at_unix_ms",
				"The verification error should identify the unsigned timeline column")
		}
	})
}

func TestMigrateRejectsPartialSchemaWithoutMutation(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		_, err := env.DB.NewRaw("CREATE TABLE crn_schedule (sentinel TEXT)").Exec(env.Ctx)
		require.NoError(t, err, "The partial cron schema fixture should be created for %s", env.DS.Kind)

		err = Migrate(env.Ctx, env.DB, env.DS.Kind)
		require.ErrorIs(t, err, ErrSchemaOutdated,
			"A partial cron schema should fail instead of being repaired for %s", env.DS.Kind)

		for _, table := range []string{"crn_fire_request", "crn_run"} {
			exists, probeErr := sqlmigration.TableExists(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, probeErr, "Table metadata should remain readable for %s on %s", table, env.DS.Kind)
			assert.False(t, exists,
				"Migration should not create missing table %s in a partial %s schema", table, env.DS.Kind)
		}

		sentinelExists, probeErr := columnExists(env.Ctx, env.DB, env.DS.Kind, "crn_schedule", "sentinel")
		require.NoError(t, probeErr, "Partial table metadata should remain readable for %s", env.DS.Kind)
		assert.True(t, sentinelExists, "Migration should leave the existing partial table unchanged for %s", env.DS.Kind)
	})
}

func TestMigrateWaitsForMigrationLockBeforeProvisioning(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		holderCtx, cancelHolder := context.WithTimeout(env.Ctx, 15*time.Second)
		acquired := make(chan struct{})
		release := make(chan struct{})

		holderResult := make(chan error, 1)
		go func() {
			holderResult <- withMigrationLock(holderCtx, env.DB, env.DS.Kind,
				func(ctx context.Context, _ orm.DB) error {
					close(acquired)

					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
		}()

		var releaseOnce sync.Once

		releaseHolder := func() {
			releaseOnce.Do(func() {
				close(release)
			})
		}

		var (
			waitOnce  sync.Once
			holderErr error
		)

		waitForHolder := func() error {
			waitOnce.Do(func() {
				holderErr = <-holderResult
			})

			return holderErr
		}
		t.Cleanup(func() {
			cancelHolder()
			releaseHolder()

			_ = waitForHolder()
		})

		select {
		case <-acquired:
		case <-holderCtx.Done():
			require.NoError(t, holderCtx.Err(),
				"Migration lock holder should acquire the native lock for %s", env.DS.Kind)
		}

		blockedCtx, cancelBlocked := context.WithTimeout(env.Ctx, 250*time.Millisecond)
		startedAt := time.Now()
		err := Migrate(blockedCtx, env.DB, env.DS.Kind)
		elapsed := time.Since(startedAt)

		cancelBlocked()
		require.Error(t, err, "Migration should fail when its lock deadline expires for %s", env.DS.Kind)
		assert.Less(t, elapsed, 2*time.Second,
			"Migration lock wait should respect the short deadline for %s", env.DS.Kind)

		for _, table := range expectedTables {
			exists, probeErr := sqlmigration.TableExists(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, probeErr, "Table metadata should remain readable for %s on %s", table, env.DS.Kind)
			assert.False(t, exists,
				"Blocked migration should not create table %s before acquiring the %s lock", table, env.DS.Kind)
		}

		releaseHolder()
		require.NoError(t, waitForHolder(), "Migration lock holder should release cleanly for %s", env.DS.Kind)
		require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind),
			"Migration should succeed after the native lock is released for %s", env.DS.Kind)
	})
}

func TestMigrateRejectsIncompatibleSchemaWithoutRepair(t *testing.T) {
	t.Run("MissingColumn", func(t *testing.T) {
		db := testx.NewTestDB(t)
		ctx := context.Background()

		require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
		_, err := db.NewRaw("ALTER TABLE crn_schedule DROP COLUMN fire_at_unix_ms").Exec(ctx)
		require.NoError(t, err, "Fixture timeline column should be removed")

		err = Migrate(ctx, db, config.SQLite)
		require.ErrorIs(t, err, ErrSchemaOutdated,
			"Migration should reject an existing schema with a missing column")
		assert.ErrorContains(t, err, "crn_schedule.fire_at_unix_ms",
			"Migration error should identify the missing column")

		exists, probeErr := columnExists(ctx, db, config.SQLite, "crn_schedule", "fire_at_unix_ms")
		require.NoError(t, probeErr, "Column metadata should remain readable")
		assert.False(t, exists, "Migration should not restore a missing column")
	})

	t.Run("MissingIndex", func(t *testing.T) {
		db := testx.NewTestDB(t)
		ctx := context.Background()

		require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
		_, err := db.NewRaw("DROP INDEX idx_crn_run__claimed_at_unix_ms").Exec(ctx)
		require.NoError(t, err, "Fixture claim index should be removed")

		err = Migrate(ctx, db, config.SQLite)
		require.ErrorIs(t, err, ErrSchemaOutdated,
			"Migration should reject an existing schema with a missing index")
		assert.ErrorContains(t, err, "idx_crn_run__claimed_at_unix_ms",
			"Migration error should identify the missing index capability")

		exists, probeErr := indexCapabilityExists(ctx, db, config.SQLite, schemaIndex{
			table:   "crn_run",
			columns: []string{"claimed_at_unix_ms", "id"},
		})
		require.NoError(t, probeErr, "Index metadata should remain readable")
		assert.False(t, exists, "Migration should not restore a missing index")
	})

	t.Run("NonUniqueRecoveryFence", func(t *testing.T) {
		db := testx.NewTestDB(t)
		ctx := context.Background()

		require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
		_, err := db.NewRaw("DROP TABLE crn_fire_request").Exec(ctx)
		require.NoError(t, err, "Valid fire request table should be removed")
		_, err = db.NewRaw(`CREATE TABLE crn_fire_request (
	    id VARCHAR(32) NOT NULL CONSTRAINT pk_crn_fire_request PRIMARY KEY,
    schedule_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    scheduled_at_unix_ms BIGINT NOT NULL,
    source_run_id VARCHAR(32)
)`).Exec(ctx)
		require.NoError(t, err, "Fire request table without a recovery fence should be created")
		_, err = db.NewRaw(`CREATE INDEX uk_crn_fire_request__source_run_id
    ON crn_fire_request(source_run_id)`).Exec(ctx)
		require.NoError(t, err, "Same-named non-unique recovery index should be created")
		_, err = db.NewRaw(`CREATE INDEX idx_crn_fire_request__schedule_id_scheduled_at_unix_ms
    ON crn_fire_request(schedule_id, scheduled_at_unix_ms, id)`).Exec(ctx)
		require.NoError(t, err, "Fire request claim index should be created")

		err = Migrate(ctx, db, config.SQLite)
		require.ErrorIs(t, err, ErrSchemaOutdated,
			"Migration should reject a non-unique recovery fence")
		assert.ErrorContains(t, err, "unique",
			"Migration error should identify the missing unique capability")
	})

	t.Run("ObsoleteTimelineColumn", func(t *testing.T) {
		db := testx.NewTestDB(t)
		ctx := context.Background()

		require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
		_, err := db.NewRaw("ALTER TABLE crn_schedule ADD COLUMN fire_at TIMESTAMP").Exec(ctx)
		require.NoError(t, err, "Obsolete wall timeline column should be added")

		err = Migrate(ctx, db, config.SQLite)
		require.ErrorIs(t, err, ErrSchemaOutdated,
			"Migration should reject a schema retaining an obsolete timeline column")
		assert.ErrorContains(t, err, "obsolete column crn_schedule.fire_at",
			"Migration error should identify the obsolete timeline column")
	})

	columnCases := []struct {
		name            string
		scheduledColumn string
		wantDetail      string
	}{
		{
			name:            "ColumnTypeMismatch",
			scheduledColumn: "scheduled_at_unix_ms TEXT NOT NULL",
			wantDetail:      "has type text, want int64",
		},
		{
			name:            "ColumnNullabilityMismatch",
			scheduledColumn: "scheduled_at_unix_ms BIGINT",
			wantDetail:      "nullable=true, want nullable=false",
		},
	}

	for _, tc := range columnCases {
		t.Run(tc.name, func(t *testing.T) {
			db := testx.NewTestDB(t)
			ctx := context.Background()

			require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
			_, err := db.NewRaw("DROP TABLE crn_fire_request").Exec(ctx)
			require.NoError(t, err, "Valid fire request table should be removed")

			ddl := fmt.Sprintf(`CREATE TABLE crn_fire_request (
    id VARCHAR(32) NOT NULL CONSTRAINT pk_crn_fire_request PRIMARY KEY,
    schedule_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    %s,
    source_run_id VARCHAR(32),
    CONSTRAINT uk_crn_fire_request__source_run_id UNIQUE (source_run_id)
);
CREATE INDEX idx_crn_fire_request__schedule_id_scheduled_at_unix_ms
    ON crn_fire_request(schedule_id, scheduled_at_unix_ms, id)`, tc.scheduledColumn)
			_, err = db.NewRaw(ddl).Exec(ctx)
			require.NoError(t, err, "Incompatible fire request table should be created")

			err = Migrate(ctx, db, config.SQLite)
			require.ErrorIs(t, err, ErrSchemaOutdated,
				"Migration should reject an incompatible column capability")
			assert.ErrorContains(t, err, "crn_fire_request.scheduled_at_unix_ms",
				"Migration error should identify the incompatible column")
			assert.ErrorContains(t, err, tc.wantDetail,
				"Migration error should identify the incompatible capability")
		})
	}

	t.Run("ColumnWidthMismatch", func(t *testing.T) {
		db := testx.NewTestDB(t)
		ctx := context.Background()

		require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
		_, err := db.NewRaw("DROP TABLE crn_fire_request").Exec(ctx)
		require.NoError(t, err, "Valid fire request table should be removed")
		_, err = db.NewRaw(`CREATE TABLE crn_fire_request (
    id VARCHAR(32) NOT NULL CONSTRAINT pk_crn_fire_request PRIMARY KEY,
    schedule_id VARCHAR(31) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    scheduled_at_unix_ms BIGINT NOT NULL,
    source_run_id VARCHAR(32),
    CONSTRAINT uk_crn_fire_request__source_run_id UNIQUE (source_run_id)
);
CREATE INDEX idx_crn_fire_request__schedule_id_scheduled_at_unix_ms
    ON crn_fire_request(schedule_id, scheduled_at_unix_ms, id)`).Exec(ctx)
		require.NoError(t, err, "Fire request table with a narrow schedule ID should be created")

		err = Migrate(ctx, db, config.SQLite)
		require.ErrorIs(t, err, ErrSchemaOutdated,
			"Migration should reject a VARCHAR narrower than the persisted contract")
		assert.ErrorContains(t, err, "crn_fire_request.schedule_id has length 31, want at least 32",
			"Migration error should identify the incompatible column width")
	})

	t.Run("WiderColumnIsAccepted", func(t *testing.T) {
		db := testx.NewTestDB(t)
		ctx := context.Background()

		require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
		_, err := db.NewRaw("DROP TABLE crn_fire_request").Exec(ctx)
		require.NoError(t, err, "Valid fire request table should be removed")
		_, err = db.NewRaw(`CREATE TABLE crn_fire_request (
    id VARCHAR(32) NOT NULL CONSTRAINT pk_crn_fire_request PRIMARY KEY,
    schedule_id VARCHAR(64) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    scheduled_at_unix_ms BIGINT NOT NULL,
    source_run_id VARCHAR(32),
    CONSTRAINT uk_crn_fire_request__source_run_id UNIQUE (source_run_id)
);
CREATE INDEX idx_crn_fire_request__schedule_id_scheduled_at_unix_ms
    ON crn_fire_request(schedule_id, scheduled_at_unix_ms, id)`).Exec(ctx)
		require.NoError(t, err, "Fire request table with a widened schedule ID should be created")

		assert.NoError(t, Migrate(ctx, db, config.SQLite),
			"A DBA-widened column keeps every capability and must verify")
	})

	t.Run("SQLitePrimaryKeysRejectNull", func(t *testing.T) {
		db := testx.NewTestDB(t)
		ctx := context.Background()

		require.NoError(t, Migrate(ctx, db, config.SQLite), "Fixture cron schema should migrate")
		_, err := db.NewRaw(`INSERT INTO crn_fire_request
    (id, schedule_id, kind, scheduled_at_unix_ms)
VALUES (NULL, 'schedule', 'manual', 0)`).Exec(ctx)
		require.Error(t, err, "A SQLite fire request must reject a NULL primary key")
	})
}
