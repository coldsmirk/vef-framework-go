package formeditor

import (
	"fmt"
	"math"
	"strings"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// kindByType maps a designer widget type to the approval field kind whose
// runtime value validation accepts the widget's submitted value (string for
// input / textarea / date / datetime, [start, end] pair for daterange, JSON
// number for number, scalar-or-array for select). switch (boolean) is
// deliberately absent: it surfaces as an unmappable error instead.
var kindByType = map[string]approval.FieldKind{
	"textfield":      approval.FieldInput,
	"code-editor":    approval.FieldTextarea,
	"textarea":       approval.FieldTextarea,
	"number":         approval.FieldNumber,
	"select":         approval.FieldSelect,
	"multi-select":   approval.FieldSelect,
	"radio":          approval.FieldSelect,
	"checkbox-group": approval.FieldSelect,
	"date":           approval.FieldDate,
	"datetime":       approval.FieldDatetime,
	"daterange":      approval.FieldDateRange,
	"upload":         approval.FieldUpload,
}

// multiValueSelectTypes are the selection widgets whose runtime value is a list
// of chosen option values rather than a single scalar. Projected with
// FormFieldDefinition.IsMultiple = true so deploy validation and the flow
// designer offer element-membership operators instead of scalar equality.
var multiValueSelectTypes = collections.NewHashSetFrom("checkbox-group", "multi-select")

// unmappableTypes are widget types that bind a value the approval contract
// cannot carry (as opposed to unknown consumer-registered types).
var unmappableTypes = collections.NewHashSetFrom("switch")

// columnTypeByWidget is the deterministic widget → storage column type. number
// (integer/decimal by precision), date (date vs sized string by granularity),
// upload (text vs json by maxCount), and the string family (textfield / select /
// radio, sized by maxLength) are resolved separately in inferColumnType, so they
// are absent here. The switch entry mirrors the form-editor table verbatim but
// is never reached through projectLeaf — it fails the unmappable check first.
var columnTypeByWidget = map[string]approval.ColumnDataType{
	"switch":         approval.ColumnBoolean,
	"datetime":       approval.ColumnDatetime,
	"checkbox-group": approval.ColumnJSON,
	"multi-select":   approval.ColumnJSON,
	"daterange":      approval.ColumnJSON,
	"textarea":       approval.ColumnText,
	"code-editor":    approval.ColumnText,
}

// classifyWidget reports the projection-blocking fault of a root-scope keyed
// node's widget type: an unmappable (switch) or unknown type. A subform (table)
// and every mapped leaf widget pass. The parser runs this on every sighting
// before the cross-device dedupe, so a type conflict fails identically no
// matter which device sees the key first.
//
// The TS twin (@vef-framework-react/approval-form-bridge, project.ts) must
// mirror this order — classify before dedupe — or the two projectors disagree
// on a key sighted first as a valid widget and again as an unmappable one.
func classifyWidget(node *richBlock) error {
	if node.Type == "subform" {
		return nil
	}

	if _, ok := kindByType[node.Type]; ok {
		return nil
	}

	if unmappableTypes.Contains(node.Type) {
		return errUnmappableFieldType(node.Key, node.Type)
	}

	return errUnknownFieldType(node.Key, node.Type)
}

// projectNode projects one root-scope keyed node: a subform becomes a table
// field, a keyed leaf a scalar field.
func projectNode(node *richBlock, dataSources map[string]richDataSource) (*approval.FormFieldDefinition, error) {
	if node.Type == "subform" {
		return projectSubform(node, dataSources)
	}

	return projectLeaf(node, node.Key, dataSources)
}

// projectLeaf projects one keyed leaf field. It fails (aborting the deploy) when
// the widget's value shape cannot satisfy the runtime validation. path carries
// the table-key prefix when the field is a detail-table column.
func projectLeaf(node *richBlock, path string, dataSources map[string]richDataSource) (*approval.FormFieldDefinition, error) {
	kind, ok := kindByType[node.Type]
	if !ok {
		if unmappableTypes.Contains(node.Type) {
			return nil, errUnmappableFieldType(path, node.Type)
		}

		return nil, errUnknownFieldType(path, node.Type)
	}

	isRequired, rule := splitValidation(node.Validate)
	columnType := resolveColumnType(node)

	field := &approval.FormFieldDefinition{
		Key:   node.Key,
		Kind:  kind,
		Label: resolveLabel(node),
	}

	if node.Placeholder != "" {
		field.Placeholder = node.Placeholder
	}

	if isRequired {
		field.IsRequired = true
	}

	if multiValueSelectTypes.Contains(node.Type) {
		field.IsMultiple = true
	}

	if (node.Type == "date" || node.Type == "daterange") && node.Granularity != "" {
		field.Granularity = approval.DateGranularity(node.Granularity)
	}

	// Non-day date granularities store fixed-length strings (4 for year, 7 for
	// quarter/month) in a VARCHAR column; carry the length bound on Validation so
	// table DDL renders VARCHAR(4)/VARCHAR(7) and parity mirrors column-type.ts.
	if node.Type == "date" && columnType == approval.ColumnString {
		if length := field.Granularity.MaxLength(); length > 0 && (rule == nil || rule.MaxLength == nil) {
			if rule == nil {
				rule = new(approval.ValidationRule)
			}

			rule.MaxLength = &length
		}
	}

	options, optionSource := resolveOptionSource(node, dataSources)
	if options != nil {
		field.Options = options
	}

	if optionSource != nil {
		field.OptionSource = optionSource
	}

	if rule != nil {
		field.Validation = rule
	}

	if columnType != "" {
		field.ColumnType = columnType
	}

	// Scale pairs only with a decimal column that actually configured precision;
	// an explicit non-decimal override or a precision-less decimal carries none.
	if columnType == approval.ColumnDecimal && node.Precision != nil && *node.Precision > 0 {
		scale := *node.Precision
		field.Scale = &scale
	}

	return field, nil
}

// projectSubform projects a root-scope subform into a single-level table field.
// The template is walked with root-scope semantics: layout containers are
// transparent, a nested subform is a contract violation, and every keyed
// template leaf becomes a column via projectLeaf.
//
// In dynamic mode (default when rows is omitted), isRequired mirrors min >= 1
// and table-level validation bounds the row count from min / max. In fixed mode
// (matrix/checklist table), FixedRows carries the itemKey, optional headerLabel,
// and either frozen static Items or a post-dereference Source (codeSet/remote),
// while isRequired mirrors rows.required.
func projectSubform(node *richBlock, dataSources map[string]richDataSource) (*approval.FormFieldDefinition, error) {
	var columns []approval.FormFieldDefinition

	seen := collections.NewHashSet[string]()

	err := walkRootKeyed(node.Template, func(child *richBlock) error {
		// A duplicate column key binds the same value twice — first sighting
		// wins, mirroring the root walk's dedup.
		if !seen.Add(child.Key) {
			return nil
		}

		if child.Type == "subform" {
			return errNestedSubform(node.Key + "." + child.Key)
		}

		column, err := projectLeaf(child, node.Key+"."+child.Key, dataSources)
		if err != nil {
			return err
		}

		column.SortOrder = len(columns)
		columns = append(columns, *column)

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(columns) == 0 {
		return nil, errTableColumnsEmpty(node.Key)
	}

	field := &approval.FormFieldDefinition{
		Key:     node.Key,
		Kind:    approval.FieldTable,
		Label:   resolveLabel(node),
		Columns: columns,
	}

	rows := node.Rows
	if rows == nil || rows.Mode == "" || rows.Mode == "dynamic" {
		applyDynamicSubformRows(field, rows)

		return field, nil
	}

	if rows.Mode != "fixed" {
		return nil, errTableRowsModeInvalid(node.Key, rows.Mode)
	}

	if err := applyFixedSubformRows(field, rows, seen, dataSources); err != nil {
		return nil, err
	}

	return field, nil
}

func applyDynamicSubformRows(field *approval.FormFieldDefinition, rows *richSubformRows) {
	if rows == nil {
		return
	}

	if rows.Min != nil && *rows.Min >= 1 {
		field.IsRequired = true
	}

	rule := new(approval.ValidationRule)
	hasRule := false

	if rows.Min != nil && *rows.Min > 0 {
		rule.MinLength = rows.Min
		hasRule = true
	}

	if rows.Max != nil {
		rule.MaxLength = rows.Max
		hasRule = true
	}

	if hasRule {
		field.Validation = rule
	}
}

func applyFixedSubformRows(
	field *approval.FormFieldDefinition,
	rows *richSubformRows,
	columnKeys collections.Set[string],
	dataSources map[string]richDataSource,
) error {
	if strings.TrimSpace(rows.ItemKey) == "" {
		return errTableFixedRowsItemKeyEmpty(field.Key)
	}

	if columnKeys.Contains(rows.ItemKey) {
		return errTableFixedRowsItemKeyConflict(field.Key, rows.ItemKey)
	}

	items, source := resolveOptionSourceValue(rows.Source, dataSources)
	if source == nil && !isValidStaticFixedRowItems(items) {
		return errTableFixedRowsSourceInvalid(field.Key)
	}

	fixed := &approval.TableFixedRows{
		ItemKey:     rows.ItemKey,
		HeaderLabel: rows.HeaderLabel,
	}
	if source != nil {
		fixed.Source = source
	} else {
		fixed.Items = items
	}

	field.FixedRows = fixed

	if rows.Required != nil && *rows.Required {
		field.IsRequired = true
	}

	return nil
}

func isValidStaticFixedRowItems(items []approval.FieldOption) bool {
	if len(items) == 0 {
		return false
	}

	seen := collections.NewHashSetWithCapacity[string](len(items))
	for _, item := range items {
		val, ok := formatFixedItemValue(item.Value)
		if !ok || !seen.Add(val) {
			return false
		}
	}

	return true
}

func formatFixedItemValue(raw any) (string, bool) {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", false
		}

		return v, true

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v), true

	case float32:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return "", false
		}

		return fmt.Sprint(v), true

	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", false
		}

		return fmt.Sprint(v), true

	default:
		return "", false
	}
}

