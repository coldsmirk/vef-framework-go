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
type DBEnv struct {
	T     *testing.T
	Ctx   context.Context
	RawDB *sql.DB
	BunDB *bun.DB
	DB    orm.DB
	DS    *config.DataSourceConfig
}
