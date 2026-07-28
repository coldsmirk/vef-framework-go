package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