// resolveOptionSource collapses a selection field's option source into exactly
// one of two projections: the enumerated option list of a static source, or the
// descriptor of a remote or code set one. A ref is dereferenced first, so the
// result is always post-dereference and a consumer never chases a dataSourceId
// (a form-global data source is never a code set: the designer offers code sets
// inline only).
//
// A remote source is emitted even when its request is missing — that is a broken
// designer document, and deploy validation names it rather than the projection
// silently degrading it to a source-less field. A dangling ref, an unknown kind
// and a field with no source all yield neither: the field carries no options, so
// the server accepts any submitted value.
func resolveOptionSource(
	node *richBlock,
	dataSources map[string]richDataSource,
) ([]approval.FieldOption, *approval.FieldOptionSource) {
	return resolveOptionSourceValue(node.DataSource, dataSources)
}

func resolveOptionSourceValue(
	source *richOptionSource,
	dataSources map[string]richDataSource,
) ([]approval.FieldOption, *approval.FieldOptionSource) {
	if source == nil {
		return nil, nil
	}

	kind, options, request, mapping := source.Kind, source.Options, source.Request, source.Mapping

	if kind == "ref" {
		referenced, ok := dataSources[source.DataSourceID]
		if !ok {
			return nil, nil
		}

		kind, options, request, mapping = referenced.Kind, referenced.Options, referenced.Request, referenced.Mapping
	}

	switch kind {
	case "static":
		return options, nil

	case "remote":
		return nil, &approval.FieldOptionSource{
			Kind:    approval.OptionSourceRemote,
			Request: request,
			Mapping: mapping,
		}

	case "codeSet":
		return nil, &approval.FieldOptionSource{
			Kind:    approval.OptionSourceCodeSet,
			CodeSet: source.CodeSet,
		}

	default:
		return nil, nil
	}
}

