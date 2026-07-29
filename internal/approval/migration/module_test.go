package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// TestRegisterMigration pins the start-up hook's contract: the schema work runs
// from the lifecycle (not from the fx.Invoke call, which has no context to run
// with), and verification runs on every boot rather than only when auto-migration
// is enabled. Every subtest establishes the schema it needs, so running one
// under -run cannot pass for a reason the whole test would not have found.
func TestRegisterMigration(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		sources := &config.DataSourcesConfig{
			Map: map[string]config.DataSourceConfig{config.PrimaryDataSourceName: *env.DS},
		}

		start := func(t *testing.T, autoMigrate bool) error {
			t.Helper()

			lc := fxtest.NewLifecycle(t)
			registerMigration(lc, &config.ApprovalConfig{AutoMigrate: autoMigrate}, env.DB, sources)

			return lc.Start(env.Ctx)
		}

		provision := func(t *testing.T) {
			t.Helper()

			require.NoError(t, start(t, true),
				"The schema should provision for %s", env.DS.Kind)
		}

		t.Run("ProvisionsAndVerifiesWhenAutoMigrateIsOn", func(t *testing.T) {
			provision(t)
		})

		t.Run("VerifiesAnExistingSchemaWhenAutoMigrateIsOff", func(t *testing.T) {
			provision(t)

			require.NoError(t, start(t, false),
				"A provisioned schema should verify without auto-migration for %s", env.DS.Kind)
		})

		t.Run("FailsOnADamagedSchemaWhenAutoMigrateIsOff", func(t *testing.T) {
			// The regression this guards: copying storage's "return early when
			// AutoMigrate is off" shape would skip Verify entirely, and a schema
			// restored without its constraints would reach the engine's
			// compare-and-set writes unchecked.
			provision(t)

			_, err := env.DB.NewRaw("DROP TABLE IF EXISTS apv_urge_record").Exec(env.Ctx)
			require.NoError(t, err, "The fixture table should drop for %s", env.DS.Kind)

			err = start(t, false)
			require.Error(t, err, "A damaged schema must abort start-up for %s", env.DS.Kind)
			assert.ErrorIs(t, err, ErrSchemaOutdated,
				"The failure must classify as an outdated schema for %s", env.DS.Kind)
			assert.Contains(t, err.Error(), "apv_urge_record",
				"The failure must name the damaged table, not merely any missing one, for %s", env.DS.Kind)
		})
	})
}
