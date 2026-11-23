package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// ValidateFormFields validates the structural integrity of the parsed form
// fields at deploy time: unique non-empty field keys, known field kinds,
// compilable validation patterns, and coherent min/max bounds. Everything
// checked here would otherwise only fail when an applicant submits — and an
// uncompilable pattern would even be misreported to them as a data error — so
// a broken field list must be rejected before the version is created. A nil
// list (a flow without a form) is valid.
func (*FlowDefinitionService) ValidateFormFields(fields []approval.FormFieldDefinition) error {
	keys := collections.NewHashSetWithCapacity[string](len(fields))

	for _, field := range fields {
		// TrimSpace, not == "": a whitespace-only key is as unusable as an empty
		// one — it sanitizes to an empty storage identifier, which only the
		// table-mode identifier checks would catch (as an opaque generated-
		// identifier fault) — so reject it here as a clear form-design error for
		// every storage mode.
		if strings.TrimSpace(field.Key) == "" {
			return errFormFieldKeyEmpty
		}

		if !keys.Add(field.Key) {
			return fmt.Errorf("%w: %q", errDuplicateFormFieldKey, field.Key)
		}

		if !field.Kind.IsValid() {
			return fmt.Errorf("%w: %q for field %q", errInvalidFormFieldKind, field.Kind, field.Key)
		}

		if err := validateDateGranularity(field, field.Key); err != nil {
			return err
		}

		if err := validateFieldValidationRule(field); err != nil {
			return err
		}

		if err := validateOptionSource(field, field.Key); err != nil {
			return err
		}

		if err := validateTableColumns(field); err != nil {
			return err
		}
	}

	return nil
}

// validateTableColumns checks the row shape of a table field: at least one
// column, column keys unique within the table, known column kinds, no nested
// tables — detail tables are single-level by design (deep structures belong to
// business tables reached via the business binding) — and valid fixed-row
// configuration when declared. Column validation rules reuse the scalar-field
// checks. Scalar fields must not declare columns or fixed rows; silently
// ignoring them would hide a designer bug.
func validateTableColumns(field approval.FormFieldDefinition) error {
	if field.Kind != approval.FieldTable {
		if len(field.Columns) > 0 {
			return fmt.Errorf("%w: field %q", errColumnsOnScalarField, field.Key)
		}

		if field.FixedRows != nil {
			return fmt.Errorf("%w: field %q", errFixedRowsOnScalarField, field.Key)
		}

		return nil
	}

	if len(field.Columns) == 0 {
		return fmt.Errorf("%w: field %q", errTableColumnsRequired, field.Key)
	}

	keys := collections.NewHashSetWithCapacity[string](len(field.Columns))

	for _, column := range field.Columns {
		if strings.TrimSpace(column.Key) == "" {
			return fmt.Errorf("%w: in table %q", errFormFieldKeyEmpty, field.Key)
		}

		if !keys.Add(column.Key) {
			return fmt.Errorf("%w: column %q in table %q", errDuplicateFormFieldKey, column.Key, field.Key)
		}

		if column.Kind == approval.FieldTable {
			return fmt.Errorf("%w: column %q in table %q", errNestedTableColumn, column.Key, field.Key)
		}

		if !column.Kind.IsValid() {
			return fmt.Errorf("%w: %q for column %q in table %q", errInvalidFormFieldKind, column.Kind, column.Key, field.Key)
		}

		if err := validateDateGranularity(column, field.Key+"."+column.Key); err != nil {
			return err
		}

		if err := validateFieldValidationRule(column); err != nil {
			return err
		}

		if err := validateOptionSource(column, field.Key+"."+column.Key); err != nil {
			return err
		}
	}

	return validateTableFixedRows(field, keys)
}

