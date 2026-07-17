package jssql

import "errors"

var (
	// ErrExecuteDisabled is thrown into the script when sql.execute is called on a
	// library built without WithExecute.
	ErrExecuteDisabled = errors.New("jssql: execute disabled")
	// ErrQueryNotReadOnly is thrown into the script when sql.queryList receives a
	// statement the read-only guard rejects: a non-read statement, a
	// data-modifying CTE, a dialect-specific side-effecting function, or SQL
	// the parser cannot understand (the guard fails closed).
	ErrQueryNotReadOnly = errors.New("jssql: statement is not read-only")
	// ErrTooManyRows is thrown into the script when a query yields more rows
	// than the configured limit.
	ErrTooManyRows = errors.New("jssql: result exceeds row limit")
)
