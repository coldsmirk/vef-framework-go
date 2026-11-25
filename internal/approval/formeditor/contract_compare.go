package formeditor

import (
	"maps"
	"reflect"
	"strconv"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// compareFieldContracts compares two projected FormFieldDefinitions (PC vs
// mobile) for data-contract equality, ignoring purely presentational attributes
// (Label, Placeholder, SortOrder, Validation.Message, FixedRows.HeaderLabel,
// and column order). Mirrors compareFieldContracts in
// @vef-framework-react/approval-form-bridge (contract-compare.ts).
func compareFieldContracts(path string, pc, mobile *approval.FormFieldDefinition) error {
	pcGranularity := effectiveFieldGranularity(pc)
	mobileGranularity := effectiveFieldGranularity(mobile)

	if pc.Kind != mobile.Kind || pc.IsMultiple != mobile.IsMultiple || pcGranularity != mobileGranularity {
		return errCrossDeviceKindMismatch(
			path,
			describeFieldShape(pc.Kind, pc.IsMultiple, pcGranularity),
			describeFieldShape(mobile.Kind, mobile.IsMultiple, mobileGranularity),
		)
	}

	if pc.Kind == approval.FieldTable {
		return compareTableContracts(path, pc, mobile)
	}

	if !optionsContractEqual(pc, mobile) {
		return errCrossDeviceOptionsMismatch(path)
	}

	if !validationContractEqual(pc, mobile) {
		return errCrossDeviceValidationMismatch(path)
	}

	if !storageContractEqual(pc, mobile) {
		return errCrossDeviceStorageMismatch(
			path,
			describeStorageShape(pc.ColumnType, pc.Scale),
			describeStorageShape(mobile.ColumnType, mobile.Scale),
		)
	}

	return nil
}

func compareTableContracts(path string, pc, mobile *approval.FormFieldDefinition) error {
	if !fixedRowsContractEqual(pc.FixedRows, mobile.FixedRows) {
		return errCrossDeviceTableRowsMismatch(path)
	}

	if !validationContractEqual(pc, mobile) {
		return errCrossDeviceValidationMismatch(path)
	}

	if len(pc.Columns) != len(mobile.Columns) {
		return errCrossDeviceTableMismatch(path)
	}

	mobileByKey := make(map[string]*approval.FormFieldDefinition, len(mobile.Columns))
	for i := range mobile.Columns {
		mobileByKey[mobile.Columns[i].Key] = &mobile.Columns[i]
	}

	for i := range pc.Columns {
		if _, ok := mobileByKey[pc.Columns[i].Key]; !ok {
			return errCrossDeviceTableMismatch(path)
		}
	}

	for i := range pc.Columns {
		pcCol := &pc.Columns[i]
		if err := compareFieldContracts(path+"."+pcCol.Key, pcCol, mobileByKey[pcCol.Key]); err != nil {
			return err
		}
	}

	return nil
}

func effectiveFieldGranularity(field *approval.FormFieldDefinition) approval.DateGranularity {
	if field.Kind == approval.FieldDate || field.Kind == approval.FieldDateRange {
		return field.Granularity.Effective()
	}

	return ""
}

// describeFieldShape formats a projected kind + multiplicity / granularity for
// cross-device diagnostics. Single-valued / default-day kinds stay their bare
// kind name; multi-value selects and non-day date / daterange fields are
// distinguished so value-shape conflicts across devices read clearly.
func describeFieldShape(kind approval.FieldKind, isMultiple bool, granularity approval.DateGranularity) string {
	if kind == approval.FieldSelect && isMultiple {
		return string(kind) + " (multiple)"
	}

	if (kind == approval.FieldDate || kind == approval.FieldDateRange) && granularity != "" && granularity != approval.DateGranularityDay {
		return string(kind) + " (" + string(granularity) + ")"
	}

	return string(kind)
}

func describeStorageShape(columnType approval.ColumnDataType, scale *int) string {
	if columnType == "" {
		return "default"
	}

	if columnType == approval.ColumnDecimal && scale != nil && *scale > 0 {
		return string(columnType) + "(" + strconv.Itoa(*scale) + ")"
	}

	return string(columnType)
}

func fixedRowsContractEqual(a, b *approval.TableFixedRows) bool {
	if a == nil || b == nil {
		return a == b
	}

	// HeaderLabel is purely presentational and deliberately excluded.
	return a.ItemKey == b.ItemKey &&
		optionListEqual(a.Items, b.Items) &&
		optionSourceEqual(a.Source, b.Source)
}

func optionsContractEqual(a, b *approval.FormFieldDefinition) bool {
	return optionListEqual(a.Options, b.Options) && optionSourceEqual(a.OptionSource, b.OptionSource)
}

func optionListEqual(a, b []approval.FieldOption) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Label != b[i].Label || !equalJSONValue(a[i].Value, b[i].Value) {
			return false
		}
	}

	return true
}

