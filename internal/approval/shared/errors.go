package shared

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Error codes for the approval module (40xxx range).
const (
	ErrCodeFlowNotFound              = 40001
	ErrCodeFlowNotActive             = 40002
	ErrCodeNoPublishedVersion        = 40003
	ErrCodeVersionNotDraft           = 40004
	ErrCodeInvalidFlowDesign         = 40005
	ErrCodeFlowCodeExists            = 40006
	ErrCodeVersionNotFound           = 40007
	ErrCodeInvalidBusinessIdentifier = 40008

	ErrCodeInstanceNotFound          = 40101
	ErrCodeInstanceCompleted         = 40102
	ErrCodeNotAllowedInitiate        = 40103
	ErrCodeWithdrawNotAllowed        = 40104
	ErrCodeResubmitNotAllowed        = 40105
	ErrCodeInvalidInstanceTransition = 40106

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

	ErrCodeNoAssignee            = 40301
	ErrCodeAssigneeResolveFailed = 40302

	ErrCodeFormValidationFailed = 40401
	ErrCodeFieldNotEditable     = 40402

	ErrCodeDelegationNotFound = 40501
	ErrCodeDelegationConflict = 40502

	ErrCodeUrgeCooldown = 40601

	ErrCodeAccessDenied       = 40701
	ErrCodeInstanceNotRunning = 40702
)

