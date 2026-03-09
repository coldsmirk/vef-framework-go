package decimal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStringer is a helper type implementing fmt.Stringer for testing.
type TestStringer struct {
	value string
}

func (s TestStringer) String() string {
	return s.value
}

func TestNewFromAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected Decimal
		wantErr  bool
	}{
		// Decimal types
		{"Decimal", NewFromInt(42), NewFromInt(42), false},
		{"DecimalPointerNonNil", new(NewFromFloat(3.14)), NewFromFloat(3.14), false},
		{"DecimalPointerNil", (*Decimal)(nil), Zero, false},

		// Signed integers
		{"Int", int(100), NewFromInt(100), false},
		{"Int8", int8(8), NewFromInt(8), false},
		{"Int16", int16(16), NewFromInt(16), false},
		{"Int32", int32(32), NewFromInt(32), false},
		{"Int64", int64(64), NewFromInt(64), false},
		{"IntNegative", int(-50), NewFromInt(-50), false},

		// Unsigned integers
		{"Uint", uint(200), NewFromUint64(200), false},
		{"Uint8", uint8(8), NewFromUint64(8), false},
		{"Uint16", uint16(16), NewFromUint64(16), false},
		{"Uint32", uint32(32), NewFromUint64(32), false},
		{"Uint64", uint64(64), NewFromUint64(64), false},
		{"Uint64ExceedsInt64Max", uint64(1<<63 + 100), NewFromUint64(1<<63 + 100), false},

		// Floats
		{"Float32", float32(1.5), NewFromFloat32(1.5), false},
		{"Float64", float64(2.5), NewFromFloat(2.5), false},
		{"Float64Negative", float64(-3.14), NewFromFloat(-3.14), false},

		// String types
		{"String", "123.456", RequireFromString("123.456"), false},
		{"StringInvalid", "not_a_number", Zero, true},
		{"ByteSlice", []byte("789.01"), RequireFromString("789.01"), false},
		{"ByteSliceInvalid", []byte("bad"), Zero, true},

		// Bool
		{"BoolTrue", true, One, false},
		{"BoolFalse", false, Zero, false},

		// fmt.Stringer
		{"Stringer", TestStringer{"99.99"}, RequireFromString("99.99"), false},
		{"StringerInvalid", TestStringer{"bad"}, Zero, true},

		// Unsupported
		{"UnsupportedStruct", struct{}{}, Zero, true},
		{"NilInput", nil, Zero, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewFromAny(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "Should return error")

				return
			}

			require.NoError(t, err, "Should convert without error")
			assert.True(t, tt.expected.Equal(got),
				"Should return %s, got %s", tt.expected.String(), got.String())
		})
	}
}

func TestMustFromAny(t *testing.T) {
	t.Parallel()

	t.Run("ValidValue", func(t *testing.T) {
		t.Parallel()

		got := MustFromAny(42)
		assert.True(t, NewFromInt(42).Equal(got), "Should convert int to Decimal")
	})

	t.Run("PanicsOnUnsupportedType", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			MustFromAny(struct{}{})
		}, "Should panic for unsupported type")
	})
}
