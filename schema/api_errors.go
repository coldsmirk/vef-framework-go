package schema

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for schema API errors (2300-2399).
const (
	ErrCodeTableNotFound = 2300
)

// Predefined schema API errors.
var (
	ErrTableNotFound = result.Err(
		i18n.T("schema_table_not_found"),
		result.WithCode(ErrCodeTableNotFound),
	)
)
