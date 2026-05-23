package shared

// Message IDs for the approval module's i18n keys.
// Use with i18n.T() to resolve to the active language.
const (
	// Flow definition errors.
	ErrMessageFlowNotFound              = "approval_flow_not_found"
	ErrMessageFlowNotActive             = "approval_flow_not_active"
	ErrMessageNoPublishedVersion        = "approval_no_published_version"
	ErrMessageVersionNotDraft           = "approval_version_not_draft"
	ErrMessageInvalidFlowDesign         = "approval_invalid_flow_design"
	ErrMessageFlowCodeExists            = "approval_flow_code_exists"
	ErrMessageVersionNotFound           = "approval_version_not_found"
	ErrMessageInvalidBusinessIdentifier = "approval_invalid_business_identifier"

	// Instance errors.
	ErrMessageInstanceNotFound          = "approval_instance_not_found"
	ErrMessageInstanceCompleted         = "approval_instance_completed"
	ErrMessageNotAllowedInitiate        = "approval_not_allowed_initiate"
	ErrMessageWithdrawNotAllowed        = "approval_withdraw_not_allowed"
	ErrMessageResubmitNotAllowed        = "approval_resubmit_not_allowed"
	ErrMessageInvalidInstanceTransition = "approval_invalid_instance_transition"

	// Task errors.
	ErrMessageTaskNotFound             = "approval_task_not_found"
	ErrMessageTaskNotPending           = "approval_task_not_pending"
	ErrMessageNotAssignee              = "approval_not_assignee"
	ErrMessageInvalidTaskTransition    = "approval_invalid_task_transition"
	ErrMessageRollbackNotAllowed       = "approval_rollback_not_allowed"
	ErrMessageAddAssigneeNotAllowed    = "approval_add_assignee_not_allowed"
	ErrMessageTransferNotAllowed       = "approval_transfer_not_allowed"
	ErrMessageOpinionRequired          = "approval_opinion_required"
	ErrMessageManualCcNotAllowed       = "approval_manual_cc_not_allowed"
	ErrMessageRemoveAssigneeNotAllowed = "approval_remove_assignee_not_allowed"
	ErrMessageInvalidAddAssigneeType   = "approval_invalid_add_assignee_type"
	ErrMessageNotApplicant             = "approval_not_applicant"
	ErrMessageInvalidRollbackTarget    = "approval_invalid_rollback_target"
	ErrMessageLastAssigneeRemoval      = "approval_last_assignee_removal"
	ErrMessageInvalidTransferTarget    = "approval_invalid_transfer_target"

	// Assignee resolution errors.
	ErrMessageNoAssignee            = "approval_no_assignee"
	ErrMessageAssigneeResolveFailed = "approval_assignee_resolve_failed"

	// Form aggregate errors.
	ErrMessageFormValidationFailed = "approval_form_validation_failed"
	ErrMessageFieldNotEditable     = "approval_field_not_editable"
	ErrMessageFormDataTooLarge     = "approval_form_data_too_large"

	// Delegation errors.
	ErrMessageDelegationNotFound = "approval_delegation_not_found"
	ErrMessageDelegationConflict = "approval_delegation_conflict"

	// Urge errors.
	ErrMessageUrgeTooFrequent = "approval_urge_too_frequent"

	// Access errors.
	ErrMessageAccessDenied       = "approval_access_denied"
	ErrMessageInstanceNotRunning = "approval_instance_not_running"

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
