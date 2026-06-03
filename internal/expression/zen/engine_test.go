package zen_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
	"github.com/coldsmirk/vef-framework-go/internal/expression/zen"
)

func TestEngine(t *testing.T) {
	eng := zen.New()
	ctx := context.Background()

	t.Run("EvaluateArithmetic", func(t *testing.T) {
		got, err := expression.EvaluateAs[int](ctx, eng, "a + b", map[string]any{"a": 1, "b": 2})
		require.NoError(t, err, "Evaluate should succeed")
		assert.Equal(t, 3, got, "Sum a + b should be 3")
	})

	t.Run("EvaluateStructEnv", func(t *testing.T) {
		env := struct {
			Price float64 `json:"price"`
			Qty   float64 `json:"qty"`
		}{Price: 2, Qty: 3}

		got, err := expression.EvaluateAs[float64](ctx, eng, "price * qty", env)
		require.NoError(t, err, "Evaluate against a struct env should succeed")
		assert.Equal(t, float64(6), got, "Product price * qty should be 6")
	})

	t.Run("BooleanValue", func(t *testing.T) {
		value, err := eng.Evaluate(ctx, "a > b", map[string]any{"a": 5, "b": 1})
		require.NoError(t, err, "Boolean expression should evaluate")

		got, err := value.Bool()
		require.NoError(t, err, "Result should be boolean")
		assert.True(t, got, "Comparison 5 > 1 should be true")
	})

	t.Run("Predicate", func(t *testing.T) {
		ok, err := expression.Match(ctx, eng, ">= 5", map[string]any{"$": 10})
		require.NoError(t, err, "Unary predicate should evaluate")
		assert.True(t, ok, "Predicate 10 >= 5 should be true")

		ok, err = expression.Match(ctx, eng, ">= 5", map[string]any{"$": 1})
		require.NoError(t, err, "Unary predicate should evaluate")
		assert.False(t, ok, "Predicate 1 >= 5 should be false")
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

	t.Run("CompileDefersMalformedErrorToRun", func(t *testing.T) {
		program, err := eng.Compile("a +")
		require.NoError(t, err, "Compile must defer parse errors and report no error")
		require.NotNil(t, program, "Compile must return a usable program even for malformed source")
		assert.Equal(t, "a +", program.Source(), "Source should return the original malformed expression")

		_, err = program.Run(ctx, nil)
		require.Error(t, err, "Run should surface the deferred parse error")
		assert.ErrorIs(t, err, expression.ErrEvaluationFailed, "Run error should wrap ErrEvaluationFailed")
	})

	t.Run("CompilePredicate", func(t *testing.T) {
		program, err := eng.Compile(">= 5", expression.AsPredicate())
		require.NoError(t, err, "Compile as predicate should succeed")

		hit, err := program.Run(ctx, map[string]any{"$": 10})
		require.NoError(t, err, "Predicate run should succeed")
		ok, err := hit.Bool()
		require.NoError(t, err, "Predicate result should be boolean")
		assert.True(t, ok, "Predicate 10 >= 5 should be true")

		miss, err := program.Run(ctx, map[string]any{"$": 1})
		require.NoError(t, err, "Predicate run should succeed")
		ok, err = miss.Bool()
		require.NoError(t, err, "Predicate result should be boolean")
		assert.False(t, ok, "Predicate 1 >= 5 should be false")
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
