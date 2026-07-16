package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
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
		// TrimSpace, not == "": a whitespace-only key survives to publish and the
		// storage DDL sanitizes it to an empty identifier, exploding as a bare SQL
		// syntax error instead of a form-design rejection here.
		if strings.TrimSpace(field.Key) == "" {
			return errFormFieldKeyEmpty
		}

		if !keys.Add(field.Key) {
			return fmt.Errorf("%w: %q", errDuplicateFormFieldKey, field.Key)
		}

		if !field.Kind.IsValid() {
			return fmt.Errorf("%w: %q for field %q", errInvalidFormFieldKind, field.Kind, field.Key)
		}

		if err := validateFieldValidationRule(field); err != nil {
			return err
		}

		if err := validateTableColumns(field); err != nil {
			return err
		}
	}

	return nil
}

// validateTableColumns checks the row shape of a table field: at least one
// column, column keys unique within the table, known column kinds, and no
// nested tables — detail tables are single-level by design (deep structures
// belong to business tables reached via the business binding). Column
// validation rules reuse the scalar-field checks. Scalar fields must not
// declare columns at all; silently ignoring them would hide a designer bug.
func validateTableColumns(field approval.FormFieldDefinition) error {
	if field.Kind != approval.FieldTable {
		if len(field.Columns) > 0 {
			return fmt.Errorf("%w: field %q", errColumnsOnScalarField, field.Key)
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

		if err := validateFieldValidationRule(column); err != nil {
			return err
		}
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
