package approval

// FormFieldDefinition represents a single form field.
type FormFieldDefinition struct {
	// Key is the unique identifier for this field (used in form data keys).
	Key string `json:"key"`
	// Kind is the field type (e.g., "input", "textarea", "select", "number", "date", "datetime", "daterange", "upload", "table").
	Kind FieldKind `json:"kind"`
	// Label is the display label.
	Label string `json:"label"`
	// Placeholder is the input placeholder text.
	Placeholder string `json:"placeholder,omitempty"`
	// DefaultValue is the default value for this field.
	DefaultValue any `json:"defaultValue,omitempty"`
	// IsRequired indicates whether this field is required.
	IsRequired bool `json:"isRequired,omitempty"`
	// IsMultiple indicates whether a selection field (Kind == FieldSelect) holds
	// multiple chosen values as a list (e.g., checkbox-group, multi-select).
	// Drives condition operator validation and the flow designer's operator picker.
	IsMultiple bool `json:"isMultiple,omitempty"`
	// Granularity specifies the calendar granularity of a date or daterange field
	// ("year", "quarter", "month", or "day"; empty defaults to "day"). Valid only
	// on FieldDate and FieldDateRange; deploy validation rejects it on any other
	// kind or with an unrecognized value.
	Granularity DateGranularity `json:"granularity,omitempty"`
	// Options is the list of selectable options (for select, radio, checkbox, etc.).
	Options []FieldOption `json:"options,omitempty"`
	// OptionSource describes where a selection field's options come from when
	// they could not be enumerated into Options: a remote source the host
	// resolves, or a host data dictionary. The framework never resolves a remote
	// one — select validation checks a submitted value against Options and
	// accepts anything when there are none — and carries it for consumers that
	// must render a stored value as its label rather than as the raw value (see
	// RemoteOptionRequest.HasBoundParams for when that is a single lookup and
	// when it is per-row). A data dictionary is checked: with a CodeSetLoader
	// registered, a submitted value must be one of the dictionary's codes.
	OptionSource *FieldOptionSource `json:"optionSource,omitempty"`
	// Validation contains validation rules.
	Validation *ValidationRule `json:"validation,omitempty"`
	// Props contains additional component-specific properties.
	Props map[string]any `json:"props,omitempty"`
	// SortOrder controls the display order.
	SortOrder int `json:"sortOrder"`
	// ColumnType is the dialect-independent logical column type used when the
	// flow version's StorageMode is StorageTable. Empty falls back to a coarse
	// type derived from Kind (back-compat with schemas authored before this field).
	ColumnType ColumnDataType `json:"columnType,omitempty"`
	// Scale is the number of fractional digits for a ColumnDecimal column (the
	// DECIMAL/NUMERIC scale). Nil means an integer-shaped decimal (scale 0).
	Scale *int `json:"scale,omitempty"`
	// Columns defines the row shape of a table field (Kind == FieldTable):
	// each column is itself a field definition and reuses the same kinds and
	// validation rules, except that a column must not be another table —
	// detail tables are single-level by design. For a dynamic table field
	// (FixedRows == nil), Validation.MinLength / MaxLength bound the ROW COUNT,
	// and IsRequired means "at least one row". For a fixed-row table
	// (FixedRows != nil), the read-only header cell under FixedRows.ItemKey is
	// kept out of Columns.
	Columns []FormFieldDefinition `json:"columns,omitempty"`
	// FixedRows describes fixed-row (matrix/checklist) semantics for a table
	// field (Kind == FieldTable). Nil means dynamic rows (the applicant adds
	// and removes rows freely). Valid only on FieldTable; deploy validation
	// rejects it on any other kind.
	FixedRows *TableFixedRows `json:"fixedRows,omitempty"`
}

// TableFixedRows describes the fixed-row contract of a FieldTable field.
// Exactly one of Items or Source is populated:
//   - Static (inline or dereferenced ref-to-static) enumerates Items in order;
//     submission validation requires submitted rows to match Items exactly
//     (count, order, value and label).
//   - CodeSet sets Source{Kind: OptionSourceCodeSet, CodeSet}; submission
//     validation checks each row's item value against the host's CodeSetLoader
//     (when registered) and enforces unique values.
//   - Remote sets Source{Kind: OptionSourceRemote, Request, Mapping};
//     submission validation checks row item shape and unique values only.
type TableFixedRows struct {
	// ItemKey is the row key holding the snapshotted {value, label} item cell.
	// It is never listed in FormFieldDefinition.Columns and is read-only for the
	// applicant.
	ItemKey string `json:"itemKey"`
	// HeaderLabel is the display title of the row-header column.
	HeaderLabel string `json:"headerLabel,omitempty"`
	// Items holds the frozen static row items in display order.
	Items []FieldOption `json:"items,omitempty"`
	// Source describes a non-static item source (codeSet or remote), post-dereference.
	Source *FieldOptionSource `json:"source,omitempty"`
}

