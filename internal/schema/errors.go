package schema

import "errors"

// ErrTableNotFound is returned when a table does not exist.
var ErrTableNotFound = errors.New("table not found")
