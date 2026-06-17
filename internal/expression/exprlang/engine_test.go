package exprlang_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
	"github.com/coldsmirk/vef-framework-go/internal/expression/exprlang"
)

func TestEngine(t *testing.T) {
	eng := exprlang.New()
	ctx := context.Background()

	t.Run("EvaluateArithmetic", func(t *testing.T) {
		got, err := expression.EvaluateAs[int](ctx, eng, "a + b", map[string]any{"a": 1, "b": 2})
		require.NoError(t, err, "Evaluate should succeed")
		assert.Equal(t, 3, got, "Sum a + b should be 3")
	})

	t.Run("EvaluateStructEnv", func(t *testing.T) {
		// A struct env is reached by its serialized (json) field names, so the
		// expression matches the derived-field convention the transformer relies on.
		env := struct {
			Price float64 `json:"price"`
			Qty   float64 `json:"qty"`
		}{Price: 2, Qty: 3}

		got, err := expression.EvaluateAs[float64](ctx, eng, "price * qty", env)
		require.NoError(t, err, "Evaluate against a struct env should succeed")
		assert.Equal(t, float64(6), got, "Product price * qty should be 6")
	})

	t.Run("NestedAccessAndLogicalKeywords", func(t *testing.T) {
		// Mirrors the approval expression-condition syntax: nested member access on
		// a map plus the and/or/not keyword operators.
		env := map[string]any{
			"formData": map[string]any{"amount": 5000, "department": "sales"},
		}

		ok, err := expression.EvaluateAs[bool](ctx, eng, `formData.amount > 1000 and formData.department == "sales"`, env)
		require.NoError(t, err, "Nested-access logical expression should evaluate")
		assert.True(t, ok, "5000 > 1000 and department == sales should be true")

		ok, err = expression.EvaluateAs[bool](ctx, eng, `not (formData.amount > 1000) or formData.department == "hr"`, env)
		require.NoError(t, err, "not/or expression should evaluate")
		assert.False(t, ok, "neither disjunct holds, so the result should be false")
	})

	t.Run("BooleanValue", func(t *testing.T) {
		value, err := eng.Evaluate(ctx, "a > b", map[string]any{"a": 5, "b": 1})
		require.NoError(t, err, "Boolean expression should evaluate")

		got, err := value.Bool()
		require.NoError(t, err, "Result should be boolean")
		assert.True(t, got, "Comparison 5 > 1 should be true")
	})

	t.Run("UndefinedVariableIsNil", func(t *testing.T) {
		value, err := eng.Evaluate(ctx, "missing == nil", map[string]any{})
		require.NoError(t, err, "Referencing an undefined variable should not error")

		got, err := value.Bool()
		require.NoError(t, err, "Result should be boolean")
		assert.True(t, got, "An undefined variable should resolve to nil")
	})

	t.Run("AbsentFieldOnPresentMapIsNil", func(t *testing.T) {
		// The realistic approval-condition shape: formData is always present but a
		// referenced field may be absent. Member access on a present map must stay
		// nil-safe (unlike access through a wholly undefined root, which errors).
		value, err := eng.Evaluate(ctx, "formData.amount == nil", map[string]any{"formData": map[string]any{}})
		require.NoError(t, err, "Accessing an absent field on a present map should not error")

		got, err := value.Bool()
		require.NoError(t, err, "Result should be boolean")
		assert.True(t, got, "An absent field should resolve to nil")
	})

	t.Run("Predicate", func(t *testing.T) {
		ok, err := expression.Match(ctx, eng, "score >= 5", map[string]any{"score": 10})
		require.NoError(t, err, "Predicate should evaluate")
		assert.True(t, ok, "Predicate 10 >= 5 should be true")

		ok, err = expression.Match(ctx, eng, "score >= 5", map[string]any{"score": 1})
		require.NoError(t, err, "Predicate should evaluate")
		assert.False(t, ok, "Predicate 1 >= 5 should be false")
	})

	t.Run("CompilePredicateRejectsNonBool", func(t *testing.T) {
		// AsPredicate compiles with a boolean expectation, so a non-boolean
		// expression fails eagerly at compile time.
		_, err := eng.Compile("1 + 1", expression.AsPredicate())
		require.Error(t, err, "A non-boolean predicate should fail to compile")
		assert.ErrorIs(t, err, expression.ErrEvaluationFailed, "Compile error should wrap ErrEvaluationFailed")
	})

	t.Run("CompileReuse", func(t *testing.T) {
		program, err := eng.Compile("x * 2")
		require.NoError(t, err, "Compile should succeed")
		assert.Equal(t, "x * 2", program.Source(), "Source should return the expression")

		first, err := program.Run(ctx, map[string]any{"x": 3})
		require.NoError(t, err, "First run should succeed")
		n1, err := expression.DecodeValue[int](first)
		require.NoError(t, err, "First result should decode")
		assert.Equal(t, 6, n1, "Product 3 * 2 should be 6")

		second, err := program.Run(ctx, map[string]any{"x": 5})
		require.NoError(t, err, "Second run should succeed")
		n2, err := expression.DecodeValue[int](second)
		require.NoError(t, err, "Second result should decode")
		assert.Equal(t, 10, n2, "Product 5 * 2 should be 10")
	})

	t.Run("EvaluationError", func(t *testing.T) {
		_, err := eng.Evaluate(ctx, "a +", nil)
		require.Error(t, err, "An invalid expression should error")
		assert.ErrorIs(t, err, expression.ErrEvaluationFailed, "Error should wrap ErrEvaluationFailed")
	})

	t.Run("CompileReportsMalformedEagerly", func(t *testing.T) {
		program, err := eng.Compile("a +")
		require.Error(t, err, "Compile should report the parse error eagerly")
		assert.Nil(t, program, "No program should be returned for malformed source")
		assert.ErrorIs(t, err, expression.ErrEvaluationFailed, "Compile error should wrap ErrEvaluationFailed")
	})

	t.Run("CanceledContext", func(t *testing.T) {
		canceled, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := eng.Evaluate(canceled, "a + b", map[string]any{"a": 1, "b": 2})
		require.Error(t, err, "Evaluate should honor an already-canceled context")
		assert.ErrorIs(t, err, context.Canceled, "Evaluate should return the raw context error")
		assert.NotErrorIs(t, err, expression.ErrEvaluationFailed, "Cancellation must not be wrapped as an evaluation failure")

		program, err := eng.Compile("a + b")
		require.NoError(t, err, "Compile should succeed")

		_, err = program.Run(canceled, map[string]any{"a": 1, "b": 2})
		require.Error(t, err, "Run should honor an already-canceled context")
		assert.ErrorIs(t, err, context.Canceled, "Run should return the raw context error")
		assert.NotErrorIs(t, err, expression.ErrEvaluationFailed, "Cancellation must not be wrapped as an evaluation failure")
	})
}
