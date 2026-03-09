package orm_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// registry holds all ORM test suite factories, populated by init() functions in each suite file.
var registry = testx.NewRegistry[BaseTestSuite]()

// baseFactory creates a BaseTestSuite from a DBEnv — called once per database.
func baseFactory(env *testx.DBEnv) *BaseTestSuite {
	setupTestFixtures(env.T, env.Ctx, env.DB, env.RawDB, env.DS.Kind)

	return &BaseTestSuite{
		ctx:   env.Ctx,
		db:    env.DB,
		rawDB: env.RawDB,
		ds:    env.DS,
	}
}

// setupTestFixtures creates tables and loads test data from fixture files.
func setupTestFixtures(t *testing.T, _ context.Context, db orm.DB, rawDB *sql.DB, kind config.DBKind) {
	t.Helper()

	models := []any{
		(*User)(nil),
		(*Post)(nil),
		(*Tag)(nil),
		(*PostTag)(nil),
		(*Category)(nil),
		(*Comment)(nil),
		(*UserFavorite)(nil),
	}

	db.RegisterModel(models...)
	require.NoError(t, db.ResetModel(context.Background(), models...), "Failed to reset models for %s", kind)

	fixtures, err := testfixtures.New(
		testfixtures.Database(rawDB),
		testfixtures.Dialect(string(kind)),
		testfixtures.Directory("fixtures"),
		testfixtures.DangerousSkipTestDatabaseCheck(),
	)
	require.NoError(t, err, "Failed to create fixtures loader for %s", kind)
	require.NoError(t, fixtures.Load(), "Failed to load fixtures for %s", kind)
}

// TestAll runs every registered ORM suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}
