package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/expression/exprlang"
)

// TestFieldConditionEvaluator tests field condition evaluator scenarios.
func TestFieldConditionEvaluator(t *testing.T) {
	e := NewFieldConditionEvaluator()
	assert.Equal(t, approval.ConditionField, e.Kind(), "Should return ConditionField type")

	ctx := context.Background()
	ec := &approval.EvaluationContext{
		FormData: approval.FormData{
			"name":       "alice",
			"amount":     5000,
			"amountF":    5000.5,
			"department": "sales",
			"tags":       []string{"vip", "premium"},
			"greeting":   "hello world",
			"empty_str":  "",
			"int_val":    int64(100),
			"start_date": "2026-01-10",
		},
		ApplicantID:           "user1",
		ApplicantDepartmentID: new("dept1"),
	}

	tests := []struct {
		name     string
		cond     approval.Condition
		expected bool
	}{
		// eq / ne
		{"EqStringMatch", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "eq", Value: "alice"}, true},
		{"EqStringNoMatch", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "eq", Value: "bob"}, false},
		{"EqNumericCrossType", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "eq", Value: 5000.0}, true},
		{"EqNilSubjectNilValue", approval.Condition{Kind: approval.ConditionField, Subject: "nonexistent", Operator: "eq", Value: nil}, true},
		{"NeString", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "ne", Value: "bob"}, true},
		{"NeStringSame", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "ne", Value: "alice"}, false},

		// gt / gte / lt / lte (int)
		{"GtIntTrue", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "gt", Value: 3000}, true},
		{"GtIntFalse", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "gt", Value: 5000}, false},
		{"GteIntEqual", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "gte", Value: 5000}, true},
		{"LtIntTrue", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "lt", Value: 6000}, true},
		{"LtIntFalse", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "lt", Value: 5000}, false},
		{"LteIntEqual", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "lte", Value: 5000}, true},

		// gt / lt (float64)
		{"GtFloatTrue", approval.Condition{Kind: approval.ConditionField, Subject: "amountF", Operator: "gt", Value: 5000.0}, true},
		{"LtFloatTrue", approval.Condition{Kind: approval.ConditionField, Subject: "amountF", Operator: "lt", Value: 5001.0}, true},

		// cross-type numeric comparison (int field vs float value)
		{"GtIntFieldFloatValue", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "gt", Value: 4999.9}, true},
		{"LtFloatFieldIntValue", approval.Condition{Kind: approval.ConditionField, Subject: "amountF", Operator: "lt", Value: 5001}, true},

		// string ordering (ISO dates compare chronologically)
		{"GtDateString", approval.Condition{Kind: approval.ConditionField, Subject: "start_date", Operator: "gt", Value: "2026-01-01"}, true},
		{"LteDateString", approval.Condition{Kind: approval.ConditionField, Subject: "start_date", Operator: "lte", Value: "2026-01-10"}, true},

		// in / not_in
		{"InStringArray", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{"alice", "bob"}}, true},
		{"InStringArrayNotFound", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{"bob", "charlie"}}, false},
		{"InEmptySlice", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{}}, false},
		{"InNumericList", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "in", Value: []any{5000.0, 9000.0}}, true},
		{"NotInStringArray", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{"bob", "charlie"}}, true},
		{"NotInStringArrayFound", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{"alice", "bob"}}, false},
		{"NotInEmptySlice", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{}}, true},
		{"InWithAnySlice", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []any{"alice", "charlie"}}, true},

		// contains / not_contains
		{"ContainsTrue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: "world"}, true},
		{"ContainsFalse", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: "mars"}, false},
		{"ContainsEmptyValue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: ""}, true},
		{"ContainsOnList", approval.Condition{Kind: approval.ConditionField, Subject: "tags", Operator: "contains", Value: "vip"}, true},
		{"ContainsOnListMissing", approval.Condition{Kind: approval.ConditionField, Subject: "tags", Operator: "contains", Value: "basic"}, false},
		{"ContainsNilSubject", approval.Condition{Kind: approval.ConditionField, Subject: "nonexistent", Operator: "contains", Value: "x"}, false},
		{"NotContainsTrue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "not_contains", Value: "mars"}, true},
		{"NotContainsFalse", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "not_contains", Value: "world"}, false},

		// starts_with / ends_with
		{"StartsWithTrue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "starts_with", Value: "hello"}, true},
		{"StartsWithFalse", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "starts_with", Value: "world"}, false},
		{"StartsWithEmptyValue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "starts_with", Value: ""}, true},
		{"EndsWithTrue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "ends_with", Value: "world"}, true},
		{"EndsWithFalse", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "ends_with", Value: "hello"}, false},
		{"EndsWithEmptyValue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "ends_with", Value: ""}, true},

		// is_empty / is_not_empty
		{"IsEmptyNil", approval.Condition{Kind: approval.ConditionField, Subject: "nonexistent", Operator: "is_empty"}, true},
		{"IsEmptyEmptyString", approval.Condition{Kind: approval.ConditionField, Subject: "empty_str", Operator: "is_empty"}, true},
		{"IsEmptyNonEmpty", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "is_empty"}, false},
		{"IsEmptyOnNumber", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "is_empty"}, false},
		{"IsNotEmptyOnNumber", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "is_not_empty"}, true},
		{"IsNotEmptyString", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "is_not_empty"}, true},
		{"IsNotEmptyNil", approval.Condition{Kind: approval.ConditionField, Subject: "nonexistent", Operator: "is_not_empty"}, false},

		// Special subjects
		{"ApplicantSubject", approval.Condition{Kind: approval.ConditionField, Subject: "applicantId", Operator: "eq", Value: "user1"}, true},
		{"DepartmentSubject", approval.Condition{Kind: approval.ConditionField, Subject: "applicantDepartmentId", Operator: "eq", Value: "dept1"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, tt.cond, ec)
			require.NoError(t, err, "Should evaluate without error")
			assert.Equal(t, tt.expected, result, "Should return expected result")
		})
	}
}