// resolveColumnType is the column type to emit for a leaf field. An explicit
// designer override always wins; otherwise the widget-derived inference applies,
// EXCEPT the number widget's precision-less integer fallback — the runtime
// rejects fractional values on an integer column, so a plain number field stays
// column-type-less and the backend falls back to a gate-free numeric column.
func resolveColumnType(node *richBlock) approval.ColumnDataType {
	if node.ColumnType != "" {
		return approval.ColumnDataType(node.ColumnType)
	}

	inferred := inferColumnType(node)
	if node.Type == "number" && inferred == approval.ColumnInteger {
		return ""
	}

	return inferred
}

// inferColumnType infers a keyed field's column type from its widget, honoring
// an explicit override first. number is integer or decimal by its precision; the
// deterministic widgets map through columnTypeByWidget; textfield / select /
// radio and any unmapped widget fall back to a sized string or lossless text by
// their maxLength.
func inferColumnType(node *richBlock) approval.ColumnDataType {
	if node.ColumnType != "" {
		return approval.ColumnDataType(node.ColumnType)
	}

	if node.Type == "number" {
		if node.Precision != nil && *node.Precision > 0 {
			return approval.ColumnDecimal
		}

		return approval.ColumnInteger
	}

	if node.Type == "date" {
		if approval.DateGranularity(node.Granularity).Effective() == approval.DateGranularityDay {
			return approval.ColumnDate
		}

		return approval.ColumnString
	}

	// An upload field's value is a storage key: one string when it accepts a
	// single file, an array of them otherwise — the same array shape
	// checkbox-group carries, so a multi-file field needs the same JSON column.
	// It cannot ride columnTypeByWidget because the answer depends on maxCount,
	// and it must not reach the maxLength fallback below: maxLength bounds a
	// string's length, while an upload's bound is its file COUNT.
	if node.Type == "upload" {
		if node.MaxCount != nil && *node.MaxCount > 1 {
			return approval.ColumnJSON
		}

		return approval.ColumnText
	}

	if mapped, ok := columnTypeByWidget[node.Type]; ok {
		return mapped
	}

	if node.Validate != nil && node.Validate.MaxLength != nil && *node.Validate.MaxLength > 0 {
		return approval.ColumnString
	}

	return approval.ColumnText
}

