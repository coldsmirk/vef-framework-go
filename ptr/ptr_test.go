package ptr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOf tests conditional pointer creation based on zero value.
func TestOf(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNil  bool
		expected string
	}{
		{"EmptyString", "", true, ""},
		{"NonEmptyString", "hello", false, "hello"},
		{"Whitespace", " ", false, " "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Of(tt.input)
			if tt.wantNil {
				assert.Nil(t, p, "Should return nil for zero value")
			} else {
				require.NotNil(t, p, "Should return non-nil for non-zero value")
				assert.Equal(t, tt.expected, *p, "Should point to the input value")
			}
		})
	}
}

// TestOfInt tests Of with int type.
func TestOfInt(t *testing.T) {
	assert.Nil(t, Of(0), "Should return nil for zero int")

	p := Of(42)
	require.NotNil(t, p, "Should return non-nil for non-zero int")
	assert.Equal(t, 42, *p, "Should point to the int value")
}

// TestOfBool tests Of with bool type.
func TestOfBool(t *testing.T) {
	assert.Nil(t, Of(false), "Should return nil for false")

	p := Of(true)
	require.NotNil(t, p, "Should return non-nil for true")
	assert.True(t, *p, "Should point to true")
}

// TestZero tests zero value generation for various types.
func TestZero(t *testing.T) {
	assert.Equal(t, 0, Zero[int](), "Should return zero for int")
	assert.Equal(t, "", Zero[string](), "Should return empty string for string")
	assert.False(t, Zero[bool](), "Should return false for bool")
	assert.Equal(t, 0.0, Zero[float64](), "Should return 0.0 for float64")
	assert.Nil(t, Zero[*int](), "Should return nil for pointer type")
	assert.Nil(t, Zero[[]int](), "Should return nil for slice type")
	assert.Nil(t, Zero[map[string]int](), "Should return nil for map type")
}

// TestZeroStruct tests Zero with struct type.
func TestZeroStruct(t *testing.T) {
	type Pair struct {
		Key   string
		Value int
	}

	z := Zero[Pair]()
	assert.Equal(t, "", z.Key, "Should return zero Key")
	assert.Equal(t, 0, z.Value, "Should return zero Value")
}

// TestValue tests pointer dereferencing with fallbacks.
func TestValue(t *testing.T) {
	t.Run("NonNilPointer", func(t *testing.T) {
		assert.Equal(t, 42, Value(new(42)), "Should return the pointed-to value")
	})

	t.Run("NilNoFallback", func(t *testing.T) {
		assert.Equal(t, 0, Value[int](nil), "Should return zero value when nil without fallback")
	})

	t.Run("NilWithFallback", func(t *testing.T) {
		assert.Equal(t, 10, Value(nil, new(10)), "Should return first non-nil fallback value")
	})

	t.Run("NilWithMultipleFallbacks", func(t *testing.T) {
		assert.Equal(t, 20, Value(nil, nil, new(20), new(30)), "Should return first non-nil fallback")
	})

	t.Run("NilWithAllNilFallbacks", func(t *testing.T) {
		assert.Equal(t, 0, Value[int](nil, nil, nil), "Should return zero when all fallbacks are nil")
	})

	t.Run("NonNilIgnoresFallbacks", func(t *testing.T) {
		assert.Equal(t, 1, Value(new(1), new(2)), "Should return primary value, ignoring fallbacks")
	})

	t.Run("StringType", func(t *testing.T) {
		assert.Equal(t, "fallback", Value(nil, new("fallback")), "Should work with string type")
	})

	t.Run("EmptyStringFallback", func(t *testing.T) {
		assert.Equal(t, "", Value(nil, new("")), "Should return empty string from non-nil fallback")
	})
}

// TestValueOrElse tests lazy fallback dereferencing.
func TestValueOrElse(t *testing.T) {
	t.Run("NonNilPointer", func(t *testing.T) {
		called := false
		result := ValueOrElse(new(42), func() int {
			called = true

			return 99
		})
		assert.Equal(t, 42, result, "Should return the pointed-to value")
		assert.False(t, called, "Should not call fallback function when pointer is non-nil")
	})

	t.Run("NilPointer", func(t *testing.T) {
		result := ValueOrElse(nil, func() int { return 99 })
		assert.Equal(t, 99, result, "Should return fallback value when nil")
	})

	t.Run("NilWithZeroFallback", func(t *testing.T) {
		result := ValueOrElse(nil, func() int { return 0 })
		assert.Equal(t, 0, result, "Should return zero from fallback function")
	})
}

// TestEqual tests pointer equality comparison.
func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        *int
		b        *int
		expected bool
	}{
		{"BothNil", nil, nil, true},
		{"FirstNil", nil, new(1), false},
		{"SecondNil", new(1), nil, false},
		{"EqualValues", new(42), new(42), true},
		{"DifferentValues", new(1), new(2), false},
		{"BothZero", new(0), new(0), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Equal(tt.a, tt.b), "Should compare pointer values correctly")
		})
	}
}

// TestEqualString tests Equal with string pointers.
func TestEqualString(t *testing.T) {
	assert.True(t, Equal(new("hello"), new("hello")), "Should be equal for same string values")
	assert.False(t, Equal(new("hello"), new("world")), "Should not be equal for different strings")
	assert.True(t, Equal[string](nil, nil), "Should be equal for both nil")
}

// TestEqualSamePointer tests Equal when both arguments point to the same address.
func TestEqualSamePointer(t *testing.T) {
	p := new(42)
	assert.True(t, Equal(p, p), "Should be equal when same pointer")
}

// TestCoalesce tests first non-nil pointer selection.
func TestCoalesce(t *testing.T) {
	t.Run("AllNil", func(t *testing.T) {
		assert.Nil(t, Coalesce[int](), "Should return nil for empty args")
		assert.Nil(t, Coalesce[int](nil, nil, nil), "Should return nil when all are nil")
	})

	t.Run("FirstNonNil", func(t *testing.T) {
		p := new(1)
		result := Coalesce(p, new(2))
		assert.Same(t, p, result, "Should return the first non-nil pointer")
	})

	t.Run("SkipsNils", func(t *testing.T) {
		p := new(42)
		result := Coalesce(nil, nil, p)
		assert.Same(t, p, result, "Should skip nil pointers and return first non-nil")
	})

	t.Run("ReturnsSamePointer", func(t *testing.T) {
		p := new(10)
		result := Coalesce(p)
		assert.Same(t, p, result, "Should return the exact same pointer, not a copy")
	})
}
