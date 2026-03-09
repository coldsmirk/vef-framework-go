package orm

import (
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/internal/log"
)

// logger is the named logger instance for the Orm package.
var logger = log.Named("orm")

// New creates a new DB instance that wraps the provided bun.IDB.
// This function is used by the dependency injection system to provide DB instances.
func New(db bun.IDB) DB {
	inst := &BunDB{db: db}

	return inst.WithNamedArg(ExprOperator, OperatorSystem)
}
