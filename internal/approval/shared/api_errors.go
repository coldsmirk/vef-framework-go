package shared

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Error definitions. Messages are resolved through i18n at package
// init time using the language selected by VEF_I18N_LANGUAGE.
//
// These are sentinel values — callers use errors.Is to recognize them,
// so the Error value must remain stable. Switching i18n language at
// runtime (e.g. via i18n.SetLanguage in tests) will not update these
// frozen messages; new translations only take effect on process restart.
var (
	ErrFlowNotFound       = result.Err(i18n.T("approval_flow_not_found"), result.WithCode(ErrCodeFlowNotFound))
	ErrFlowNotActive      = result.Err(i18n.T("approval_flow_not_active"), result.WithCode(ErrCodeFlowNotActive))
	ErrNoPublishedVersion = result.Err(i18n.T("approval_no_published_version"), result.WithCode(ErrCodeNoPublishedVersion))
	ErrVersionNotDraft    = result.Err(i18n.T("approval_version_not_draft"), result.WithCode(ErrCodeVersionNotDraft))
	ErrInvalidFlowDesign  = result.Err(i18n.T("approval_invalid_flow_design"), result.WithCode(ErrCodeInvalidFlowDesign))
	ErrFlowCodeExists     = result.Err(i18n.T("approval_flow_code_exists"), result.WithCode(ErrCodeFlowCodeExists))
	ErrVersionNotFound    = result.Err(i18n.T("approval_version_not_found"), result.WithCode(ErrCodeVersionNotFound))
	// ErrInvalidBusinessIdentifier rejects business_table / pk / status /
	// title field values that do not match a strict SQL-identifier regex.
	// Flow definitions interpolate these into UPDATE statements at runtime
	// so accepting arbitrary user input would open a SQL injection vector.
	ErrInvalidBusinessIdentifier = result.Err(
		i18n.T("approval_invalid_business_identifier"),
		result.WithCode(ErrCodeInvalidBusinessIdentifier),
	)

	ErrInstanceNotFound          = result.Err(i18n.T("approval_instance_not_found"), result.WithCode(ErrCodeInstanceNotFound))
	ErrInstanceCompleted         = result.Err(i18n.T("approval_instance_completed"), result.WithCode(ErrCodeInstanceCompleted))
	ErrNotAllowedInitiate        = result.Err(i18n.T("approval_not_allowed_initiate"), result.WithCode(ErrCodeNotAllowedInitiate))
	ErrWithdrawNotAllowed        = result.Err(i18n.T("approval_withdraw_not_allowed"), result.WithCode(ErrCodeWithdrawNotAllowed))
	ErrResubmitNotAllowed        = result.Err(i18n.T("approval_resubmit_not_allowed"), result.WithCode(ErrCodeResubmitNotAllowed))
	ErrInvalidInstanceTransition = result.Err(i18n.T("approval_invalid_instance_transition"), result.WithCode(ErrCodeInvalidInstanceTransition))

	ErrTaskNotFound             = result.Err(i18n.T("approval_task_not_found"), result.WithCode(ErrCodeTaskNotFound))
	ErrTaskNotPending           = result.Err(i18n.T("approval_task_not_pending"), result.WithCode(ErrCodeTaskNotPending))
	ErrNotAssignee              = result.Err(i18n.T("approval_not_assignee"), result.WithCode(ErrCodeNotAssignee))
	ErrInvalidTaskTransition    = result.Err(i18n.T("approval_invalid_task_transition"), result.WithCode(ErrCodeInvalidTaskTransition))
	ErrRollbackNotAllowed       = result.Err(i18n.T("approval_rollback_not_allowed"), result.WithCode(ErrCodeRollbackNotAllowed))
	ErrAddAssigneeNotAllowed    = result.Err(i18n.T("approval_add_assignee_not_allowed"), result.WithCode(ErrCodeAddAssigneeNotAllowed))
	ErrTransferNotAllowed       = result.Err(i18n.T("approval_transfer_not_allowed"), result.WithCode(ErrCodeTransferNotAllowed))
	ErrOpinionRequired          = result.Err(i18n.T("approval_opinion_required"), result.WithCode(ErrCodeOpinionRequired))
	ErrManualCcNotAllowed       = result.Err(i18n.T("approval_manual_cc_not_allowed"), result.WithCode(ErrCodeManualCcNotAllowed))
	ErrRemoveAssigneeNotAllowed = result.Err(i18n.T("approval_remove_assignee_not_allowed"), result.WithCode(ErrCodeRemoveAssigneeNotAllowed))
	ErrInvalidAddAssigneeType   = result.Err(i18n.T("approval_invalid_add_assignee_type"), result.WithCode(ErrCodeInvalidAddAssigneeType))
	ErrNotApplicant             = result.Err(i18n.T("approval_not_applicant"), result.WithCode(ErrCodeNotApplicant))
	ErrInvalidRollbackTarget    = result.Err(i18n.T("approval_invalid_rollback_target"), result.WithCode(ErrCodeInvalidRollbackTarget))
	ErrLastAssigneeRemoval      = result.Err(i18n.T("approval_last_assignee_removal"), result.WithCode(ErrCodeLastAssigneeRemoval))
	ErrInvalidTransferTarget    = result.Err(i18n.T("approval_invalid_transfer_target"), result.WithCode(ErrCodeInvalidTransferTarget))

	ErrNoAssignee            = result.Err(i18n.T("approval_no_assignee"), result.WithCode(ErrCodeNoAssignee))
	ErrAssigneeResolveFailed = result.Err(i18n.T("approval_assignee_resolve_failed"), result.WithCode(ErrCodeAssigneeResolveFailed))

	ErrFormValidationFailed = result.Err(i18n.T("approval_form_validation_failed"), result.WithCode(ErrCodeFormValidationFailed))
	ErrFieldNotEditable     = result.Err(i18n.T("approval_field_not_editable"), result.WithCode(ErrCodeFieldNotEditable))
	// ErrFormDataTooLarge rejects submissions whose JSON-encoded form data
	// would exceed FormDataMaxBytes. Stops malicious clients from blowing
	// up the JSONB column or driving the runtime into OOM via deeply
	// nested or massive maps.
	ErrFormDataTooLarge = result.Err(
		i18n.T("approval_form_data_too_large"),
		result.WithCode(ErrCodeFormValidationFailed),
	)

	ErrDelegationNotFound = result.Err(i18n.T("approval_delegation_not_found"), result.WithCode(ErrCodeDelegationNotFound))
	ErrDelegationConflict = result.Err(i18n.T("approval_delegation_conflict"), result.WithCode(ErrCodeDelegationConflict))

	ErrAccessDenied       = result.Err(i18n.T("approval_access_denied"), result.WithCode(ErrCodeAccessDenied))
	ErrInstanceNotRunning = result.Err(i18n.T("approval_instance_not_running"), result.WithCode(ErrCodeInstanceNotRunning))
)