// TestFieldConditionEvaluatorErrors covers configurations the evaluator must
// reject loudly instead of silently routing to false.
func TestFieldConditionEvaluatorErrors(t *testing.T) {
	e := NewFieldConditionEvaluator()
	ctx := context.Background()
	ec := &approval.EvaluationContext{
		FormData: approval.FormData{
			"amount": 5000,
			"name":   "alice",
		},
	}

	tests := []struct {
		name     string
		cond     approval.Condition
		sentinel error
	}{
		{"UnknownOperator", approval.Condition{Subject: "name", Operator: "nope", Value: "x"}, ErrUnsupportedOperator},
		{"OrderedNumberVsString", approval.Condition{Subject: "amount", Operator: "gt", Value: "high"}, ErrIncomparableValues},
		{"OrderedNilSubject", approval.Condition{Subject: "nonexistent", Operator: "lt", Value: 10}, ErrIncomparableValues},
		{"InWithoutList", approval.Condition{Subject: "name", Operator: "in", Value: "alice"}, ErrIncomparableValues},
		{"ContainsNumberSubject", approval.Condition{Subject: "amount", Operator: "contains", Value: "5"}, ErrIncomparableValues},
		{"StartsWithNumberSubject", approval.Condition{Subject: "amount", Operator: "starts_with", Value: "5"}, ErrIncomparableValues},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Evaluate(ctx, tt.cond, ec)
			require.ErrorIs(t, err, tt.sentinel, "Should reject with the expected sentinel")
		})
	}
}

// TestFieldConditionEvaluatorEmptyCollections tests is_empty with empty collections.
func TestFieldConditionEvaluatorEmptyCollections(t *testing.T) {
	e := NewFieldConditionEvaluator()
	ctx := context.Background()
	ec := &approval.EvaluationContext{
		FormData: approval.FormData{
			"empty_arr":     []string{},
			"empty_any_arr": []any{},
			"empty_map":     map[string]any{},
		},
	}

	tests := []struct {
		name    string
		subject string
	}{
		{"EmptyStringArray", "empty_arr"},
		{"EmptyAnyArray", "empty_any_arr"},
		{"EmptyMap", "empty_map"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, approval.Condition{Subject: tt.subject, Operator: "is_empty"}, ec)
			require.NoError(t, err, "Should evaluate without error")
			assert.True(t, result, "Should detect empty collection")
		})
	}
}

