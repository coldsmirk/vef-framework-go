package shared

import "github.com/coldsmirk/vef-framework-go/result"

// Error codes for the approval module (40xxx range).
const (
	ErrCodeFlowNotFound       = 40001
	ErrCodeFlowNotActive      = 40002
	ErrCodeNoPublishedVersion = 40003
	ErrCodeVersionNotDraft    = 40004
	ErrCodeInvalidFlowDesign  = 40005
	ErrCodeFlowCodeExists     = 40006
	ErrCodeVersionNotFound    = 40007

	ErrCodeInstanceNotFound   = 40101
	ErrCodeInstanceCompleted  = 40102
	ErrCodeNotAllowedInitiate = 40103
	ErrCodeWithdrawNotAllowed = 40104
	ErrCodeResubmitNotAllowed = 40105

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

// Error definitions.
var (
	ErrFlowNotFound       = result.Err("流程不存在", result.WithCode(ErrCodeFlowNotFound))
	ErrFlowNotActive      = result.Err("流程未激活", result.WithCode(ErrCodeFlowNotActive))
	ErrNoPublishedVersion = result.Err("无已发布版本", result.WithCode(ErrCodeNoPublishedVersion))
	ErrVersionNotDraft    = result.Err("版本非草稿状态", result.WithCode(ErrCodeVersionNotDraft))
	ErrInvalidFlowDesign  = result.Err("流程设计无效", result.WithCode(ErrCodeInvalidFlowDesign))
	ErrFlowCodeExists     = result.Err("流程编码已存在", result.WithCode(ErrCodeFlowCodeExists))
	ErrVersionNotFound    = result.Err("流程版本不存在", result.WithCode(ErrCodeVersionNotFound))

	ErrInstanceNotFound   = result.Err("审批实例不存在", result.WithCode(ErrCodeInstanceNotFound))
	ErrInstanceCompleted  = result.Err("审批实例已结束", result.WithCode(ErrCodeInstanceCompleted))
	ErrNotAllowedInitiate = result.Err("无权发起此流程", result.WithCode(ErrCodeNotAllowedInitiate))
	ErrWithdrawNotAllowed = result.Err("当前状态不允许撤回", result.WithCode(ErrCodeWithdrawNotAllowed))
	ErrResubmitNotAllowed = result.Err("当前状态不允许重新提交", result.WithCode(ErrCodeResubmitNotAllowed))

	ErrTaskNotFound             = result.Err("任务不存在", result.WithCode(ErrCodeTaskNotFound))
	ErrTaskNotPending           = result.Err("任务非待处理状态", result.WithCode(ErrCodeTaskNotPending))
	ErrNotAssignee              = result.Err("非任务审批人", result.WithCode(ErrCodeNotAssignee))
	ErrInvalidTaskTransition    = result.Err("非法的任务状态转换", result.WithCode(ErrCodeInvalidTaskTransition))
	ErrRollbackNotAllowed       = result.Err("当前节点不允许回退", result.WithCode(ErrCodeRollbackNotAllowed))
	ErrAddAssigneeNotAllowed    = result.Err("当前节点不允许加签", result.WithCode(ErrCodeAddAssigneeNotAllowed))
	ErrTransferNotAllowed       = result.Err("当前节点不允许转交", result.WithCode(ErrCodeTransferNotAllowed))
	ErrOpinionRequired          = result.Err("审批意见必填", result.WithCode(ErrCodeOpinionRequired))
	ErrManualCcNotAllowed       = result.Err("当前节点不允许手动抄送", result.WithCode(ErrCodeManualCcNotAllowed))
	ErrRemoveAssigneeNotAllowed = result.Err("当前节点不允许减签", result.WithCode(ErrCodeRemoveAssigneeNotAllowed))
	ErrInvalidAddAssigneeType   = result.Err("非法的加签类型", result.WithCode(ErrCodeInvalidAddAssigneeType))
	ErrNotApplicant             = result.Err("非审批发起人，无权操作", result.WithCode(ErrCodeNotApplicant))
	ErrInvalidRollbackTarget    = result.Err("非法的回退目标节点", result.WithCode(ErrCodeInvalidRollbackTarget))
	ErrLastAssigneeRemoval      = result.Err("无法移除最后一个有效审批人", result.WithCode(ErrCodeLastAssigneeRemoval))
	ErrInvalidTransferTarget    = result.Err("非法的转交目标审批人", result.WithCode(ErrCodeInvalidTransferTarget))

	ErrNoAssignee            = result.Err("无可用审批人", result.WithCode(ErrCodeNoAssignee))
	ErrAssigneeResolveFailed = result.Err("解析审批人失败", result.WithCode(ErrCodeAssigneeResolveFailed))

	ErrFormValidationFailed = result.Err("表单验证失败", result.WithCode(ErrCodeFormValidationFailed))
	ErrFieldNotEditable     = result.Err("字段不可编辑", result.WithCode(ErrCodeFieldNotEditable))

	ErrDelegationNotFound = result.Err("委托记录不存在", result.WithCode(ErrCodeDelegationNotFound))
	ErrDelegationConflict = result.Err("委托时间段冲突", result.WithCode(ErrCodeDelegationConflict))

	ErrUrgeCooldown = result.Err("催办冷却中，请稍后再试", result.WithCode(ErrCodeUrgeCooldown))

	ErrAccessDenied       = result.Err("无权访问此审批实例", result.WithCode(ErrCodeAccessDenied))
	ErrInstanceNotRunning = result.Err("实例非运行状态，无法操作", result.WithCode(ErrCodeInstanceNotRunning))
)
