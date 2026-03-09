package reflectx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/decimal"
)

// TestToDecimalE tests ToDecimalE for all supported types.
func TestToDecimalE(t *testing.T) {
	n := 42

	var iface any = 99

	tests := []struct {
		name     string
		input    any
		expected decimal.Decimal
		wantErr  bool
	}{
		{"Nil", nil, decimal.Zero, false},
		{"Int", 42, decimal.NewFromInt(42), false},
		{"Float64", 3.14, decimal.NewFromFloat(3.14), false},
		{"String", "99.5", decimal.RequireFromString("99.5"), false},
		{"BoolTrue", true, decimal.One, false},
		{"BoolFalse", false, decimal.Zero, false},
		{"Decimal", decimal.NewFromInt(7), decimal.NewFromInt(7), false},
		{"IntPointer", &n, decimal.NewFromInt(42), false},
		{"NilPointer", (*int)(nil), decimal.Zero, false},
		{"Interface", &iface, decimal.NewFromInt(99), false},
		{"UnsupportedSlice", []int{1}, decimal.Zero, true},
		{"InvalidString", "abc", decimal.Zero, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToDecimalE(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "Should return error")

				return
			}

			require.NoError(t, err, "Should convert without error")
			assert.True(t, tt.expected.Equal(got), "Should return %s, got %s", tt.expected, got)
		})
	}
}

// TestToDecimal tests ToDecimal returns Zero on failure.
func TestToDecimal(t *testing.T) {
	assert.True(t, decimal.NewFromInt(42).Equal(ToDecimal(42)), "Should convert int to Decimal")
	assert.True(t, decimal.Zero.Equal(ToDecimal([]int{})), "Should return Zero for unsupported type")
}
