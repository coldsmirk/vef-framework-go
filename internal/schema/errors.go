package schema

import "errors"

// ErrTableMissing is returned when a table does not exist.
var ErrTableMissing = errors.New("table not found")

var errUnsupportedDBKind = errors.New("unsupported database type")
