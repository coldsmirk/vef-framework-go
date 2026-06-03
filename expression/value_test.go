package expression_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
)

func TestValue(t *testing.T) {
	t.Run("Bool", func(t *testing.T) {
		got, err := expression.NewValue(true).Bool()
		require.NoError(t, err, "Bool should succeed for a boolean value")
		assert.True(t, got, "Bool should return the wrapped boolean")

		got, err = expression.NewValue(false).Bool()
		require.NoError(t, err, "Bool should succeed for false")
		assert.False(t, got, "Bool should return false")
	})

	t.Run("BoolWrongType", func(t *testing.T) {
		_, err := expression.NewValue("nope").Bool()
		require.Error(t, err, "Bool should fail for a non-boolean value")
		assert.ErrorIs(t, err, expression.ErrUnexpectedType, "Error should be ErrUnexpectedType")
	})

	t.Run("DecodeScalar", func(t *testing.T) {
		// Numbers arrive as float64 (JSON) and decode into the requested type.
		n, err := expression.DecodeValue[int](expression.NewValue(float64(42)))
		require.NoError(t, err, "DecodeValue[int] should succeed")
		assert.Equal(t, 42, n, "Should decode 42")

		s, err := expression.DecodeValue[string](expression.NewValue("hello"))
		require.NoError(t, err, "DecodeValue[string] should succeed")
		assert.Equal(t, "hello", s, "Should decode string")
	})

	t.Run("DecodeStruct", func(t *testing.T) {
		type point struct {
			X int `json:"x"`
			Y int `json:"y"`
		}

		raw := map[string]any{"x": float64(1), "y": float64(2)}
		got, err := expression.DecodeValue[point](expression.NewValue(raw))
		require.NoError(t, err, "DecodeValue[point] should succeed")
		assert.Equal(t, point{X: 1, Y: 2}, got, "Should decode into struct")
	})

	t.Run("DecodeError", func(t *testing.T) {
		_, err := expression.DecodeValue[int](expression.NewValue("not-a-number"))
		require.Error(t, err, "Decoding a string into int should fail")
	})

	t.Run("DecodeNil", func(t *testing.T) {
		// A null/absent result (IsNil) decodes through JSON as the zero value.
		got, err := expression.DecodeValue[int](expression.NewValue(nil))
		require.NoError(t, err, "Decoding a nil value should succeed")
		assert.Equal(t, 0, got, "Nil should decode to the zero value")
	})

	t.Run("IsNil", func(t *testing.T) {
		assert.True(t, expression.NewValue(nil).IsNil(), "Nil value should report IsNil")
		assert.False(t, expression.NewValue(0).IsNil(), "Zero is not nil")
	})

	t.Run("Interface", func(t *testing.T) {
		assert.Equal(t, "x", expression.NewValue("x").Interface(), "Interface returns the raw value")
		assert.Nil(t, expression.NewValue(nil).Interface(), "Interface should return nil for a nil value")
	})
}