// FieldOption represents a selectable option for select/radio/checkbox fields.
type FieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// FieldOptionSource describes an option source the projection could not
// enumerate. It is the post-dereference view: a ref has already been resolved
// against the form-global sources, so a consumer never has to chase one.
type FieldOptionSource struct {
	// Kind classifies the source.
	Kind OptionSourceKind `json:"kind"`
	// Request addresses the operation returning an OptionSourceRemote source's
	// option records. Deploy validation rejects a remote descriptor without
	// one, so a consumer reading a deployed version's fields can rely on it
	// being set.
	Request *RemoteOptionRequest `json:"request,omitempty"`
	// Mapping says how to read a label and a value out of each returned record.
	// Nil means the defaults.
	Mapping *RemoteOptionMapping `json:"mapping,omitempty"`
	// CodeSet names the host data dictionary of an OptionSourceCodeSet source —
	// the code set a mold CodeSetLoader loads and a CodeSetResolver translates.
	// Deploy validation rejects a code set descriptor without one.
	CodeSet string `json:"codeSet,omitempty"`
}

// RemoteOptionRequest addresses the operation that returns a remote source's
// option records, in the framework's own resource/action addressing.
//
// A parameter is either literal or bound to the live form, so the request is
// not necessarily the same for every row: a bound one is what makes a cascading
// select work, and it resolves against whatever the form held at the time. Ask
// HasBoundParams before assuming one call can translate a whole column.
type RemoteOptionRequest struct {
	// Resource is the API resource name.
	Resource string `json:"resource"`
	// Action is the operation name on that resource.
	Action string `json:"action"`
	// Version is the resource version (e.g. "v1"). Empty means the default.
	Version string `json:"version,omitempty"`
	// Params are the request parameters the designer configured, each a fixed
	// value, or a scope path or expression the form runtime resolves before
	// issuing the request. They are carried unevaluated: the framework holds no
	// form values to evaluate them against, and a consumer replaying the request
	// owns that step.
	Params map[string]DynamicParam `json:"params,omitempty"`
}

// HasBoundParams reports whether any parameter is bound to the live form, by
// reference or by expression. A request with none resolves identically for
// every row, so one call builds a value-to-label map for a whole column; a
// bound one resolves per row, and a consumer translating a list must either
// evaluate it per row or leave that field untranslated.
func (r *RemoteOptionRequest) HasBoundParams() bool {
	if r == nil {
		return false
	}

	for _, param := range r.Params {
		switch param.Kind {
		case DynamicParamReference, DynamicParamExpression:
			return true
		}
	}

	return false
}

// DynamicParam is one remote-request parameter. Exactly one of the payload
// fields is meaningful, per Kind: Value for a literal, Path for a reference,
// Source for an expression.
type DynamicParam struct {
	// Kind says which payload field carries the parameter.
	Kind DynamicParamKind `json:"kind"`
	// Value is a literal parameter's value. Deliberately not omitempty — a
	// literal false, 0 or "" is a value the designer chose, and dropping it on
	// the way to storage would silently change the request.
	Value any `json:"value"`
	// Path is a reference parameter's scope path, read by the form runtime.
	Path string `json:"path,omitempty"`
	// Source is an expression parameter's source text, evaluated by the form
	// runtime against the current form values.
	Source string `json:"source,omitempty"`
}

// RemoteOptionMapping says which keys of a returned record carry an option's
// label and value. An empty field falls back to its default: "label" for the
// label, "value" for the value.
type RemoteOptionMapping struct {
	// LabelKey is the record key holding the display label.
	LabelKey string `json:"labelKey,omitempty"`
	// ValueKey is the record key holding the stored value.
	ValueKey string `json:"valueKey,omitempty"`
	// DisabledKey is the record key marking an option as unselectable.
	DisabledKey string `json:"disabledKey,omitempty"`
	// DescriptionKey is the record key holding a secondary description.
	DescriptionKey string `json:"descriptionKey,omitempty"`
}

// ValidationRule contains validation constraints for a form field.
type ValidationRule struct {
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Message   string   `json:"message,omitempty"`
}
