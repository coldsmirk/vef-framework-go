package zen_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
	"github.com/coldsmirk/vef-framework-go/expression/zen"
)

func TestEngine(t *testing.T) {
	eng := zen.New()
	ctx := context.Background()

	t.Run("EvaluateArithmetic", func(t *testing.T) {
		got, err := expression.EvaluateAs[int](ctx, eng, "a + b", map[string]any{"a": 1, "b": 2})
		require.NoError(t, err, "evaluate should succeed")
		assert.Equal(t, 3, got, "a + b should be 3")
	})

	t.Run("EvaluateStructEnv", func(t *testing.T) {
		env := struct {
			Price float64 `json:"price"`
			Qty   float64 `json:"qty"`
		}{Price: 2, Qty: 3}

		got, err := expression.EvaluateAs[float64](ctx, eng, "price * qty", env)
		require.NoError(t, err, "evaluate against a struct env should succeed")
		assert.Equal(t, float64(6), got, "price * qty should be 6")
	})

	t.Run("BooleanValue", func(t *testing.T) {
		value, err := eng.Evaluate(ctx, "a > b", map[string]any{"a": 5, "b": 1})
		require.NoError(t, err, "boolean expression should evaluate")

		got, err := value.Bool()
		require.NoError(t, err, "result should be boolean")
		assert.True(t, got, "5 > 1 should be true")
	})

	t.Run("Predicate", func(t *testing.T) {
		ok, err := expression.Match(ctx, eng, ">= 5", map[string]any{"$": 10})
		require.NoError(t, err, "unary predicate should evaluate")
		assert.True(t, ok, "10 >= 5 should be true")

		ok, err = expression.Match(ctx, eng, ">= 5", map[string]any{"$": 1})
		require.NoError(t, err, "unary predicate should evaluate")
		assert.False(t, ok, "1 >= 5 should be false")
	})

	t.Run("CompileReuse", func(t *testing.T) {
		program, err := eng.Compile("x * 2")
		require.NoError(t, err, "compile should succeed")
		assert.Equal(t, "x * 2", program.Source(), "Source should return the expression")

		first, err := program.Run(ctx, map[string]any{"x": 3})
		require.NoError(t, err, "first run should succeed")
		n1, err := expression.DecodeValue[int](first)
		require.NoError(t, err, "decode first result")
		assert.Equal(t, 6, n1, "3 * 2 should be 6")

		second, err := program.Run(ctx, map[string]any{"x": 5})
		require.NoError(t, err, "second run should succeed")
		n2, err := expression.DecodeValue[int](second)
		require.NoError(t, err, "decode second result")
		assert.Equal(t, 10, n2, "5 * 2 should be 10")
	})

	t.Run("EvaluationError", func(t *testing.T) {
		_, err := eng.Evaluate(ctx, "a +", nil)
		require.Error(t, err, "an invalid expression should error")
		assert.ErrorIs(t, err, expression.ErrEvaluationFailed, "error should wrap ErrEvaluationFailed")
	})
}
