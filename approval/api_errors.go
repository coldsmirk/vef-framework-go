package approval

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for approval-domain API errors (40xxx range).
// 400xx: flow definition; 401xx: instance; 402xx: task; 403xx: assignee
// resolution; 404xx: form data; 406xx: urge; 407xx: access / admin.
//
// The React side keys messages by code, so a retired code is never reassigned.
const (
	ErrCodeFlowNotFound              = 40001
	ErrCodeFlowNotActive             = 40002
	ErrCodeNoPublishedVersion        = 40003
	ErrCodeVersionNotDraft           = 40004
	ErrCodeInvalidFlowDesign         = 40005
	ErrCodeFlowCodeExists            = 40006
	ErrCodeVersionNotFound           = 40007
	ErrCodeInvalidBusinessIdentifier = 40008
	ErrCodeInvalidTitleTemplate      = 40009
	ErrCodeInvalidFormDesign         = 40010
	ErrCodeBindingIncomplete         = 40011
	ErrCodeInvalidBindingMode        = 40012
	ErrCodeInvalidInitiatorKind      = 40013
	ErrCodeInvalidStorageMode        = 40014
	// 40015 is retired (former flow-binding lock); do not reassign it.
	ErrCodeBindingColumnsConflict      = 40016
	ErrCodeBindingUnexpected           = 40017
	ErrCodeBindingSchemaInvalid        = 40018
	ErrCodeBindingKeyNotUnique         = 40019
	ErrCodeBindingStatusMappingInvalid = 40020
	ErrCodeInvalidFlowLabel            = 40021
	ErrCodeInitiatorsNotAllowed        = 40022
	ErrCodeInitiatorsRequired          = 40023

	ErrCodeInstanceNotFound          = 40101
	ErrCodeInstanceCompleted         = 40102
	ErrCodeNotAllowedInitiate        = 40103
	ErrCodeWithdrawNotAllowed        = 40104
	ErrCodeResubmitNotAllowed        = 40105
	ErrCodeInvalidInstanceTransition = 40106
	ErrCodeBusinessRefRequired       = 40107
	ErrCodeBindingTargetBusy         = 40108
	ErrCodeInvalidBusinessRef        = 40109
	ErrCodeBindingProjectionNotFound = 40110

	ErrCodeTaskNotFound             = 40201
	ErrCodeTaskNotPending           = 40202
	ErrCodeNotAssignee              = 40203
	ErrCodeInvalidTaskTransition    = 40204
	ErrCodeRollbackNotAllowed       = 40205
	ErrCodeAddAssigneeNotAllowed    = 40206
	ErrCodeTransferNotAllowed       = 40207
	ErrCodeOpinionRequired          = 40208
	ErrCodeManualCcNotAllowed       = 40209
	ErrCodeRemoveAssigneeNotAllowed = 40210
	ErrCodeInvalidAddAssigneeType   = 40211
	ErrCodeNotApplicant             = 40212
	ErrCodeInvalidRollbackTarget    = 40213
	ErrCodeLastAssigneeRemoval      = 40214
	ErrCodeInvalidTransferTarget    = 40215
	ErrCodeNoUsersSpecified         = 40216

	ErrCodeNoAssignee            = 40301
	ErrCodeAssigneeResolveFailed = 40302

	ErrCodeFormValidationFailed = 40401

	ErrCodeUrgeCooldown = 40601

	ErrCodeAccessDenied        = 40701
	ErrCodeTerminateNotAllowed = 40702
)

