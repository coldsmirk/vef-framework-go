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
)
