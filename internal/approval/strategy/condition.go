package strategy

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/expression"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// Condition subjects resolved from the evaluation context instead of form data.
const (
	subjectApplicantID           = "applicantId"
	subjectApplicantDepartmentID = "applicantDepartmentId"
)

// NewFieldConditionEvaluator creates the evaluator for structured field
// conditions. Aggregators extend it with detail-table folds (sum / count /
// avg and any host-registered kind); an evaluator built without them only
// evaluates scalar conditions.
func NewFieldConditionEvaluator(aggregators ...approval.Aggregator) approval.ConditionEvaluator {
	indexed := make(map[approval.AggregateKind]approval.Aggregator, len(aggregators))
	for _, agg := range aggregators {
		indexed[agg.Kind()] = agg
	}

	return &FieldConditionEvaluator{aggregators: indexed}
}

// FieldConditionEvaluator evaluates structured field conditions natively in
// Go. Field conditions are data, not code: comparing them directly keeps the
// operator semantics typed per field kind (no string templating, no injection
// surface, no expression-engine round-trip) and guarantees the operator set
// stays in lockstep with the approval.ConditionOperator contract — adding an
// operator there without handling it here fails loudly at evaluation.
//
// Aggregate conditions fold a detail-table field through a registered
// approval.Aggregator before the comparison; the fold implementations are
// injected so new aggregate kinds extend the evaluator without modifying it.
type FieldConditionEvaluator struct {
	aggregators map[approval.AggregateKind]approval.Aggregator
}

func (*FieldConditionEvaluator) Kind() approval.ConditionKind {
	return approval.ConditionField
}

func (e *FieldConditionEvaluator) Evaluate(_ context.Context, cond approval.Condition, ec *approval.EvaluationContext) (bool, error) {
	if cond.Aggregate != "" {
		return e.evaluateAggregate(cond, ec)
	}

	subject := resolveSubjectValue(cond.Subject, ec)

	result, err := compareCondition(cond.Operator, subject, cond.Value)
	if err != nil {
		return false, fmt.Errorf("field condition %q %s: %w", cond.Subject, cond.Operator, err)
	}

	return result, nil
}

// evaluateAggregate folds the subject table's rows through the condition's
// aggregator and compares the result. Aggregates read form data only —
// globals and applicant built-ins are scalars by contract. A missing table
// value folds as zero rows; a non-list value or a non-numeric cell is a
// loud evaluation error, never a silent false: routing a request down the
// wrong branch is a business incident, a failed evaluation is a visible one.
func (e *FieldConditionEvaluator) evaluateAggregate(cond approval.Condition, ec *approval.EvaluationContext) (bool, error) {
	agg, ok := e.aggregators[cond.Aggregate]
	if !ok {
		return false, fmt.Errorf("%w: %q", ErrAggregatorNotFound, cond.Aggregate)
	}

	rows, err := tableRows(ec.FormData.Get(cond.Subject))
	if err != nil {
		return false, fmt.Errorf("aggregate %s over %q: %w", cond.Aggregate, cond.Subject, err)
	}

	var values []float64

	if cond.Aggregate.FoldsColumn() {
		values, err = extractNumericColumn(rows, cond.Column)
		if err != nil {
			return false, fmt.Errorf("aggregate %s over %q.%s: %w", cond.Aggregate, cond.Subject, cond.Column, err)
		}
	}

	result, matchable := agg.Fold(values, len(rows))
	if !matchable {
		return false, nil
	}

	matched, err := compareCondition(cond.Operator, result, cond.Value)
	if err != nil {
		return false, fmt.Errorf("aggregate condition %q %s: %w", cond.Subject, cond.Operator, err)
	}

	return matched, nil
}

// tableRows normalizes a detail-table value into its row maps. A missing
// value is an empty table; anything that is not a list of objects is a
// configuration/data fault surfaced to the caller.
func tableRows(value any) ([]map[string]any, error) {
	if value == nil {
		return nil, nil
	}

	list, err := toAnySlice(value)
	if err != nil {
		return nil, err
	}

	rows := make([]map[string]any, len(list))

	for i, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: row %d is %T, expected an object", ErrIncomparableValues, i+1, item)
		}

		rows[i] = row
	}

	return rows, nil
}

