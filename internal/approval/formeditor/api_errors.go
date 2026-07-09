package formeditor

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Projection faults surface to the deploy API caller. They are all "invalid
// form design" outcomes, so they reuse shared.ErrCodeInvalidFormDesign (and
// therefore match shared.ErrInvalidFormDesign under errors.Is) while carrying a
// specific i18n message that names the offending field key / widget type. Each
// i18n key is used in exactly one factory, so the key strings are inlined per
// the module convention rather than promoted to ErrMessage constants.

func projectionError(message string) error {
	return result.Err(message, result.WithCode(shared.ErrCodeInvalidFormDesign))
}

// errUnmappableFieldType rejects a keyed widget whose value shape the approval
// contract cannot carry (switch / daterange).
func errUnmappableFieldType(path, widgetType string) error {
	return projectionError(i18n.T("approval_form_unmappable_field_type", map[string]any{
		"field": path,
		"type":  widgetType,
	}))
}

// errUnknownFieldType rejects a keyed widget of an unregistered type — the form
// data is a closed contract, so an unprojected field would fail every submit.
func errUnknownFieldType(path, widgetType string) error {
	return projectionError(i18n.T("approval_form_unknown_field_type", map[string]any{
		"field": path,
		"type":  widgetType,
	}))
}

// errNestedSubform rejects a subform nested inside a detail-table template —
// approval detail tables are single-level only.
func errNestedSubform(path string) error {
	return projectionError(i18n.T("approval_form_nested_subform", map[string]any{"field": path}))
}

// errTableColumnsEmpty rejects a detail table whose template yields no columns.
func errTableColumnsEmpty(path string) error {
	return projectionError(i18n.T("approval_form_table_columns_empty", map[string]any{"field": path}))
}

// errCrossDeviceKindMismatch rejects a key that projects to different kinds on
// pc and mobile — the losing device would submit a value the definition rejects.
func errCrossDeviceKindMismatch(path string, pc, mobile approval.FieldKind) error {
	return projectionError(i18n.T("approval_form_cross_device_kind_mismatch", map[string]any{
		"field":  path,
		"pc":     string(pc),
		"mobile": string(mobile),
	}))
}

// errCrossDeviceTableMismatch rejects a detail table whose column set differs
// across pc and mobile — one device would submit rows with undefined columns.
func errCrossDeviceTableMismatch(path string) error {
	return projectionError(i18n.T("approval_form_cross_device_table_mismatch", map[string]any{"field": path}))
}

// errSchemaMalformed rejects a form schema that is not valid JSON.
func errSchemaMalformed(cause error) error {
	return projectionError(i18n.T("approval_form_schema_malformed", map[string]any{"error": cause.Error()}))
}
