package sqlmigration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func TestTableExists(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		exists, err := TableExists(env.Ctx, env.DB, env.DS.Kind, "smig_probe")
		require.NoError(t, err, "Probing an absent table should succeed for %s", env.DS.Kind)
		assert.False(t, exists, "An absent table must probe false for %s", env.DS.Kind)

		_, err = env.DB.NewRaw("CREATE TABLE smig_probe (id VARCHAR(32) NOT NULL PRIMARY KEY)").Exec(env.Ctx)
		require.NoError(t, err, "The probe fixture table should be created for %s", env.DS.Kind)

		exists, err = TableExists(env.Ctx, env.DB, env.DS.Kind, "smig_probe")
		require.NoError(t, err, "Probing an existing table should succeed for %s", env.DS.Kind)
		assert.True(t, exists, "An existing table must probe true for %s", env.DS.Kind)
	})
}

func TestCountTables(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		_, err := env.DB.NewRaw("CREATE TABLE smig_present (id VARCHAR(32) NOT NULL PRIMARY KEY)").Exec(env.Ctx)
		require.NoError(t, err, "The count fixture table should be created for %s", env.DS.Kind)

		count, err := CountTables(env.Ctx, env.DB, env.DS.Kind, []string{"smig_present", "smig_absent"})
		require.NoError(t, err, "Counting a mixed table set should succeed for %s", env.DS.Kind)
		assert.Equal(t, 1, count, "Only the existing table must be counted for %s", env.DS.Kind)
	})
}

func TestCountTablesRejectsUnknownKind(t *testing.T) {
	db := testx.NewTestDB(t)

	_, err := CountTables(t.Context(), db, "oracle-ish", []string{"any"})
	assert.ErrorIs(t, err, ErrUnsupportedDBKind, "An unknown dialect must fail with the sentinel")
}

func TestTableExistsFollowsTheActiveSchema(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		if env.DS.Kind != config.Postgres {
			t.Skipf("Only Postgres separates the active schema from a fixed default; %s has no equivalent", env.DS.Kind)
		}

		// Unqualified DDL lands in the connection's active schema, so the
		// probe must follow current_schema() instead of assuming 'public'.
		err := env.DB.RunOnConnection(env.Ctx, func(ctx context.Context, conn orm.DB) error {
			if _, err := conn.NewRaw("CREATE SCHEMA smig_tenant").Exec(ctx); err != nil {
				return err
			}

			if _, err := conn.NewRaw("SET search_path TO smig_tenant").Exec(ctx); err != nil {
				return err
			}

			if _, err := conn.NewRaw("CREATE TABLE smig_scoped (id VARCHAR(32) NOT NULL PRIMARY KEY)").Exec(ctx); err != nil {
				return err
			}

			exists, err := TableExists(ctx, conn, config.Postgres, "smig_scoped")
			if err != nil {
				return err
			}

			assert.True(t, exists,
				"A table created in the active non-public schema must be visible to the probe")

			return nil
		})
		require.NoError(t, err, "The scoped-schema probe should run cleanly")
	})
}
