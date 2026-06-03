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
