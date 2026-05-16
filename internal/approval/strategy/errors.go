package strategy

import "errors"

var (
	// Assignee resolver errors.
	ErrAssigneeServiceNil        = errors.New("assignee service is nil")
	ErrApplicantIDEmpty          = errors.New("applicant ID is empty")
	ErrFormFieldNameEmpty        = errors.New("form field name is empty")
	ErrFormFieldValueEmpty       = errors.New("form field value is empty")
	ErrUnsupportedFieldValueType = errors.New("unsupported form field value type")
	ErrAssigneeResolverNotFound  = errors.New("assignee resolver not found")

	// Registry lookup errors.
	ErrPassRuleNotFound           = errors.New("pass rule strategy not found")
	ErrConditionEvaluatorNotFound = errors.New("condition evaluator not found")

	// Expression evaluation errors.
	ErrExpressionReturnedNonBool = errors.New("expression returned non-bool type")

	// Registry validation errors. Surface during boot when the framework
	// strategy module is missing one of the built-in enum values.
	errBuiltinPassRuleMissing  = errors.New("no PassRuleStrategy registered for built-in PassRule")
	errBuiltinAssigneeMissing  = errors.New("no AssigneeResolver registered for built-in AssigneeKind")
	errBuiltinEvaluatorMissing = errors.New("no ConditionEvaluator registered for built-in ConditionKind")
)
