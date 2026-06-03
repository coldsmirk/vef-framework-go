package shared

// Message IDs for the approval module's i18n keys.
// Only constants referenced cross-file (template params, factory errors,
// or mapping tables) are defined here. Single-use sentinel keys are
// inlined directly at their result.Err definition in api_errors.go.
const (
	// Urge errors. Template parameters:
	//   {{.minutes}} — cooldown window in minutes
	ErrMessageUrgeTooFrequent = "approval_urge_too_frequent"

	// Form field validation errors. Template parameters:
	//   {{.field}} — field label or key
	//   {{.min}}   — minimum length / value
	//   {{.max}}   — maximum length / value
	ErrMessageFormFieldNotDefined        = "approval_form_field_not_defined"
	ErrMessageFormFieldRequired          = "approval_form_field_required"
	ErrMessageFormFieldMustBeString      = "approval_form_field_must_be_string"
	ErrMessageFormFieldMustBeNumber      = "approval_form_field_must_be_number"
	ErrMessageFormFieldMinLength         = "approval_form_field_min_length"
	ErrMessageFormFieldMaxLength         = "approval_form_field_max_length"
	ErrMessageFormFieldInvalidValidation = "approval_form_field_invalid_validation"
	ErrMessageFormFieldPatternMismatch   = "approval_form_field_pattern_mismatch"
	ErrMessageFormFieldMinValue          = "approval_form_field_min_value"
	ErrMessageFormFieldMaxValue          = "approval_form_field_max_value"
	ErrMessageFormFieldEmpty             = "approval_form_field_empty"
	ErrMessageFormFieldInvalidFileItem   = "approval_form_field_invalid_file_item"
	ErrMessageFormFieldMustBeFile        = "approval_form_field_must_be_file"
	ErrMessageFormFieldInvalidValue      = "approval_form_field_invalid_value"
)