func validateTableFixedRows(field approval.FormFieldDefinition, columnKeys collections.Set[string]) error {
	fixed := field.FixedRows
	if fixed == nil {
		return nil
	}

	if strings.TrimSpace(fixed.ItemKey) == "" {
		return fmt.Errorf("%w: table %q", errFixedRowsItemKeyEmpty, field.Key)
	}

	if columnKeys.Contains(fixed.ItemKey) {
		return fmt.Errorf("%w: itemKey %q in table %q", errFixedRowsItemKeyConflict, fixed.ItemKey, field.Key)
	}

	hasItems := len(fixed.Items) > 0
	hasSource := fixed.Source != nil

	if hasItems && hasSource {
		return fmt.Errorf("%w: table %q", errFixedRowsSourceAmbiguous, field.Key)
	}

	if !hasItems && !hasSource {
		return fmt.Errorf("%w: table %q", errFixedRowsSourceRequired, field.Key)
	}

	if hasSource {
		return validateOptionSourceSpec(fixed.Source, field.Key)
	}

	seenValues := collections.NewHashSetWithCapacity[string](len(fixed.Items))
	for _, item := range fixed.Items {
		val, ok := shared.FormatFixedRowItemValue(item.Value)
		if !ok {
			return fmt.Errorf("%w: table %q", errFixedRowsInvalidItem, field.Key)
		}

		if !seenValues.Add(val) {
			return fmt.Errorf("%w: %q in table %q", errFixedRowsDuplicateItem, val, field.Key)
		}
	}

	return nil
}

// validateOptionSource checks a field's unresolved option source. A remote
// source naming no operation would fail nowhere at deploy — select validation
// reads Options and accepts anything when there are none — and then silently
// render as a raw value in every consumer that replays it; a code set naming no
// dictionary would leave its field with no options at all.
// path is the field key, table-qualified for a detail-table column.
func validateOptionSource(field approval.FormFieldDefinition, path string) error {
	return validateOptionSourceSpec(field.OptionSource, path)
}

func validateOptionSourceSpec(source *approval.FieldOptionSource, path string) error {
	if source == nil {
		return nil
	}

	switch source.Kind {
	case approval.OptionSourceRemote:
		if source.Request == nil ||
			strings.TrimSpace(source.Request.Resource) == "" ||
			strings.TrimSpace(source.Request.Action) == "" {
			return fmt.Errorf("%w: field %q", errRemoteOptionRequestIncomplete, path)
		}

	case approval.OptionSourceCodeSet:
		if strings.TrimSpace(source.CodeSet) == "" {
			return fmt.Errorf("%w: field %q", errCodeSetOptionSourceIncomplete, path)
		}

	default:
		return fmt.Errorf("%w: %q for field %q", errInvalidOptionSourceKind, source.Kind, path)
	}

	return nil
}

// validateDateGranularity checks that Granularity is only declared on date and
// daterange fields and uses a known value. Empty means day and is always valid.
func validateDateGranularity(field approval.FormFieldDefinition, path string) error {
	if field.Granularity == "" {
		return nil
	}

	if field.Kind != approval.FieldDate && field.Kind != approval.FieldDateRange {
		return fmt.Errorf("%w: %q on %s field %q", errGranularityNotAllowed, field.Granularity, field.Kind, path)
	}

	if !field.Granularity.IsValid() {
		return fmt.Errorf("%w: %q for field %q", errInvalidDateGranularity, field.Granularity, path)
	}

	return nil
}

// validateFieldValidationRule checks a field's validation block for faults
// that would make the rule unenforceable (bad regex) or unsatisfiable
// (inverted bounds).
func validateFieldValidationRule(field approval.FormFieldDefinition) error {
	rule := field.Validation
	if rule == nil {
		return nil
	}

	if rule.Pattern != "" {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("%w: field %q: %w", errInvalidFormPattern, field.Key, err)
		}
	}

	if rule.MinLength != nil && rule.MaxLength != nil && *rule.MinLength > *rule.MaxLength {
		return fmt.Errorf("%w: field %q", errInvalidFormLengthRange, field.Key)
	}

	if rule.Min != nil && rule.Max != nil && *rule.Min > *rule.Max {
		return fmt.Errorf("%w: field %q", errInvalidFormValueRange, field.Key)
	}

	return nil
}
