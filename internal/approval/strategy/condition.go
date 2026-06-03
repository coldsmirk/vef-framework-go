package strategy

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

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

// ExpressionConditionEvaluator evaluates approval conditions written in expr-lang syntax
// (e.g. startsWith, ??"", contains). It deliberately uses expr-lang directly rather than
// the framework's swappable expression.Engine, whose only current backend is Zen (CGO).
// Routing approval evaluation through expression.Engine would force a CGO dependency into
// every framework build and break pure-Go environments. Migrate to expression.Engine only
// once a pure-Go backend is available. Compiled programs are cached by expression source
// to avoid repeated parse+type-check costs across multiple condition evaluations.
type ExpressionConditionEvaluator struct {
	cache sync.Map // key: string (expression source), value: *vm.Program
}

func (*ExpressionConditionEvaluator) Kind() approval.ConditionKind {
	return approval.ConditionExpression
}

func (e *ExpressionConditionEvaluator) Evaluate(_ context.Context, cond approval.Condition, ec *approval.EvaluationContext) (bool, error) {
	var departmentID string
	if ec.ApplicantDepartmentID != nil {
		departmentID = *ec.ApplicantDepartmentID
	}

	env := map[string]any{
		"formData":              ec.FormData.ToMap(),
		"applicantId":           ec.ApplicantID,
		"applicantDepartmentId": departmentID,
	}

	program, err := e.compile(cond.Expression, env)
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

// compile returns the compiled program for the given expression, using the cache to avoid
// repeated compilation. env is used only for type-checking on first compile.
func (e *ExpressionConditionEvaluator) compile(source string, env map[string]any) (*vm.Program, error) {
	if cached, ok := e.cache.Load(source); ok {
		return cached.(*vm.Program), nil
	}

	program, err := expr.Compile(source, expr.Env(env), expr.AsBool())
	if err != nil {
		return nil, err
	}

	e.cache.Store(source, program)

	return program, nil
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