// TestFieldConditionEvaluatorGlobals tests subject resolution against
// host-supplied globals: a global resolves like any subject, shadows a
// same-named form field (mirroring the built-in applicant subjects), and an
// absent global falls through to form data.
func TestFieldConditionEvaluatorGlobals(t *testing.T) {
	e := NewFieldConditionEvaluator()
	ctx := context.Background()
	ec := &approval.EvaluationContext{
		FormData: approval.FormData{
			"amount":     5000,
			"quotaLimit": 100, // shadowed by the global below
		},
		ApplicantID: "user1",
		Globals: map[string]any{
			"quotaLimit":     8000,
			"applicantRoles": []string{"manager", "finance"},
		},
	}

	tests := []struct {
		name     string
		cond     approval.Condition
		expected bool
	}{
		{"GlobalResolves", approval.Condition{Kind: approval.ConditionField, Subject: "quotaLimit", Operator: "gte", Value: 8000}, true},
		{"GlobalShadowsFormField", approval.Condition{Kind: approval.ConditionField, Subject: "quotaLimit", Operator: "eq", Value: 100}, false},
		{"GlobalListContains", approval.Condition{Kind: approval.ConditionField, Subject: "applicantRoles", Operator: "contains", Value: "finance"}, true},
		{"AbsentGlobalFallsThroughToForm", approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: "eq", Value: 5000}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, tt.cond, ec)
			require.NoError(t, err, "Should evaluate without error")
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExpressionConditionEvaluator exercises the expression path through the
// framework expression.Engine (expr-lang backend).
func TestExpressionConditionEvaluator(t *testing.T) {
	e := NewExpressionConditionEvaluator(exprlang.New())
	assert.Equal(t, approval.ConditionExpression, e.Kind(), "Should return ConditionExpression type")

	ctx := context.Background()

	t.Run("SimpleComparison", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:              approval.FormData{"amount": 5000},
			ApplicantID:           "user1",
			ApplicantDepartmentID: new("dept1"),
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: "formData.amount > 3000"}, ec)
		require.NoError(t, err, "Should evaluate greater-than expression")
		assert.True(t, result, "Should be true for 5000 > 3000")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: "formData.amount > 10000"}, ec)
		require.NoError(t, err, "Should evaluate greater-than expression")
		assert.False(t, result, "Should be false for 5000 > 10000")
	})

	t.Run("LogicalCombination", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:              approval.FormData{"amount": 5000, "department": "sales"},
			ApplicantID:           "user1",
			ApplicantDepartmentID: new("dept1"),
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: `formData.amount > 1000 and formData.department == "sales"`}, ec)
		require.NoError(t, err, "Should evaluate AND expression")
		assert.True(t, result, "Should be true when both conditions match")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: `formData.amount > 1000 and formData.department == "hr"`}, ec)
		require.NoError(t, err, "Should evaluate AND expression")
		assert.False(t, result, "Should be false when department does not match")
	})

	t.Run("BuiltInVariables", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:              approval.FormData{},
			ApplicantID:           "user1",
			ApplicantDepartmentID: new("dept_sales"),
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: `applicantId == "user1"`}, ec)
		require.NoError(t, err, "Should evaluate applicantId expression")
		assert.True(t, result, "Should match applicant ID")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: `applicantDepartmentId == "dept_sales"`}, ec)
		require.NoError(t, err, "Should evaluate applicantDepartmentId expression")
		assert.True(t, result, "Should match applicant dept ID")
	})

	t.Run("NilDepartmentID", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:    approval.FormData{},
			ApplicantID: "user1",
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: `applicantDepartmentId == ""`}, ec)
		require.NoError(t, err, "Should evaluate with nil department ID")
		assert.True(t, result, "Should resolve nil department ID as empty string")
	})

	t.Run("SyntaxError", func(t *testing.T) {
		ec := &approval.EvaluationContext{FormData: approval.FormData{}}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: "invalid @@@ syntax"}, ec)
		require.Error(t, err, "Should fail for invalid syntax")
		assert.Contains(t, err.Error(), "evaluate condition expression", "Should wrap the engine error")
	})

	t.Run("EmptyExpression", func(t *testing.T) {
		ec := &approval.EvaluationContext{FormData: approval.FormData{}}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: ""}, ec)
		require.Error(t, err, "Should fail for empty expression")
	})

	t.Run("NonBoolResult", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:    approval.FormData{"amount": 5000},
			ApplicantID: "user1",
		}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: "formData.amount"}, ec)
		require.ErrorIs(t, err, ErrExpressionReturnedNonBool, "Numeric result should be rejected as non-bool")
	})

	t.Run("Globals", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:    approval.FormData{"amount": 5000},
			ApplicantID: "user1",
			Globals: map[string]any{
				"quotaLimit":  8000,
				"applicantId": "forged", // must lose to the built-in binding
			},
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: "formData.amount < quotaLimit"}, ec)
		require.NoError(t, err, "Should evaluate a global binding")
		assert.True(t, result, "Should read the host-supplied global")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: `applicantId == "user1"`}, ec)
		require.NoError(t, err, "Should evaluate the built-in binding")
		assert.True(t, result, "Built-in bindings must win a collision with a global")
	})
}