// extractNumericColumn pulls one column's numeric values out of the rows.
// Nil / absent cells are skipped — SQL aggregates ignore NULLs — while a
// present non-numeric cell fails loudly (the deploy validation pins the
// column to a number field, so this means corrupt data).
func extractNumericColumn(rows []map[string]any, column string) ([]float64, error) {
	values := make([]float64, 0, len(rows))

	for i, row := range rows {
		cell, present := row[column]
		if !present || cell == nil {
			continue
		}

		number, ok := shared.ToFloat64(cell)
		if !ok {
			return nil, fmt.Errorf("%w: row %d holds %T, expected a number", ErrIncomparableValues, i+1, cell)
		}

		values = append(values, number)
	}

	return values, nil
}

// resolveSubjectValue maps a SCALAR condition subject to its runtime value:
// the two applicant attributes come from the evaluation context, then
// host-supplied globals, then form data. Applicant subjects and globals are
// resolved before form data — a form field whose key collides with one of
// them is shadowed and can never be referenced by a scalar field condition,
// matching the expression environment where formData lives under its own
// namespace. Aggregate conditions deliberately skip this chain: they fold a
// detail-table field, and globals are scalars by contract, so the subject
// resolves against form data alone (see evaluateAggregate).
func resolveSubjectValue(subject string, ec *approval.EvaluationContext) any {
	switch subject {
	case subjectApplicantID:
		return ec.ApplicantID
	case subjectApplicantDepartmentID:
		if ec.ApplicantDepartmentID == nil {
			return nil
		}

		return *ec.ApplicantDepartmentID

	default:
		if value, ok := ec.Globals[subject]; ok {
			return value
		}

		return ec.FormData.Get(subject)
	}
}

// compareCondition dispatches one operator over the subject/expected pair.
// Operators that cannot apply to the runtime value type return an error
// rather than silently evaluating to false: routing a request down the wrong
// branch is a business incident, a failed evaluation is a visible one.
func compareCondition(operator approval.ConditionOperator, subject, expected any) (bool, error) {
	switch operator {
	case approval.OperatorEquals:
		return valuesEqual(subject, expected), nil
	case approval.OperatorNotEquals:
		return !valuesEqual(subject, expected), nil

	case approval.OperatorGreater, approval.OperatorGreaterOrEq, approval.OperatorLess, approval.OperatorLessOrEq:
		return compareOrdered(operator, subject, expected)

	case approval.OperatorIn:
		return valueInList(subject, expected)
	case approval.OperatorNotIn:
		contains, err := valueInList(subject, expected)
		if err != nil {
			return false, err
		}

		return !contains, nil

	case approval.OperatorContains:
		return subjectContains(subject, expected)
	case approval.OperatorNotContains:
		contains, err := subjectContains(subject, expected)
		if err != nil {
			return false, err
		}

		return !contains, nil

	case approval.OperatorStartsWith:
		return compareStringPair(subject, expected, strings.HasPrefix)
	case approval.OperatorEndsWith:
		return compareStringPair(subject, expected, strings.HasSuffix)

	case approval.OperatorIsEmpty:
		return isEmptyValue(subject), nil
	case approval.OperatorIsNotEmpty:
		return !isEmptyValue(subject), nil

	default:
		return false, fmt.Errorf("%w: %q", ErrUnsupportedOperator, operator)
	}
}

// valuesEqual compares two values with numeric awareness: numbers compare by
// value regardless of concrete type (json decodes to float64, Go callers may
// supply ints), everything else falls back to its canonical string form —
// the same normalization the form validator applies to select options.
func valuesEqual(a, b any) bool {
	if aNum, aOK := shared.ToFloat64(a); aOK {
		if bNum, bOK := shared.ToFloat64(b); bOK {
			return aNum == bNum
		}
	}

	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return fmt.Sprint(a) == fmt.Sprint(b)
}

// compareOrdered applies an ordering operator. Two numbers compare
// numerically; two strings compare lexicographically, which orders ISO-8601
// dates chronologically. Mixed or unordered types are an error.
func compareOrdered(operator approval.ConditionOperator, subject, expected any) (bool, error) {
	if subjectNum, ok := shared.ToFloat64(subject); ok {
		expectedNum, ok := shared.ToFloat64(expected)
		if !ok {
			return false, fmt.Errorf("%w: number vs %T", ErrIncomparableValues, expected)
		}

		return orderedResult(operator, subjectNum > expectedNum, subjectNum == expectedNum), nil
	}

	subjectStr, subjectOK := subject.(string)

	expectedStr, expectedOK := expected.(string)
	if subjectOK && expectedOK {
		return orderedResult(operator, subjectStr > expectedStr, subjectStr == expectedStr), nil
	}

	return false, fmt.Errorf("%w: %T vs %T", ErrIncomparableValues, subject, expected)
}

