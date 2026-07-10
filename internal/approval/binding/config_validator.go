package binding

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/schema"
)

// ConfigValidator validates business binding columns against the primary
// database schema.
type ConfigValidator struct {
	schemas schema.Service
}

// NewConfigValidator creates a ConfigValidator.
func NewConfigValidator(schemas schema.Service) *ConfigValidator {
	return &ConfigValidator{schemas: schemas}
}

// NormalizeConfig validates the mode-dependent shape and SQL identifiers and
// returns a detached, canonical configuration. Key column order is normalized
// because equality predicates are order-independent.
func NormalizeConfig(mode approval.BindingMode, config *approval.BusinessBindingConfig) (*approval.BusinessBindingConfig, error) {
	if mode != approval.BindingBusiness {
		if config != nil {
			return nil, shared.ErrBindingUnexpected
		}

		return nil, nil
	}

	if config == nil {
		return nil, shared.ErrBindingIncomplete
	}

	normalized := &approval.BusinessBindingConfig{
		TableName:        strings.TrimSpace(config.TableName),
		KeyColumns:       make([]string, len(config.KeyColumns)),
		StatusColumn:     strings.TrimSpace(config.StatusColumn),
		InstanceIDColumn: normalizeOptionalColumn(config.InstanceIDColumn),
		StartedAtColumn:  normalizeOptionalColumn(config.StartedAtColumn),
		FinishedAtColumn: normalizeOptionalColumn(config.FinishedAtColumn),
		StatusMapping:    maps.Clone(config.StatusMapping),
	}

	for i, column := range config.KeyColumns {
		normalized.KeyColumns[i] = strings.TrimSpace(column)
	}

	slices.Sort(normalized.KeyColumns)

	if normalized.TableName == "" || len(normalized.KeyColumns) == 0 ||
		normalized.StatusColumn == "" || normalized.InstanceIDColumn == nil {
		return nil, shared.ErrBindingIncomplete
	}

	for status, value := range normalized.StatusMapping {
		trimmed := strings.TrimSpace(value)
		if !isProjectableStatus(status) || trimmed == "" {
			return nil, shared.ErrBindingStatusMappingInvalid
		}

		normalized.StatusMapping[status] = trimmed
	}

	identifiers := make([]string, 0, len(normalized.KeyColumns)+5)
	identifiers = append(identifiers, normalized.TableName)
	identifiers = append(identifiers, normalized.KeyColumns...)
	identifiers = append(identifiers, bindingWriteColumns(normalized)...)

	for _, identifier := range identifiers {
		if identifier == "" {
			return nil, shared.ErrBindingIncomplete
		}

		if err := approval.ValidateBusinessIdentifier(identifier); err != nil {
			if errors.Is(err, approval.ErrInvalidBusinessIdentifier) {
				return nil, shared.ErrInvalidBusinessIdentifier
			}

			return nil, err
		}
	}

	seen := collections.NewHashSetWithCapacity[string](len(identifiers) - 1)
	for _, column := range identifiers[1:] {
		if !seen.Add(column) {
			return nil, shared.ErrBindingColumnsConflict
		}
	}

	return normalized, nil
}

func isProjectableStatus(status approval.InstanceStatus) bool {
	switch status {
	case approval.InstanceRunning,
		approval.InstanceApproved,
		approval.InstanceRejected,
		approval.InstanceWithdrawn,
		approval.InstanceReturned,
		approval.InstanceTerminated:
		return true
	default:
		return false
	}
}

// ValidateSchema verifies that every configured column exists and that the key
// columns exactly match a complete, non-null primary or unique key.
func (v *ConfigValidator) ValidateSchema(ctx context.Context, config *approval.BusinessBindingConfig) error {
	table, err := v.schemas.GetTableSchema(ctx, config.TableName)
	if err != nil {
		if errors.Is(err, schema.ErrTableMissing) {
			return shared.ErrBindingSchemaInvalid
		}

		return fmt.Errorf("inspect business binding table %q: %w", config.TableName, err)
	}

	columns := make(map[string]schema.Column, len(table.Columns))
	for _, column := range table.Columns {
		columns[column.Name] = column
	}

	for _, configured := range append(slices.Clone(config.KeyColumns), bindingWriteColumns(config)...) {
		if _, ok := columns[configured]; !ok {
			return shared.ErrBindingSchemaInvalid
		}
	}

	for _, keyColumn := range config.KeyColumns {
		if columns[keyColumn].Nullable {
			return shared.ErrBindingKeyNotUnique
		}
	}

	if table.PrimaryKey != nil && sameColumns(config.KeyColumns, table.PrimaryKey.Columns) {
		return nil
	}

	for _, unique := range table.UniqueKeys {
		if unique.Predicate == "" && !unique.HasExpressions && sameColumns(config.KeyColumns, unique.Columns) {
			return nil
		}
	}

	return shared.ErrBindingKeyNotUnique
}

func normalizeOptionalColumn(column *string) *string {
	if column == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*column)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func bindingWriteColumns(config *approval.BusinessBindingConfig) []string {
	columns := []string{config.StatusColumn}
	for _, optional := range []*string{config.InstanceIDColumn, config.StartedAtColumn, config.FinishedAtColumn} {
		if optional != nil {
			columns = append(columns, *optional)
		}
	}

	return columns
}

func sameColumns(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	left = slices.Clone(left)
	right = slices.Clone(right)

	slices.Sort(left)
	slices.Sort(right)

	return slices.Equal(left, right)
}