// Outward-facing error sentinels. Messages are resolved through i18n at
// package init time using the language selected by VEF_I18N_LANGUAGE.
//
// These are sentinel values — callers use errors.Is to recognize them, which
// result.Error implements by comparing Code, so the values must remain
// stable. They are public because Service returns them to host code, which
// must be able to tell "flow not active" from "not allowed to initiate"
// without comparing numeric codes. Switching i18n language at runtime (e.g.
// via i18n.SetLanguage in tests) will not update these frozen messages; new
// translations only take effect on process restart.
var (
	ErrFlowNotFound       = result.Err(i18n.T("approval_flow_not_found"), result.WithCode(ErrCodeFlowNotFound))
	ErrFlowNotActive      = result.Err(i18n.T("approval_flow_not_active"), result.WithCode(ErrCodeFlowNotActive))
	ErrNoPublishedVersion = result.Err(i18n.T("approval_no_published_version"), result.WithCode(ErrCodeNoPublishedVersion))
	ErrVersionNotDraft    = result.Err(i18n.T("approval_version_not_draft"), result.WithCode(ErrCodeVersionNotDraft))
	ErrInvalidFlowDesign  = result.Err(i18n.T("approval_invalid_flow_design"), result.WithCode(ErrCodeInvalidFlowDesign))
	ErrFlowCodeExists     = result.Err(i18n.T("approval_flow_code_exists"), result.WithCode(ErrCodeFlowCodeExists))
	ErrVersionNotFound    = result.Err(i18n.T("approval_version_not_found"), result.WithCode(ErrCodeVersionNotFound))
	// ErrInvalidBusinessIdentifier rejects dynamic business table / column
	// names that do not match the SQL-identifier policy enforced by
	// ValidateBusinessIdentifier, which returns it directly. Flow CRUD
	// validation surfaces it to operators; the write-back re-checks the same
	// rule as defense-in-depth.
	ErrInvalidBusinessIdentifier = result.Err(
		i18n.T("approval_invalid_business_identifier"),
		result.WithCode(ErrCodeInvalidBusinessIdentifier),
	)
	// ErrInvalidTitleTemplate rejects an instance title template that does
	// not parse as a Go text/template at flow create / update time, so a
	// broken template cannot silently break every subsequent submission.
	ErrInvalidTitleTemplate = result.Err(i18n.T("approval_invalid_title_template"), result.WithCode(ErrCodeInvalidTitleTemplate))
	// ErrInvalidFlowLabel rejects a flow label whose key would silently break
	// the label equality filter or whose value blows past the storage bound,
	// at flow create / update time.
	ErrInvalidFlowLabel = result.Err(i18n.T("approval_invalid_flow_label"), result.WithCode(ErrCodeInvalidFlowLabel))
	// ErrInvalidFormDesign rejects a structurally broken form schema at
	// deploy time (duplicate keys, unknown field kind, uncompilable
	// validation pattern) so configuration faults never surface as data
	// errors to the applicant.
	ErrInvalidFormDesign = result.Err(i18n.T("approval_invalid_form_design"), result.WithCode(ErrCodeInvalidFormDesign))
	// ErrBindingIncomplete rejects a BindingMode=business flow missing its table,
	// key columns, status column, or instance-ID fencing column at save time.
	ErrBindingIncomplete = result.Err(i18n.T("approval_binding_incomplete"), result.WithCode(ErrCodeBindingIncomplete))

	// ErrInvalidBindingMode rejects an out-of-enum flow binding mode at save
	// time — an unknown value would silently behave like "standalone" and
	// disable the business write-back.
	ErrInvalidBindingMode = result.Err(i18n.T("approval_invalid_binding_mode"), result.WithCode(ErrCodeInvalidBindingMode))

	// ErrInvalidInitiatorKind rejects an out-of-enum initiator kind at save
	// time — an unknown value would silently never match any user.
	ErrInvalidInitiatorKind = result.Err(i18n.T("approval_invalid_initiator_kind"), result.WithCode(ErrCodeInvalidInitiatorKind))

	// ErrInitiatorsNotAllowed rejects saving initiator rules on a flow open to
	// everyone. The two settings are mutually exclusive: initiation permission
	// short-circuits on isAllInitiationAllowed and never consults the rules, so
	// storing them would show a restriction in the admin UI that does not hold.
	ErrInitiatorsNotAllowed = result.Err(i18n.T("approval_initiators_not_allowed"), result.WithCode(ErrCodeInitiatorsNotAllowed))

	// ErrInitiatorsRequired rejects a restricted flow that names nobody who may
	// start it — no rules at all, or a rule selecting nothing, which matches no
	// applicant and leaves the flow just as unstartable. It also keeps an empty
	// initiator list unambiguous: it means exactly "open to everyone", so one
	// query answers who may start a flow.
	ErrInitiatorsRequired = result.Err(i18n.T("approval_initiators_required"), result.WithCode(ErrCodeInitiatorsRequired))
	// ErrInvalidStorageMode rejects a deploy whose storage mode is neither
	// "json" nor "table". The mode is fixed for the version's lifetime and
	// drives whether a dedicated physical form table is generated at publish,
	// so an unrecognized value must be caught when the version is created.
	ErrInvalidStorageMode = result.Err(i18n.T("approval_invalid_storage_mode"), result.WithCode(ErrCodeInvalidStorageMode))
	// ErrBindingColumnsConflict rejects duplicate key/write-back columns, which
	// could otherwise mutate the lookup key or assign one column twice.
	ErrBindingColumnsConflict = result.Err(i18n.T("approval_binding_columns_conflict"), result.WithCode(ErrCodeBindingColumnsConflict))
	// ErrBindingUnexpected rejects business binding configuration on a
	// standalone flow.
	ErrBindingUnexpected = result.Err(i18n.T("approval_binding_unexpected"), result.WithCode(ErrCodeBindingUnexpected))
	// ErrBindingKeyNotUnique rejects key columns that are not backed by one
	// complete, non-null primary or unique key.
	ErrBindingKeyNotUnique = result.Err(i18n.T("approval_binding_key_not_unique"), result.WithCode(ErrCodeBindingKeyNotUnique))
	// ErrBindingStatusMappingInvalid rejects unknown approval statuses and blank
	// target values in a business status mapping.
	ErrBindingStatusMappingInvalid = result.Err(
		i18n.T("approval_binding_status_mapping_invalid"),
		result.WithCode(ErrCodeBindingStatusMappingInvalid),
	)

	ErrInstanceNotFound          = result.Err(i18n.T("approval_instance_not_found"), result.WithCode(ErrCodeInstanceNotFound))
	ErrInstanceCompleted         = result.Err(i18n.T("approval_instance_completed"), result.WithCode(ErrCodeInstanceCompleted))
	ErrNotAllowedInitiate        = result.Err(i18n.T("approval_not_allowed_initiate"), result.WithCode(ErrCodeNotAllowedInitiate))
	ErrWithdrawNotAllowed        = result.Err(i18n.T("approval_withdraw_not_allowed"), result.WithCode(ErrCodeWithdrawNotAllowed))
	ErrResubmitNotAllowed        = result.Err(i18n.T("approval_resubmit_not_allowed"), result.WithCode(ErrCodeResubmitNotAllowed))
	ErrInvalidInstanceTransition = result.Err(i18n.T("approval_invalid_instance_transition"), result.WithCode(ErrCodeInvalidInstanceTransition))
	ErrBusinessRefRequired       = result.Err(i18n.T("approval_business_ref_required"), result.WithCode(ErrCodeBusinessRefRequired))
	ErrBindingTargetBusy         = result.Err(i18n.T("approval_binding_target_busy"), result.WithCode(ErrCodeBindingTargetBusy))
	ErrInvalidBusinessRef        = result.Err(i18n.T("approval_invalid_business_ref"), result.WithCode(ErrCodeInvalidBusinessRef))
	ErrBindingProjectionNotFound = result.Err(
		i18n.T("approval_binding_projection_not_found"),
		result.WithCode(ErrCodeBindingProjectionNotFound),
	)

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
	// would exceed vef.approval.form_data_max_bytes. Stops malicious clients
	// from blowing up the JSONB column or driving the runtime into OOM via
	// deeply nested or massive maps.
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

// ErrBindingTableMissing rejects a binding whose configured table does not
// exist in the primary database, naming the missing table. It carries
// ErrCodeBindingSchemaInvalid so errors.Is and the React code map still treat
// it as one class with the column variant.
func ErrBindingTableMissing(table string) result.Error {
	return result.Err(
		i18n.T("approval_binding_table_missing", map[string]any{"table": table}),
		result.WithCode(ErrCodeBindingSchemaInvalid),
	)
}

// ErrBindingColumnMissing rejects a binding whose configured column does not
// exist in the primary database, naming the missing column. It shares
// ErrCodeBindingSchemaInvalid with the table variant.
func ErrBindingColumnMissing(column string) result.Error {
	return result.Err(
		i18n.T("approval_binding_column_missing", map[string]any{"column": column}),
		result.WithCode(ErrCodeBindingSchemaInvalid),
	)
}
