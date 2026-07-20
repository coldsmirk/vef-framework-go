package sqlmigration

import (
	"context"
	"embed"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

//go:embed testdata/scripts/*.sql
var fixtureFS embed.FS

// fixtureScripts exposes the test DDL under the "scripts/<kind>.sql" layout
// Plan.Scripts expects.
func fixtureScripts(t *testing.T) fs.FS {
	t.Helper()

	scripts, err := fs.Sub(fixtureFS, "testdata")
	require.NoError(t, err, "The fixture script bundle should resolve")

	return scripts
}

func TestRunProvisionsExactlyOnceUnderConcurrency(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		const workerCount = 8

		plan := Plan{
			Label:          "sqlmigration fixture",
			Kind:           env.DS.Kind,
			Scripts:        fixtureScripts(t),
			ExpectedTables: []string{"smig_run_fixture"},
		}

		// The fixture DDL carries no IF NOT EXISTS: a second execution fails,
		// so only the migration lock keeps concurrent boots correct.
		start := make(chan struct{})

		results := make(chan error, workerCount)
		for range workerCount {
			go func() {
				<-start

				results <- Run(env.Ctx, env.DB, plan)
			}()
		}

		close(start)

		for range workerCount {
			require.NoError(t, <-results,
				"Every concurrent Run must succeed: one provisions, the rest observe for %s", env.DS.Kind)
		}

		require.NoError(t, Run(env.Ctx, env.DB, plan),
			"Replaying the migration should be a no-op for %s", env.DS.Kind)

		exists, err := TableExists(env.Ctx, env.DB, env.DS.Kind, "smig_run_fixture")
		require.NoError(t, err, "The provisioned table should be probeable for %s", env.DS.Kind)
		assert.True(t, exists, "The migration must leave the expected table behind for %s", env.DS.Kind)
	})
}

func TestRunExecutesPreHooksUnderTheLock(t *testing.T) {
	db := testx.NewTestDB(t)
	ctx := context.Background()

	_, err := db.NewRaw("CREATE TABLE smig_retired (id VARCHAR(32) NOT NULL PRIMARY KEY)").Exec(ctx)
	require.NoError(t, err, "The retired fixture table should be created")

	plan := Plan{
		Label:          "sqlmigration fixture",
		Kind:           "sqlite",
		Scripts:        fixtureScripts(t),
		ExpectedTables: []string{"smig_run_fixture"},
		Pre: []func(ctx context.Context, db orm.DB) error{
			func(ctx context.Context, db orm.DB) error {
				_, err := db.NewRaw("DROP TABLE IF EXISTS smig_retired").Exec(ctx)

				return err
			},
		},
	}

	require.NoError(t, Run(ctx, db, plan), "Run should execute the pre hook and provision")

	retired, err := TableExists(ctx, db, plan.Kind, "smig_retired")
	require.NoError(t, err, "The retired table should be probeable")
	assert.False(t, retired, "The pre hook must have dropped the retired table")

	provisioned, err := TableExists(ctx, db, plan.Kind, "smig_run_fixture")
	require.NoError(t, err, "The provisioned table should be probeable")
	assert.True(t, provisioned, "The migration must leave the expected table behind")
}
