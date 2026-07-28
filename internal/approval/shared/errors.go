package shared

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
	ErrCodeInvalidTitleTemplate      = 40009
	ErrCodeInvalidFormDesign         = 40010
	ErrCodeBindingIncomplete         = 40011
	ErrCodeInvalidBindingMode        = 40012
	ErrCodeInvalidInitiatorKind      = 40013
	ErrCodeInvalidStorageMode        = 40014
	// 40015 is retired (former flow-binding lock); do not reassign it, the
	// React side keys messages by code.
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
