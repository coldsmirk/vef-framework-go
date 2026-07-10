package approval

import (
	"context"
	"encoding/json"
)

// FormSchemaParser derives the flat field list the framework consumes from the
// host-owned form schema document. The framework stores and returns the schema
// verbatim and never interprets it; only the parsed fields enter form
// validation, storage-table DDL, aggregate checks, and field-permission
// resolution. Parsing runs once at flow deploy; runtime paths read the
// persisted fields, so parser upgrades never affect already-deployed versions.
type FormSchemaParser interface {
	// ParseFormFields extracts the flat field definitions from the raw form
	// schema document. A nil or empty schema yields (nil, nil) — a flow
	// without a form. Errors abort the deploy.
	//
	// ctx carries the deploy request's deadline and cancellation. The built-in
	// parser is pure and ignores it, but a host parser may perform I/O (resolving
	// remote option sources, calling an external validation service) while
	// deriving fields, and must honor ctx cancellation on those paths.
	ParseFormFields(ctx context.Context, schema json.RawMessage) ([]FormFieldDefinition, error)
}
