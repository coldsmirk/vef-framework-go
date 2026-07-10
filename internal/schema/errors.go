package schema

import (
	"errors"

	pkgschema "github.com/coldsmirk/vef-framework-go/schema"
)

// ErrTableMissing is returned when a table does not exist.
var ErrTableMissing = pkgschema.ErrTableMissing

var errUnsupportedDBKind = errors.New("unsupported database type")