// Error definitions. Messages are resolved through i18n at package
// init time using the language selected by VEF_I18N_LANGUAGE.
//
// These are sentinel values — callers use errors.Is to recognize them,
// so the Error value must remain stable. Switching i18n language at
// runtime (e.g. via i18n.SetLanguage in tests) will not update these
// frozen messages; new translations only take effect on process restart.
var (
	ErrFlowNotFound       = result.Err(i18n.T(ErrMessageFlowNotFound), result.WithCode(ErrCodeFlowNotFound))
	ErrFlowNotActive      = result.Err(i18n.T(ErrMessageFlowNotActive), result.WithCode(ErrCodeFlowNotActive))
	ErrNoPublishedVersion = result.Err(i18n.T(ErrMessageNoPublishedVersion), result.WithCode(ErrCodeNoPublishedVersion))
	ErrVersionNotDraft    = result.Err(i18n.T(ErrMessageVersionNotDraft), result.WithCode(ErrCodeVersionNotDraft))
	ErrInvalidFlowDesign  = result.Err(i18n.T(ErrMessageInvalidFlowDesign), result.WithCode(ErrCodeInvalidFlowDesign))
	ErrFlowCodeExists     = result.Err(i18n.T(ErrMessageFlowCodeExists), result.WithCode(ErrCodeFlowCodeExists))
	ErrVersionNotFound    = result.Err(i18n.T(ErrMessageVersionNotFound), result.WithCode(ErrCodeVersionNotFound))
	// ErrInvalidBusinessIdentifier rejects business_table / pk / status /
	// title field values that do not match a strict SQL-identifier regex.
	// Flow definitions interpolate these into UPDATE statements at runtime
	// so accepting arbitrary user input would open a SQL injection vector.
	ErrInvalidBusinessIdentifier = result.Err(
		i18n.T(ErrMessageInvalidBusinessIdentifier),
		result.WithCode(ErrCodeInvalidBusinessIdentifier),
	)

	ErrInstanceNotFound          = result.Err(i18n.T(ErrMessageInstanceNotFound), result.WithCode(ErrCodeInstanceNotFound))
	ErrInstanceCompleted         = result.Err(i18n.T(ErrMessageInstanceCompleted), result.WithCode(ErrCodeInstanceCompleted))
	ErrNotAllowedInitiate        = result.Err(i18n.T(ErrMessageNotAllowedInitiate), result.WithCode(ErrCodeNotAllowedInitiate))
	ErrWithdrawNotAllowed        = result.Err(i18n.T(ErrMessageWithdrawNotAllowed), result.WithCode(ErrCodeWithdrawNotAllowed))
	ErrResubmitNotAllowed        = result.Err(i18n.T(ErrMessageResubmitNotAllowed), result.WithCode(ErrCodeResubmitNotAllowed))
	ErrInvalidInstanceTransition = result.Err(i18n.T(ErrMessageInvalidInstanceTransition), result.WithCode(ErrCodeInvalidInstanceTransition))

	ErrTaskNotFound             = result.Err(i18n.T(ErrMessageTaskNotFound), result.WithCode(ErrCodeTaskNotFound))
	ErrTaskNotPending           = result.Err(i18n.T(ErrMessageTaskNotPending), result.WithCode(ErrCodeTaskNotPending))
	ErrNotAssignee              = result.Err(i18n.T(ErrMessageNotAssignee), result.WithCode(ErrCodeNotAssignee))
	ErrInvalidTaskTransition    = result.Err(i18n.T(ErrMessageInvalidTaskTransition), result.WithCode(ErrCodeInvalidTaskTransition))
	ErrRollbackNotAllowed       = result.Err(i18n.T(ErrMessageRollbackNotAllowed), result.WithCode(ErrCodeRollbackNotAllowed))
	ErrAddAssigneeNotAllowed    = result.Err(i18n.T(ErrMessageAddAssigneeNotAllowed), result.WithCode(ErrCodeAddAssigneeNotAllowed))
	ErrTransferNotAllowed       = result.Err(i18n.T(ErrMessageTransferNotAllowed), result.WithCode(ErrCodeTransferNotAllowed))
	ErrOpinionRequired          = result.Err(i18n.T(ErrMessageOpinionRequired), result.WithCode(ErrCodeOpinionRequired))
	ErrManualCcNotAllowed       = result.Err(i18n.T(ErrMessageManualCcNotAllowed), result.WithCode(ErrCodeManualCcNotAllowed))
	ErrRemoveAssigneeNotAllowed = result.Err(i18n.T(ErrMessageRemoveAssigneeNotAllowed), result.WithCode(ErrCodeRemoveAssigneeNotAllowed))
	ErrInvalidAddAssigneeType   = result.Err(i18n.T(ErrMessageInvalidAddAssigneeType), result.WithCode(ErrCodeInvalidAddAssigneeType))
	ErrNotApplicant             = result.Err(i18n.T(ErrMessageNotApplicant), result.WithCode(ErrCodeNotApplicant))
	ErrInvalidRollbackTarget    = result.Err(i18n.T(ErrMessageInvalidRollbackTarget), result.WithCode(ErrCodeInvalidRollbackTarget))
	ErrLastAssigneeRemoval      = result.Err(i18n.T(ErrMessageLastAssigneeRemoval), result.WithCode(ErrCodeLastAssigneeRemoval))
	ErrInvalidTransferTarget    = result.Err(i18n.T(ErrMessageInvalidTransferTarget), result.WithCode(ErrCodeInvalidTransferTarget))

	ErrNoAssignee            = result.Err(i18n.T(ErrMessageNoAssignee), result.WithCode(ErrCodeNoAssignee))
	ErrAssigneeResolveFailed = result.Err(i18n.T(ErrMessageAssigneeResolveFailed), result.WithCode(ErrCodeAssigneeResolveFailed))

	ErrFormValidationFailed = result.Err(i18n.T(ErrMessageFormValidationFailed), result.WithCode(ErrCodeFormValidationFailed))
	ErrFieldNotEditable     = result.Err(i18n.T(ErrMessageFieldNotEditable), result.WithCode(ErrCodeFieldNotEditable))
	// ErrFormDataTooLarge rejects submissions whose JSON-encoded form data
	// would exceed FormDataMaxBytes. Stops malicious clients from blowing
	// up the JSONB column or driving the runtime into OOM via deeply
	// nested or massive maps.
	ErrFormDataTooLarge = result.Err(
		i18n.T(ErrMessageFormDataTooLarge),
		result.WithCode(ErrCodeFormValidationFailed),
	)

	ErrDelegationNotFound = result.Err(i18n.T(ErrMessageDelegationNotFound), result.WithCode(ErrCodeDelegationNotFound))
	ErrDelegationConflict = result.Err(i18n.T(ErrMessageDelegationConflict), result.WithCode(ErrCodeDelegationConflict))

	ErrAccessDenied       = result.Err(i18n.T(ErrMessageAccessDenied), result.WithCode(ErrCodeAccessDenied))
	ErrInstanceNotRunning = result.Err(i18n.T(ErrMessageInstanceNotRunning), result.WithCode(ErrCodeInstanceNotRunning))
)
