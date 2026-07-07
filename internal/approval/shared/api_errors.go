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
	// ErrInvalidBusinessIdentifier rejects business_table / pk / status field
	// values that do not match a strict SQL-identifier regex. Flow definitions
	// interpolate these into UPDATE statements at runtime so accepting arbitrary
	// user input would open a SQL injection vector.
	ErrInvalidBusinessIdentifier = result.Err(
		i18n.T("approval_invalid_business_identifier"),
		result.WithCode(ErrCodeInvalidBusinessIdentifier),
	)
	// ErrInvalidTitleTemplate rejects an instance title template that does
	// not parse as a Go text/template at flow create / update time, so a
	// broken template cannot silently break every subsequent submission.
	ErrInvalidTitleTemplate = result.Err(i18n.T("approval_invalid_title_template"), result.WithCode(ErrCodeInvalidTitleTemplate))
	// ErrInvalidFormDesign rejects a structurally broken form schema at
	// deploy time (duplicate keys, unknown field kind, uncompilable
	// validation pattern) so configuration faults never surface as data
	// errors to the applicant.
	ErrInvalidFormDesign = result.Err(i18n.T("approval_invalid_form_design"), result.WithCode(ErrCodeInvalidFormDesign))
	// ErrBindingIncomplete rejects a BindingMode=business flow missing any of
	// business_table / business_pk_field / business_status_field at create or
	// update time, so a half-configured business binding is caught when the
	// admin saves the flow rather than silently no-op'ing the status write-back
	// on the first completed instance.
	ErrBindingIncomplete = result.Err(i18n.T("approval_binding_incomplete"), result.WithCode(ErrCodeBindingIncomplete))

	// ErrInvalidBindingMode rejects an out-of-enum flow binding mode at save
	// time — an unknown value would silently behave like "standalone" and
	// disable the business write-back.
	ErrInvalidBindingMode = result.Err(i18n.T("approval_invalid_binding_mode"), result.WithCode(ErrCodeInvalidBindingMode))

	// ErrInvalidInitiatorKind rejects an out-of-enum initiator kind at save
	// time — an unknown value would silently never match any user.
	ErrInvalidInitiatorKind = result.Err(i18n.T("approval_invalid_initiator_kind"), result.WithCode(ErrCodeInvalidInitiatorKind))
	// ErrInvalidStorageMode rejects a deploy whose storage mode is neither
	// "json" nor "table". The mode is fixed for the version's lifetime and
	// drives whether a dedicated physical form table is generated at publish,
	// so an unrecognized value must be caught when the version is created.
	ErrInvalidStorageMode = result.Err(i18n.T("approval_invalid_storage_mode"), result.WithCode(ErrCodeInvalidStorageMode))
	// ErrFlowBindingLocked rejects changing a flow's business-binding
	// configuration (mode / table / pk / status / optional linkage columns)
	// while any instance of the flow is still running. Re-pointing the binding
	// mid-flight would make in-flight instances write their outcome back to a
	// different business record than they were started against, so the binding
	// is frozen until they complete.
	ErrFlowBindingLocked = result.Err(i18n.T("approval_flow_binding_locked"), result.WithCode(ErrCodeFlowBindingLocked))
	// ErrBindingColumnsConflict rejects two business-binding fields naming the
	// same column. The write-back emits a single UPDATE whose SET list would
	// then assign the column twice — a runtime SQL error surfacing in the
	// applicant's start transaction — so the conflict is caught when the admin
	// saves the flow instead.
	ErrBindingColumnsConflict = result.Err(i18n.T("approval_binding_columns_conflict"), result.WithCode(ErrCodeBindingColumnsConflict))

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
	ErrNoUsersSpecified         = result.Err(i18n.T("approval_no_users_specified"), result.WithCode(ErrCodeNoUsersSpecified))

	ErrNoAssignee            = result.Err(i18n.T("approval_no_assignee"), result.WithCode(ErrCodeNoAssignee))
	ErrAssigneeResolveFailed = result.Err(i18n.T("approval_assignee_resolve_failed"), result.WithCode(ErrCodeAssigneeResolveFailed))

	ErrFormValidationFailed = result.Err(i18n.T("approval_form_validation_failed"), result.WithCode(ErrCodeFormValidationFailed))
	// ErrFormDataTooLarge rejects submissions whose JSON-encoded form data
	// would exceed FormDataMaxBytes. Stops malicious clients from blowing
	// up the JSONB column or driving the runtime into OOM via deeply
	// nested or massive maps.
	ErrFormDataTooLarge = result.Err(
		i18n.T("approval_form_data_too_large"),
		result.WithCode(ErrCodeFormValidationFailed),
	)

	ErrAccessDenied = result.Err(i18n.T("approval_access_denied"), result.WithCode(ErrCodeAccessDenied))
	// ErrTerminateNotAllowed rejects force-closing an instance whose status
	// has no terminate transition on the instance state machine (already in
	// a final status).
	ErrTerminateNotAllowed = result.Err(i18n.T("approval_terminate_not_allowed"), result.WithCode(ErrCodeTerminateNotAllowed))
)
