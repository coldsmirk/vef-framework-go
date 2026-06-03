package orm

import (
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

// logger is the named logger instance for the Orm package.
var logger = logx.Named("orm")

// New creates a new DB instance that wraps the provided bun.IDB.
// This function is used by the dependency injection system to provide DB instances.
// It binds the default "system" operator under PlaceholderKeyOperator so audit
// columns resolve even before a request overrides it with the real principal
// (see api/middleware/contextual.go). The key must be the bare "Operator" — bun
// matches the ?Operator placeholder against the registered name without the '?'.
func New(db bun.IDB) DB {
	return newBunDB(db).WithNamedArg(PlaceholderKeyOperator, OperatorSystem)
}
