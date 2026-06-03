package testx

import (
	"context"
	"database/sql"
	"testing"

	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// DBEnv encapsulates the database environment for cross-database integration tests.
// Contains both the raw sql.DB connection and a wrapped orm.DB for convenience.
//
// Ctx is a deliberate test-only exception to the "don't store context on a struct"
// guideline: it is set to context.Background() by ForEachDB so suites that receive
// a *DBEnv can pass a ready-to-use context without threading one through every call.
// Production code must never follow this pattern.
type DBEnv struct {
	T     *testing.T
	Ctx   context.Context //nolint:containedctx // test-only ergonomic exception; see type doc
	RawDB *sql.DB
	BunDB *bun.DB
	DB    orm.DB
	DS    *config.DataSourceConfig
}
