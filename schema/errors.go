package schema

import "errors"

// ErrTableMissing indicates that schema inspection could not find the
// requested table in the current database/schema.
var ErrTableMissing = errors.New("schema: table not found")