// splitValidation splits the designer's validate mixin into the backend's
// isRequired flag and the residual validation rule (nil when empty).
func splitValidation(v *richValidate) (bool, *approval.ValidationRule) {
	if v == nil {
		return false, nil
	}

	isRequired := v.Required != nil && *v.Required

	if v.MinLength == nil && v.MaxLength == nil && v.Min == nil && v.Max == nil && v.Pattern == "" && v.Message == "" {
		return isRequired, nil
	}

	return isRequired, &approval.ValidationRule{
		MinLength: v.MinLength,
		MaxLength: v.MaxLength,
		Min:       v.Min,
		Max:       v.Max,
		Pattern:   v.Pattern,
		Message:   v.Message,
	}
}

// resolveLabel is the field's label, falling back to its key when the designer
// left it unset. An explicit empty label is preserved, matching `label ?? key`.
func resolveLabel(node *richBlock) string {
	if node.Label != nil {
		return *node.Label
	}

	return node.Key
}

// peekKind is the kind a keyed node would project to, without projecting it —
// used to detect a cross-device kind conflict on an already-seen key. Returns
// the empty kind for an unmappable or unknown widget.
func peekKind(node *richBlock) approval.FieldKind {
	if node.Type == "subform" {
		return approval.FieldTable
	}

	if kind, ok := kindByType[node.Type]; ok {
		return kind
	}

	return ""
}

// peekGranularity returns the effective granularity of a date or daterange
// widget for cross-device comparison (empty resolves to day), or "" for other
// widget types.
func peekGranularity(node *richBlock) approval.DateGranularity {
	if node.Type == "date" || node.Type == "daterange" {
		return approval.DateGranularity(node.Granularity).Effective()
	}

	return ""
}

// describeFieldShape formats a projected kind + multiplicity / granularity for
// cross-device comparison and error messages. Single-valued / default-day kinds
// stay their bare kind name; multi-value selects and non-day date / daterange
// fields are distinguished so value-shape conflicts across devices are caught.
func describeFieldShape(kind approval.FieldKind, isMultiple bool, granularity approval.DateGranularity) string {
	if kind == approval.FieldSelect && isMultiple {
		return string(kind) + " (multiple)"
	}

	if (kind == approval.FieldDate || kind == approval.FieldDateRange) && granularity != "" && granularity != approval.DateGranularityDay {
		return string(kind) + " (" + string(granularity) + ")"
	}

	return string(kind)
}

// peekTableSignature is the contract signature of a subform's row shape: its
// row mode (dynamic vs fixed + itemKey) and the ordered, deduplicated key:kind
// pairs of its template's root-scope keyed leaves (layout containers
// transparent, matching the projection walk). Two devices whose signatures
// differ deploy a row shape at least one of them cannot submit.
func peekTableSignature(node *richBlock) string {
	modePrefix := "dynamic|"
	if node.Rows != nil {
		switch node.Rows.Mode {
		case "", "dynamic":
			modePrefix = "dynamic|"
		case "fixed":
			modePrefix = "fixed:" + node.Rows.ItemKey + "|"
		default:
			modePrefix = "invalid:" + node.Rows.Mode + "|"
		}
	}

	var parts []string

	seen := collections.NewHashSet[string]()

	_ = walkRootKeyed(node.Template, func(child *richBlock) error {
		if !seen.Add(child.Key) {
			return nil
		}

		kind := "?"
		if child.Type == "subform" {
			kind = string(approval.FieldTable)
		} else if mapped, ok := kindByType[child.Type]; ok {
			kind = describeFieldShape(mapped, multiValueSelectTypes.Contains(child.Type), peekGranularity(child))
		}

		parts = append(parts, child.Key+":"+kind)

		return nil
	})

	return modePrefix + strings.Join(parts, ",")
}
