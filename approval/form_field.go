package approval

// FormFieldDefinition represents a single form field.
type FormFieldDefinition struct {
	// Key is the unique identifier for this field (used in form data keys).
	Key string `json:"key"`
	// Kind is the field type (e.g., "input", "textarea", "select", "number", "date", "upload", "table").
	Kind FieldKind `json:"kind"`
	// Label is the display label.
	Label string `json:"label"`
	// Placeholder is the input placeholder text.
	Placeholder string `json:"placeholder,omitempty"`
	// DefaultValue is the default value for this field.
	DefaultValue any `json:"defaultValue,omitempty"`
	// IsRequired indicates whether this field is required.
	IsRequired bool `json:"isRequired,omitempty"`
	// Options is the list of selectable options (for select, radio, checkbox, etc.).
	Options []FieldOption `json:"options,omitempty"`
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
	// detail tables are single-level by design. For a table field itself,
	// Validation.MinLength / MaxLength bound the ROW COUNT, and IsRequired
	// means "at least one row".
	Columns []FormFieldDefinition `json:"columns,omitempty"`
}

// FieldOption represents a selectable option for select/radio/checkbox fields.
type FieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
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
