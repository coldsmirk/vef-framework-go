package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
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
		},
		ApplicantID:     "user1",
		ApplicantDeptID: new("dept1"),
	}

	tests := []struct {
		name     string
		cond     approval.Condition
		expected bool
	}{
		// eq / ne
		{"EqStringMatch", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "eq", Value: "alice"}, true},
		{"EqStringNoMatch", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "eq", Value: "bob"}, false},
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

		// in / not_in
		{"InStringArray", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{"alice", "bob"}}, true},
		{"InStringArrayNotFound", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{"bob", "charlie"}}, false},
		{"InEmptySlice", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{}}, false},
		{"NotInStringArray", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{"bob", "charlie"}}, true},
		{"NotInStringArrayFound", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{"alice", "bob"}}, false},
		{"NotInEmptySlice", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{}}, true},
		{"InWithAnySlice", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "in", Value: []any{"alice", "charlie"}}, true},

		// contains / not_contains
		{"ContainsTrue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: "world"}, true},
		{"ContainsFalse", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: "mars"}, false},
		{"ContainsEmptyValue", approval.Condition{Kind: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: ""}, true},
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
		{"IsNotEmptyString", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "is_not_empty"}, true},
		{"IsNotEmptyNil", approval.Condition{Kind: approval.ConditionField, Subject: "nonexistent", Operator: "is_not_empty"}, false},

		// Special subjects
		{"ApplicantSubject", approval.Condition{Kind: approval.ConditionField, Subject: "applicantId", Operator: "eq", Value: "user1"}, true},
		{"DeptSubject", approval.Condition{Kind: approval.ConditionField, Subject: "applicantDeptId", Operator: "eq", Value: "dept1"}, true},

		// Unknown operator
		{"UnknownOperator", approval.Condition{Kind: approval.ConditionField, Subject: "name", Operator: "unknown_op", Value: "x"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, tt.cond, ec)
			require.NoError(t, err, "Should evaluate without error")
			assert.Equal(t, tt.expected, result, "Should return expected result")
		})
	}
}

// TestFieldConditionEvaluatorIsEmptyOnNonStringField tests is_empty/is_not_empty on non-string fields.
func TestFieldConditionEvaluatorIsEmptyOnNonStringField(t *testing.T) {
	e := NewFieldConditionEvaluator()
	ctx := context.Background()
	ec := &approval.EvaluationContext{
		FormData: approval.FormData{"amount": 5000},
	}

	tests := []struct {
		name     string
		operator string
	}{
		{"IsEmptyOnInt", "is_empty"},
		{"IsNotEmptyOnInt", "is_not_empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Evaluate(ctx, approval.Condition{Subject: "amount", Operator: tt.operator}, ec)
			require.Error(t, err, "Should fail for non-string/collection field")
			assert.Contains(t, err.Error(), "run expression", "Should wrap runtime error")
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

// TestExpressionConditionEvaluator tests expression condition evaluator scenarios.
func TestExpressionConditionEvaluator(t *testing.T) {
	e := NewExpressionConditionEvaluator()
	assert.Equal(t, approval.ConditionExpression, e.Kind(), "Should return ConditionExpression type")

	ctx := context.Background()

	t.Run("SimpleComparison", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:        approval.FormData{"amount": 5000},
			ApplicantID:     "user1",
			ApplicantDeptID: new("dept1"),
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
			FormData:        approval.FormData{"amount": 5000, "department": "sales"},
			ApplicantID:     "user1",
			ApplicantDeptID: new("dept1"),
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: `formData.amount > 1000 && formData.department == "sales"`}, ec)
		require.NoError(t, err, "Should evaluate AND expression")
		assert.True(t, result, "Should be true when both conditions match")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: `formData.amount > 1000 && formData.department == "hr"`}, ec)
		require.NoError(t, err, "Should evaluate AND expression")
		assert.False(t, result, "Should be false when department does not match")
	})

	t.Run("BuiltInVariables", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:        approval.FormData{},
			ApplicantID:     "user1",
			ApplicantDeptID: new("dept_sales"),
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: `applicantId == "user1"`}, ec)
		require.NoError(t, err, "Should evaluate applicantId expression")
		assert.True(t, result, "Should match applicant ID")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: `applicantDeptId == "dept_sales"`}, ec)
		require.NoError(t, err, "Should evaluate applicantDeptId expression")
		assert.True(t, result, "Should match applicant dept ID")
	})

	t.Run("SyntaxError", func(t *testing.T) {
		ec := &approval.EvaluationContext{FormData: approval.FormData{}}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: "invalid @@@ syntax"}, ec)
		require.Error(t, err, "Should fail for invalid syntax")
		assert.Contains(t, err.Error(), "compile expression", "Should wrap compile error")
	})

	t.Run("EmptyExpression", func(t *testing.T) {
		ec := &approval.EvaluationContext{FormData: approval.FormData{}}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: ""}, ec)
		require.Error(t, err, "Should fail for empty expression")
	})

	t.Run("RuntimeError", func(t *testing.T) {
		ec := &approval.EvaluationContext{
			FormData:        approval.FormData{"amount": "not_a_number"},
			ApplicantID:     "user1",
			ApplicantDeptID: new("dept1"),
		}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: "formData.amount > 100"}, ec)
		require.Error(t, err, "Should fail when expression evaluation fails at runtime")
	})
}

