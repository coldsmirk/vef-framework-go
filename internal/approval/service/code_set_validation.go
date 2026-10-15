package service

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/mold"
)

// validateCodeSetValues checks every submitted value of a select field whose
// options come from a host data dictionary (an OptionSourceCodeSet source)
// against that dictionary, detail-table columns included. It runs after the
// structural validation, so the values already have the shapes their fields
// declare. Without a loader the host has registered no dictionaries, and the
// values are accepted as a remote source's are.
//
// The check reads the loader rather than the mold module's cached
// CodeSetResolver, which is scoped to that module; each dictionary is loaded at
// most once per call.
func validateCodeSetValues(ctx context.Context, loader mold.CodeSetLoader, fields []approval.FormFieldDefinition, formData map[string]any) error {
	if loader == nil {
		return nil
	}

	checker := codeSetChecker{loader: loader, loaded: make(map[string]map[string]string)}

	for _, field := range fields {
		value, exists := formData[field.Key]
		if !exists || isEmptyFormValue(value) {
			continue
		}

		if field.Kind == approval.FieldTable {
			if err := checker.checkRows(ctx, field, value); err != nil {
				return err
			}

			continue
		}

		valid, err := checker.check(ctx, field, value)
		if err != nil {
			return err
		}

		if !valid {
			return newFormValidationError(i18n.T(shared.ErrMessageFormFieldInvalidValue, map[string]any{"field": fieldLabel(field)}))
		}
	}

	return nil
}

// codeSetChecker tests values against host data dictionaries, loading each
// dictionary once.
type codeSetChecker struct {
	loader mold.CodeSetLoader
	loaded map[string]map[string]string
}

// checkRows applies check to every code-set column of a detail table's rows,
// naming the row of a value the dictionary does not hold.
func (c *codeSetChecker) checkRows(ctx context.Context, field approval.FormFieldDefinition, value any) error {
	rows, _ := value.([]any)

	for i, raw := range rows {
		row, _ := raw.(map[string]any)

		for _, column := range field.Columns {
			cell, exists := row[column.Key]
			if !exists || isEmptyFormValue(cell) {
				continue
			}

			valid, err := c.check(ctx, column, cell)
			if err != nil {
				return err
			}

			if !valid {
				return newFormValidationError(i18n.T(shared.ErrMessageFormFieldTableCell, map[string]any{
					"field": fieldLabel(field), "row": i + 1,
					"message": i18n.T(shared.ErrMessageFormFieldInvalidValue, map[string]any{"field": fieldLabel(column)}),
				}))
			}
		}
	}

	return nil
}

// check reports whether every item of a select value is a code of the field's
// dictionary. A field without a code set source is always valid.
func (c *codeSetChecker) check(ctx context.Context, field approval.FormFieldDefinition, value any) (bool, error) {
	source := field.OptionSource
	if field.Kind != approval.FieldSelect || source == nil || source.Kind != approval.OptionSourceCodeSet {
		return true, nil
	}

	codes, err := c.codes(ctx, source.CodeSet)
	if err != nil {
		return false, err
	}

	for _, item := range selectItems(value) {
		if _, ok := codes[fmt.Sprint(item)]; !ok {
			return false, nil
		}
	}

	return true, nil
}

func (c *codeSetChecker) codes(ctx context.Context, codeSet string) (map[string]string, error) {
	if codes, ok := c.loaded[codeSet]; ok {
		return codes, nil
	}

	codes, err := c.loader.Load(ctx, codeSet)
	if err != nil {
		return nil, fmt.Errorf("load code set %q: %w", codeSet, err)
	}

	c.loaded[codeSet] = codes

	return codes, nil
}

// selectItems lists the items of a select value: each element of a multiple
// selection, or the single value itself.
func selectItems(value any) []any {
	switch items := value.(type) {
	case []any:
		return items

	case []string:
		result := make([]any, len(items))
		for i, item := range items {
			result[i] = item
		}

		return result

	default:
		return []any{value}
	}
}