func TestAggregateConditions(t *testing.T) {
	e := NewFieldConditionEvaluator(NewSumAggregator(), NewCountAggregator(), NewAvgAggregator())
	ec := &approval.EvaluationContext{FormData: approval.NewFormData(map[string]any{
		"items": []any{
			map[string]any{"amount": 100, "note": "a"},
			map[string]any{"amount": 250.5},
			map[string]any{"amount": nil},
		},
		"empty":  []any{},
		"broken": "not-a-list",
		"badRow": []any{map[string]any{"amount": "NaN"}},
	})}

	cond := func(agg approval.AggregateKind, column string, op approval.ConditionOperator, value any) approval.Condition {
		return approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: agg, Column: column, Operator: op, Value: value}
	}

	t.Run("SumComparesTotal", func(t *testing.T) {
		matched, err := e.Evaluate(context.Background(), cond(approval.AggregateSum, "amount", approval.OperatorGreater, 300), ec)
		require.NoError(t, err, "sum should evaluate")
		assert.True(t, matched, "350.5 > 300 should match")

		matched, err = e.Evaluate(context.Background(), cond(approval.AggregateSum, "amount", approval.OperatorGreater, 400), ec)
		require.NoError(t, err, "sum should evaluate")
		assert.False(t, matched, "350.5 > 400 should not match")
	})

	t.Run("CountFoldsRowsIncludingNilCells", func(t *testing.T) {
		matched, err := e.Evaluate(context.Background(), cond(approval.AggregateCount, "", approval.OperatorEquals, 3), ec)
		require.NoError(t, err, "count should evaluate")
		assert.True(t, matched, "count folds rows, not non-nil cells")
	})

	t.Run("AvgSkipsNilCells", func(t *testing.T) {
		matched, err := e.Evaluate(context.Background(), cond(approval.AggregateAvg, "amount", approval.OperatorGreaterOrEq, 175.25), ec)
		require.NoError(t, err, "avg should evaluate")
		assert.True(t, matched, "avg over the two non-nil amounts is 175.25")
	})

	t.Run("MissingTableFoldsAsEmpty", func(t *testing.T) {
		missing := approval.Condition{Kind: approval.ConditionField, Subject: "absent", Aggregate: approval.AggregateSum, Column: "amount", Operator: approval.OperatorEquals, Value: 0}
		matched, err := e.Evaluate(context.Background(), missing, ec)
		require.NoError(t, err, "a missing table folds as zero rows")
		assert.True(t, matched, "sum over an empty table is 0")

		count := approval.Condition{Kind: approval.ConditionField, Subject: "empty", Aggregate: approval.AggregateCount, Operator: approval.OperatorEquals, Value: 0}
		matched, err = e.Evaluate(context.Background(), count, ec)
		require.NoError(t, err, "count over an empty table should evaluate")
		assert.True(t, matched, "count of an empty table is 0")
	})

	t.Run("AvgOverEmptyMatchesNothing", func(t *testing.T) {
		empty := approval.Condition{Kind: approval.ConditionField, Subject: "empty", Aggregate: approval.AggregateAvg, Column: "amount", Operator: approval.OperatorLessOrEq, Value: 1e18}
		matched, err := e.Evaluate(context.Background(), empty, ec)
		require.NoError(t, err, "avg over an empty table is not an error")
		assert.False(t, matched, "avg over zero rows has no value and must match nothing — SQL NULL semantics")
	})

	t.Run("NonListValueFailsLoudly", func(t *testing.T) {
		bad := approval.Condition{Kind: approval.ConditionField, Subject: "broken", Aggregate: approval.AggregateCount, Operator: approval.OperatorEquals, Value: 0}
		_, err := e.Evaluate(context.Background(), bad, ec)
		require.Error(t, err, "a non-list table value must fail the evaluation, not silently mismatch")
	})

	t.Run("NonNumericCellFailsLoudly", func(t *testing.T) {
		bad := approval.Condition{Kind: approval.ConditionField, Subject: "badRow", Aggregate: approval.AggregateSum, Column: "amount", Operator: approval.OperatorEquals, Value: 0}
		_, err := e.Evaluate(context.Background(), bad, ec)
		require.Error(t, err, "a present non-numeric cell must fail the evaluation")
	})

	t.Run("UnknownAggregatorFailsLoudly", func(t *testing.T) {
		bare := NewFieldConditionEvaluator()
		_, err := bare.Evaluate(context.Background(), cond(approval.AggregateSum, "amount", approval.OperatorEquals, 0), ec)
		require.ErrorIs(t, err, ErrAggregatorNotFound, "an unregistered aggregate kind must fail loudly")
	})
}

func TestAggregateIgnoresCollidingGlobal(t *testing.T) {
	e := NewFieldConditionEvaluator(NewCountAggregator())
	ec := &approval.EvaluationContext{
		// A host global colliding with the table key shadows SCALAR subjects,
		// but aggregates fold form data by contract — globals are scalars.
		Globals:  map[string]any{"items": "shadow"},
		FormData: approval.NewFormData(map[string]any{"items": []any{map[string]any{"qty": 1}}}),
	}

	cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateCount, Operator: approval.OperatorEquals, Value: 1}
	matched, err := e.Evaluate(context.Background(), cond, ec)
	require.NoError(t, err, "the colliding global must not shadow the aggregate's table read")
	assert.True(t, matched, "count folds the form table, not the global")
}