func optionSourceEqual(a, b *approval.FieldOptionSource) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.Kind != b.Kind {
		return false
	}

	switch a.Kind {
	case approval.OptionSourceCodeSet:
		return a.CodeSet == b.CodeSet

	case approval.OptionSourceRemote:
		return remoteRequestEqual(a.Request, b.Request) && remoteMappingEqual(a.Mapping, b.Mapping)

	default:
		return a.CodeSet == b.CodeSet &&
			remoteRequestEqual(a.Request, b.Request) &&
			remoteMappingEqual(a.Mapping, b.Mapping)
	}
}

func remoteRequestEqual(a, b *approval.RemoteOptionRequest) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.Resource != b.Resource || a.Action != b.Action || a.Version != b.Version {
		return false
	}

	if len(a.Params) != len(b.Params) {
		return false
	}

	return maps.EqualFunc(a.Params, b.Params, dynamicParamEqual)
}

func dynamicParamEqual(a, b approval.DynamicParam) bool {
	if a.Kind != b.Kind {
		return false
	}

	switch a.Kind {
	case approval.DynamicParamLiteral:
		return equalJSONValue(a.Value, b.Value)

	case approval.DynamicParamReference:
		return a.Path == b.Path

	case approval.DynamicParamExpression:
		return a.Source == b.Source

	default:
		return a.Path == b.Path && a.Source == b.Source && equalJSONValue(a.Value, b.Value)
	}
}

func remoteMappingEqual(a, b *approval.RemoteOptionMapping) bool {
	var left, right approval.RemoteOptionMapping
	if a != nil {
		left = *a
	}

	if b != nil {
		right = *b
	}

	return left == right
}

func validationContractEqual(a, b *approval.FormFieldDefinition) bool {
	if a.IsRequired != b.IsRequired {
		return false
	}

	var left, right approval.ValidationRule
	if a.Validation != nil {
		left = *a.Validation
	}

	if b.Validation != nil {
		right = *b.Validation
	}

	// Message is presentation copy and deliberately excluded.
	return equalIntPtr(left.MinLength, right.MinLength) &&
		equalIntPtr(left.MaxLength, right.MaxLength) &&
		equalFloatPtr(left.Min, right.Min) &&
		equalFloatPtr(left.Max, right.Max) &&
		left.Pattern == right.Pattern
}

func storageContractEqual(a, b *approval.FormFieldDefinition) bool {
	return a.ColumnType == b.ColumnType && equalIntPtr(a.Scale, b.Scale)
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func equalFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// equalJSONValue compares two JSON-decoded option or literal parameter values
// with JS-compatible type distinction (number != string, numeric equality
// across numeric Go types, recursive comparison for slices and maps).
func equalJSONValue(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}

	if numA, okA := shared.ToFloat64(a); okA {
		numB, okB := shared.ToFloat64(b)

		return okB && numA == numB
	}

	if _, okB := shared.ToFloat64(b); okB {
		return false
	}

	switch left := a.(type) {
	case string:
		right, ok := b.(string)

		return ok && left == right

	case bool:
		right, ok := b.(bool)

		return ok && left == right

	case []any:
		right, ok := b.([]any)
		if !ok || len(left) != len(right) {
			return false
		}

		for i := range left {
			if !equalJSONValue(left[i], right[i]) {
				return false
			}
		}

		return true

	case map[string]any:
		right, ok := b.(map[string]any)
		if !ok || len(left) != len(right) {
			return false
		}

		return maps.EqualFunc(left, right, equalJSONValue)

	default:
		return reflect.DeepEqual(a, b)
	}
}
