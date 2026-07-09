package approval

import "encoding/json"

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
	ParseFormFields(schema json.RawMessage) ([]FormFieldDefinition, error)
}
