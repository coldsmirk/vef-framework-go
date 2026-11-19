package migration

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func TestVerify(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind),
			"The approval schema should provision for %s", env.DS.Kind)

		t.Run("AcceptsAFreshlyMigratedSchema", func(t *testing.T) {
			assert.NoError(t, Verify(env.Ctx, env.DB, env.DS.Kind),
				"A schema this migration just created must verify for %s", env.DS.Kind)
		})

		t.Run("UpgradesSubmissionKeys", func(t *testing.T) {
			_, err := env.DB.NewRaw("DROP TABLE apv_submission_key").Exec(env.Ctx)
			require.NoError(t, err, "Should emulate a previous installation")
			require.ErrorIs(t, Verify(env.Ctx, env.DB, env.DS.Kind), ErrSchemaOutdated, "A missing reservation table must fail verification")
			for range 2 {
				require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind), "Reservation migration must be replayable")
			}
			pk, err := sqlmigration.LoadTablePrimaryKey(env.Ctx, env.DB, env.DS.Kind, "apv_submission_key")
			require.NoError(t, err, "Should read the reservation key")
			assert.Equal(t, []string{"tenant_id", "idempotency_key"}, pk, "The tenant/key pair must be unique")
			require.NoError(t, Verify(env.Ctx, env.DB, env.DS.Kind), "The upgraded schema should verify")
		})
		t.Run("UpgradesTheAnnotationTable", func(t *testing.T) {
			const table = "apv_annotation"

			fresh, err := sqlmigration.LoadTableColumns(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect a fresh annotation table")

			for _, col := range []string{"visit_id", "task_id", "anchor", "parent_id", "resolution", "published_at", "author_department_id", "author_department_name"} {
				require.Contains(t, fresh, col, "Schema must declare %s", col)
				assert.True(t, fresh[col].Nullable, "%s must allow NULL", col)
			}

			_, err = env.DB.NewRaw("DROP TABLE apv_annotation").Exec(env.Ctx)
			require.NoError(t, err, "Should emulate the schema before annotations")
			require.ErrorIs(t, Verify(env.Ctx, env.DB, env.DS.Kind), ErrSchemaOutdated, "Missing annotations must fail verification")
			require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind), "The upgrade should create the annotation table")
			upgraded, err := sqlmigration.LoadTableColumns(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect the upgraded table")
			assert.Equal(t, fresh, upgraded, "Fresh and upgraded annotation columns must agree")

			indexes, err := sqlmigration.LoadTableIndexes(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect annotation indexes")

			for _, cols := range [][]string{{"instance_id", "status"}, {"task_id"}, {"parent_id"}} {
				assert.True(t, slices.ContainsFunc(indexes, func(i sqlmigration.Index) bool { return slices.Equal(i.Columns, cols) }), "Should index %v", cols)
			}
		})

		t.Run("UpgradesVoidPolicy", func(t *testing.T) {
			const table = "apv_flow"

			fresh, err := sqlmigration.LoadTableColumns(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect freshly created policies")

			for _, name := range []string{"void_policy", "void_flow_code"} {
				require.Contains(t, fresh, name, "Fresh schema must contain %s", name)
				_, err = env.DB.NewRaw("ALTER TABLE apv_flow DROP COLUMN " + name).Exec(env.Ctx)
				require.NoError(t, err, "Should recreate the schema before %s", name)
			}

			require.ErrorIs(t, Verify(env.Ctx, env.DB, env.DS.Kind), ErrSchemaOutdated, "Both missing policy columns require an upgrade")
			require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind), "Should add the live policy to existing flows")
			upgraded, err := sqlmigration.LoadTableColumns(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect upgraded policies")

			for _, name := range []string{"void_policy", "void_flow_code"} {
				require.Equal(t, fresh[name], upgraded[name], "Upgraded and fresh %s definitions must agree", name)
			}
		})
		t.Run("UpgradesVoidRecords", func(t *testing.T) {
			const table = "apv_void_record"

			fresh, err := sqlmigration.LoadTableColumns(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect fresh void records")
			_, err = env.DB.NewRaw("DROP TABLE apv_void_record").Exec(env.Ctx)
			require.NoError(t, err, "Should simulate an older database")
			require.ErrorIs(t, Verify(env.Ctx, env.DB, env.DS.Kind), ErrSchemaOutdated, "Missing void records must fail verification")
			require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind), "Should upgrade an older database")
			upgraded, err := sqlmigration.LoadTableColumns(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect upgraded records")
			assert.Equal(t, fresh, upgraded, "Fresh and upgraded void records must agree")

			indexes, err := sqlmigration.LoadTableIndexes(env.Ctx, env.DB, env.DS.Kind, table)
			require.NoError(t, err, "Should inspect void record indexes")

			for _, cols := range [][]string{{"instance_id", "status"}, {"request_instance_id"}, {"tenant_id", "status"}} {
				assert.True(t, slices.ContainsFunc(indexes, func(i sqlmigration.Index) bool { return slices.Equal(i.Columns, cols) }), "Should index %v", cols)
			}
		})
		t.Run("RejectsANarrowKindColumn", func(t *testing.T) {
			// The shape every database provisioned before the kind
			// vocabularies became open registries carries: wide enough for the
			// short built-ins, too narrow for `department_leader` — let alone a
			// host kind — so deploying such a flow failed at the insert with a
			// driver-level "value too long" naming neither the kind nor the fix.
			if env.DS.Kind == config.SQLite {
				t.Skip("SQLite does not enforce a declared VARCHAR bound")
			}

			_, err := env.DB.NewRaw("DROP TABLE IF EXISTS apv_flow_node_cc").Exec(env.Ctx)
			require.NoError(t, err, "The fixture table should drop for %s", env.DS.Kind)

			_, err = env.DB.NewRaw(`CREATE TABLE apv_flow_node_cc (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    node_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL
)`).Exec(env.Ctx)
			require.NoError(t, err, "The narrow fixture table should be created for %s", env.DS.Kind)

			err = Verify(env.Ctx, env.DB, env.DS.Kind)
			require.Error(t, err, "A narrow kind column must fail verification for %s", env.DS.Kind)
			assert.ErrorIs(t, err, ErrSchemaOutdated,
				"The failure must classify as an outdated schema for %s", env.DS.Kind)
			assert.Contains(t, err.Error(), "apv_flow_node_cc.kind",
				"The failure must name the offending column for %s", env.DS.Kind)

			// Restore the table so the remaining subtests see a sound schema.
			_, err = env.DB.NewRaw("DROP TABLE IF EXISTS apv_flow_node_cc").Exec(env.Ctx)
			require.NoError(t, err, "The fixture table should drop for %s", env.DS.Kind)
			require.NoError(t, Migrate(env.Ctx, env.DB, env.DS.Kind),
				"The approval schema should re-provision for %s", env.DS.Kind)
		})

		t.Run("RejectsATableWithoutAPrimaryKey", func(t *testing.T) {
			// The shape a dump restored without constraints leaves behind: the
			// table is present and correctly named, so CREATE TABLE IF NOT
			// EXISTS skips it and only this check can catch it. Rows then stop
			// being uniquely addressable, which breaks the engine's
			// compare-and-set writes.
			_, err := env.DB.NewRaw("DROP TABLE IF EXISTS apv_urge_record").Exec(env.Ctx)
			require.NoError(t, err, "The fixture table should drop for %s", env.DS.Kind)

			_, err = env.DB.NewRaw(`CREATE TABLE apv_urge_record (
    id VARCHAR(32) NOT NULL,
    tenant_id VARCHAR(32) NOT NULL
)`).Exec(env.Ctx)
			require.NoError(t, err, "The keyless fixture table should be created for %s", env.DS.Kind)

			err = Verify(env.Ctx, env.DB, env.DS.Kind)
			require.Error(t, err, "A table without a primary key must fail verification for %s", env.DS.Kind)
			assert.ErrorIs(t, err, ErrSchemaOutdated,
				"The failure must classify as an outdated schema for %s", env.DS.Kind)
			assert.Contains(t, err.Error(), "apv_urge_record",
				"The failure must name the offending table for %s", env.DS.Kind)
		})

		t.Run("RejectsAMissingTable", func(t *testing.T) {
			_, err := env.DB.NewRaw("DROP TABLE IF EXISTS apv_urge_record").Exec(env.Ctx)
			require.NoError(t, err, "The fixture table should drop for %s", env.DS.Kind)

			err = Verify(env.Ctx, env.DB, env.DS.Kind)
			require.Error(t, err, "A missing table must fail verification for %s", env.DS.Kind)
			assert.ErrorIs(t, err, ErrSchemaOutdated,
				"The failure must classify as an outdated schema for %s", env.DS.Kind)
		})
	})
}