// TestBuildFieldExpression tests expression generation from structured conditions.
func TestBuildFieldExpression(t *testing.T) {
	tests := []struct {
		name     string
		cond     approval.Condition
		expected string
	}{
		{"Eq", approval.Condition{Subject: "name", Operator: "eq", Value: "alice"}, `formData["name"] == "alice"`},
		{"Ne", approval.Condition{Subject: "name", Operator: "ne", Value: "bob"}, `formData["name"] != "bob"`},
		{"Gt", approval.Condition{Subject: "amount", Operator: "gt", Value: 100}, `formData["amount"] > 100`},
		{"Gte", approval.Condition{Subject: "amount", Operator: "gte", Value: 100}, `formData["amount"] >= 100`},
		{"Lt", approval.Condition{Subject: "amount", Operator: "lt", Value: 100}, `formData["amount"] < 100`},
		{"Lte", approval.Condition{Subject: "amount", Operator: "lte", Value: 100}, `formData["amount"] <= 100`},
		{"In", approval.Condition{Subject: "name", Operator: "in", Value: []string{"a", "b"}}, `formData["name"] in ["a", "b"]`},
		{"NotIn", approval.Condition{Subject: "name", Operator: "not_in", Value: []string{"a"}}, `not (formData["name"] in ["a"])`},
		{"Contains", approval.Condition{Subject: "name", Operator: "contains", Value: "li"}, `formData["name"] contains "li"`},
		{"NotContains", approval.Condition{Subject: "name", Operator: "not_contains", Value: "x"}, `not (formData["name"] contains "x")`},
		{"StartsWith", approval.Condition{Subject: "name", Operator: "starts_with", Value: "al"}, `formData["name"] startsWith "al"`},
		{"EndsWith", approval.Condition{Subject: "name", Operator: "ends_with", Value: "ce"}, `formData["name"] endsWith "ce"`},
		{"IsEmpty", approval.Condition{Subject: "field", Operator: "is_empty"}, `len(formData["field"] ?? "") == 0`},
		{"IsNotEmpty", approval.Condition{Subject: "field", Operator: "is_not_empty"}, `len(formData["field"] ?? "") > 0`},
		{"ApplicantSubject", approval.Condition{Subject: "applicantId", Operator: "eq", Value: "u1"}, `applicantId == "u1"`},
		{"DeptSubject", approval.Condition{Subject: "applicantDeptId", Operator: "eq", Value: "d1"}, `applicantDeptId == "d1"`},
		{"Unknown", approval.Condition{Subject: "x", Operator: "nope"}, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, buildFieldExpression(tt.cond), "Should generate expected expression")
		})
	}
}

// TestFormatExprValue tests value formatting for expr-lang literals.
func TestFormatExprValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"Nil", nil, "nil"},
		{"String", "hello", `"hello"`},
		{"StringWithQuotes", `say "hi"`, `"say \"hi\""`},
		{"Int", 42, "42"},
		{"Float", 3.14, "3.14"},
		{"Bool", true, "true"},
		{"StringSlice", []string{"a", "b"}, `["a", "b"]`},
		{"AnySlice", []any{"x", 1}, `["x", 1]`},
		{"EmptySlice", []string{}, "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatExprValue(tt.input), "Should format value as expected")
		})
	}
}
