package service

import (
	"fmt"
	"regexp"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// ValidateFormDefinition validates the structural integrity of a form schema
// at deploy time: unique non-empty field keys, known field kinds, compilable
// validation patterns, and coherent min/max bounds. Everything checked here
// would otherwise only fail when an applicant submits — and an uncompilable
// pattern would even be misreported to them as a data error — so a broken
// schema must be rejected before the version is created.
func (*FlowDefinitionService) ValidateFormDefinition(def *approval.FormDefinition) error {
	if def == nil {
		return nil
	}

	keys := collections.NewHashSetWithCapacity[string](len(def.Fields))

	for _, field := range def.Fields {
		if field.Key == "" {
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
