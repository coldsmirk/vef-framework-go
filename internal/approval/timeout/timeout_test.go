package timeout_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/approval/migration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// registry holds all timeout test suite factories, populated by init() in each suite file.
var registry = testx.NewRegistry[testx.DBEnv]()

// baseFactory runs approval migrations and returns the DBEnv.
func baseFactory(env *testx.DBEnv) *testx.DBEnv {
	require.NoError(env.T, migration.Migrate(env.Ctx, env.DB, env.DS.Kind), "Should run approval migration")

	return env
}

// TestAll runs every registered timeout suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}
