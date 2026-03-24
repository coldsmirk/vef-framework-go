package strategy

import (
	"context"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// NewFieldConditionEvaluator creates a new FieldConditionEvaluator.
func NewFieldConditionEvaluator() approval.ConditionEvaluator {
	return &FieldConditionEvaluator{delegate: NewExpressionConditionEvaluator()}
}

// FieldConditionEvaluator evaluates field-based conditions by converting them to expressions
// and delegating to ExpressionConditionEvaluator.
type FieldConditionEvaluator struct {
	delegate approval.ConditionEvaluator
}

func (*FieldConditionEvaluator) Kind() approval.ConditionKind {
	return approval.ConditionField
}

func (e *FieldConditionEvaluator) Evaluate(ctx context.Context, cond approval.Condition, ec *approval.EvaluationContext) (bool, error) {
	expression := buildFieldExpression(cond)

	return e.delegate.Evaluate(ctx, approval.Condition{Expression: expression}, ec)
}

// NewExpressionConditionEvaluator creates a new ExpressionConditionEvaluator.
func NewExpressionConditionEvaluator() approval.ConditionEvaluator {
	return new(ExpressionConditionEvaluator)
}

// ExpressionConditionEvaluator evaluates expr-lang expressions.
type ExpressionConditionEvaluator struct{}

func (*ExpressionConditionEvaluator) Kind() approval.ConditionKind {
	return approval.ConditionExpression
}

func (*ExpressionConditionEvaluator) Evaluate(_ context.Context, cond approval.Condition, ec *approval.EvaluationContext) (bool, error) {
	var departmentID string
	if ec.ApplicantDepartmentID != nil {
		departmentID = *ec.ApplicantDepartmentID
	}

	env := map[string]any{
		"formData":              ec.FormData.ToMap(),
		"applicantId":           ec.ApplicantID,
		"applicantDepartmentId": departmentID,
	}

	program, err := expr.Compile(cond.Expression, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("compile expression: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("run expression: %w", err)
	}

	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("%w: %T", ErrExpressionReturnedNonBool, result)
	}

	return boolResult, nil
}

// buildFieldExpression converts a structured field condition to an expr-lang expression string.
func buildFieldExpression(cond approval.Condition) string {
	subject := resolveSubjectExpr(cond.Subject)
	rhs := formatExprValue(cond.Value)

	switch cond.Operator {
	case "eq":
		return subject + " == " + rhs
	case "ne":
		return subject + " != " + rhs
	case "gt":
		return subject + " > " + rhs
	case "gte":
		return subject + " >= " + rhs
	case "lt":
		return subject + " < " + rhs
	case "lte":
		return subject + " <= " + rhs
	case "in":
		return subject + " in " + rhs
	case "not_in":
		return "not (" + subject + " in " + rhs + ")"
	case "contains":
		return subject + " contains " + rhs
	case "not_contains":
		return "not (" + subject + " contains " + rhs + ")"
	case "starts_with":
		return subject + " startsWith " + rhs
	case "ends_with":
		return subject + " endsWith " + rhs
	case "is_empty":
		return `len(` + subject + ` ?? "") == 0`
	case "is_not_empty":
		return `len(` + subject + ` ?? "") > 0`
	default:
		return "false"
	}
}

// resolveSubjectExpr maps a condition subject to its expr-lang accessor.
func resolveSubjectExpr(subject string) string {
	switch subject {
	case "applicantId", "applicantDepartmentId":
		return subject
	default:
		return fmt.Sprintf(`formData["%s"]`, subject)
	}
}

// formatExprValue converts a Go value to its expr-lang literal representation.
func formatExprValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "nil"
	case string:
		return fmt.Sprintf("%q", val)
	case []string:
		parts := make([]string, len(val))
		for i, s := range val {
			parts[i] = fmt.Sprintf("%q", s)
		}

		return "[" + strings.Join(parts, ", ") + "]"

	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatExprValue(item)
		}

		return "[" + strings.Join(parts, ", ") + "]"

	default:
		return fmt.Sprint(val)
	}
}