// orderedResult folds a three-way comparison (greater / equal) into the
// boolean answer for the given ordering operator.
func orderedResult(operator approval.ConditionOperator, greater, equal bool) bool {
	switch operator {
	case approval.OperatorGreater:
		return greater
	case approval.OperatorGreaterOrEq:
		return greater || equal
	case approval.OperatorLess:
		return !greater && !equal
	case approval.OperatorLessOrEq:
		return !greater
	default:
		return false
	}
}

// valueInList reports whether subject equals any element of the expected
// list. The designer always supplies a list for in / not_in; a non-list is a
// configuration error.
func valueInList(subject, expected any) (bool, error) {
	list, err := toAnySlice(expected)
	if err != nil {
		return false, err
	}

	for _, item := range list {
		if valuesEqual(subject, item) {
			return true, nil
		}
	}

	return false, nil
}

// subjectContains implements the contains operator for both shapes the form
// can produce: substring match on text fields, element match on multi-value
// fields (multi-select, upload lists).
func subjectContains(subject, expected any) (bool, error) {
	switch typed := subject.(type) {
	case nil:
		return false, nil
	case string:
		expectedStr, ok := expected.(string)
		if !ok {
			return false, fmt.Errorf("%w: string contains %T", ErrIncomparableValues, expected)
		}

		return strings.Contains(typed, expectedStr), nil

	default:
		list, err := toAnySlice(subject)
		if err != nil {
			return false, fmt.Errorf("%w: contains on %T", ErrIncomparableValues, subject)
		}

		for _, item := range list {
			if valuesEqual(item, expected) {
				return true, nil
			}
		}

		return false, nil
	}
}

// compareStringPair applies a string predicate, requiring both sides to be
// text.
func compareStringPair(subject, expected any, predicate func(s, prefix string) bool) (bool, error) {
	subjectStr, subjectOK := subject.(string)

	expectedStr, expectedOK := expected.(string)
	if !subjectOK || !expectedOK {
		return false, fmt.Errorf("%w: %T vs %T", ErrIncomparableValues, subject, expected)
	}

	return predicate(subjectStr, expectedStr), nil
}

// isEmptyValue is the typed emptiness check behind is_empty / is_not_empty:
// nil, blank text, and empty collections are empty; numbers and booleans
// never are. This is deliberately total over all value types so the operator
// is safe on every field kind the designer offers it for.
func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// toAnySlice normalizes the two list shapes a JSON decoder or Go caller can
// produce.
func toAnySlice(value any) ([]any, error) {
	switch typed := value.(type) {
	case []any:
		return typed, nil
	case []string:
		list := make([]any, len(typed))
		for i, item := range typed {
			list[i] = item
		}

		return list, nil

	default:
		return nil, fmt.Errorf("%w: expected a list, got %T", ErrIncomparableValues, value)
	}
}

// NewExpressionConditionEvaluator creates the evaluator for free-form
// expression conditions backed by the framework expression engine.
func NewExpressionConditionEvaluator(engine expression.Engine) approval.ConditionEvaluator {
	return &ExpressionConditionEvaluator{engine: engine}
}

// ExpressionConditionEvaluator evaluates expression conditions through the
// framework's expression.Engine abstraction, so approval depends on the
// engine contract rather than any concrete backend. The expression
// environment carries formData (the instance form), applicantId,
// applicantDepartmentId, and every host-supplied global from
// EvaluationContext.Globals as a top-level binding (built-ins win a name
// collision).
type ExpressionConditionEvaluator struct {
	engine expression.Engine
}

func (*ExpressionConditionEvaluator) Kind() approval.ConditionKind {
	return approval.ConditionExpression
}

func (e *ExpressionConditionEvaluator) Evaluate(ctx context.Context, cond approval.Condition, ec *approval.EvaluationContext) (bool, error) {
	var departmentID string
	if ec.ApplicantDepartmentID != nil {
		departmentID = *ec.ApplicantDepartmentID
	}

	env := make(map[string]any, len(ec.Globals)+3)
	maps.Copy(env, ec.Globals)

	// Built-in bindings are assigned last so they always win a collision with
	// a host-supplied global.
	env["formData"] = ec.FormData.ToMap()
	env["applicantId"] = ec.ApplicantID
	env["applicantDepartmentId"] = departmentID

	value, err := e.engine.Evaluate(ctx, cond.Expression, env)
	if err != nil {
		return false, fmt.Errorf("evaluate condition expression: %w", err)
	}

	result, err := value.Bool()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrExpressionReturnedNonBool, err)
	}

	return result, nil
}
