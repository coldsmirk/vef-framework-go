package approval

// FormDefinition represents the form schema definition for a flow version.
type FormDefinition struct {
	Fields []FormFieldDefinition `json:"fields"`
}

// FormFieldDefinition represents a single form field.
type FormFieldDefinition struct {
	// Key is the unique identifier for this field (used in form data keys).
	Key string `json:"key"`
	// Kind is the field type (e.g., "input", "textarea", "select", "number", "date", "upload").
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
