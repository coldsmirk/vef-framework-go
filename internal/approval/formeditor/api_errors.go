package formeditor

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Projection faults surface to the deploy API caller. They are all "invalid
// form design" outcomes, so they reuse approval.ErrCodeInvalidFormDesign (and
// therefore match approval.ErrInvalidFormDesign under errors.Is) while carrying a
// specific i18n message that names the offending field key / widget type. Each
// i18n key is used in exactly one factory, so the key strings are inlined per
// the module convention rather than promoted to ErrMessage constants.

func projectionError(message string) error {
	return result.Err(message, result.WithCode(approval.ErrCodeInvalidFormDesign))
}

// errUnmappableFieldType rejects a keyed widget whose value shape the approval
// contract cannot carry (switch).
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

// errTableRowsModeInvalid rejects a detail table whose rows.mode is neither
// "dynamic" nor "fixed".
func errTableRowsModeInvalid(path, mode string) error {
	return projectionError(i18n.T("approval_form_table_rows_mode_invalid", map[string]any{
		"field": path,
		"mode":  mode,
	}))
}

// errTableFixedRowsItemKeyEmpty rejects a fixed-row detail table whose itemKey
// is missing or whitespace-only.
func errTableFixedRowsItemKeyEmpty(path string) error {
	return projectionError(i18n.T("approval_form_table_fixed_rows_item_key_empty", map[string]any{"field": path}))
}

// errTableFixedRowsItemKeyConflict rejects a fixed-row detail table whose
// itemKey collides with one of its template column keys.
func errTableFixedRowsItemKeyConflict(path, itemKey string) error {
	return projectionError(i18n.T("approval_form_table_fixed_rows_item_key_conflict", map[string]any{
		"field":   path,
		"itemKey": itemKey,
	}))
}

// errTableFixedRowsSourceInvalid rejects a fixed-row detail table whose item
// source is absent, dangling, or a static list with no items, blank values, or
// duplicate values.
func errTableFixedRowsSourceInvalid(path string) error {
	return projectionError(i18n.T("approval_form_table_fixed_rows_source_invalid", map[string]any{"field": path}))
}

// errCrossDeviceKindMismatch rejects a key that projects to different kinds,
// selection multiplicities, or date granularities on pc and mobile — the losing
// device would submit a value shape the definition rejects.
func errCrossDeviceKindMismatch(path, pc, mobile string) error {
	return projectionError(i18n.T("approval_form_cross_device_kind_mismatch", map[string]any{
		"field":  path,
		"pc":     pc,
		"mobile": mobile,
	}))
}

// errCrossDeviceOptionsMismatch rejects a field whose projected option source
// (static option list, code set, or remote request/mapping) differs across pc
// and mobile.
func errCrossDeviceOptionsMismatch(path string) error {
	return projectionError(i18n.T("approval_form_cross_device_options_mismatch", map[string]any{"field": path}))
}

// errCrossDeviceValidationMismatch rejects a field or table whose projected
// validation constraints (required, length/value bounds, pattern) differ across
// pc and mobile.
func errCrossDeviceValidationMismatch(path string) error {
	return projectionError(i18n.T("approval_form_cross_device_validation_mismatch", map[string]any{"field": path}))
}

// errCrossDeviceStorageMismatch rejects a field whose projected storage column
// type or decimal scale differs across pc and mobile.
func errCrossDeviceStorageMismatch(path, pc, mobile string) error {
	return projectionError(i18n.T("approval_form_cross_device_storage_mismatch", map[string]any{
		"field":  path,
		"pc":     pc,
		"mobile": mobile,
	}))
}

// errCrossDeviceTableMismatch rejects a detail table whose template column set
// differs across pc and mobile — one device would submit rows with undefined
// columns.
func errCrossDeviceTableMismatch(path string) error {
	return projectionError(i18n.T("approval_form_cross_device_table_mismatch", map[string]any{"field": path}))
}

// errCrossDeviceTableRowsMismatch rejects a detail table whose row mode
// (dynamic vs fixed) or fixed-row contract (itemKey, static items, or source)
// differs across pc and mobile.
func errCrossDeviceTableRowsMismatch(path string) error {
	return projectionError(i18n.T("approval_form_cross_device_table_rows_mismatch", map[string]any{"field": path}))
}

// errSchemaMalformed rejects a form schema that is not valid JSON.
func errSchemaMalformed(cause error) error {
	return projectionError(i18n.T("approval_form_schema_malformed", map[string]any{"error": cause.Error()}))
}
