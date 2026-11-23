package shared

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatFixedRowItemValue(t *testing.T) {
	t.Run("ValidValues", func(t *testing.T) {
		cases := []struct {
			name  string
			input any
			want  string
		}{
			{name: "String", input: "item_1", want: "item_1"},
			{name: "Int", input: 42, want: "42"},
			{name: "Int64", input: int64(-7), want: "-7"},
			{name: "Uint", input: uint(9), want: "9"},
			{name: "Float64Whole", input: float64(2), want: "2"},
			{name: "Float64Decimal", input: 1.5, want: "1.5"},
			{name: "Float64Large", input: float64(1_000_000), want: "1000000"},
			{name: "NegativeZero", input: math.Copysign(0, -1), want: "0"},
			{name: "JSONNumberInt", input: json.Number("2"), want: "2"},
			{name: "JSONNumberWholeFloat", input: json.Number("2.0"), want: "2"},
			{name: "JSONNumberDecimal", input: json.Number("3.25"), want: "3.25"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := FormatFixedRowItemValue(tc.input)
				assert.True(t, ok, "Should accept valid fixed-row item value")
				assert.Equal(t, tc.want, got, "Should format fixed-row item value canonically")
			})
		}
	})

	t.Run("InvalidValues", func(t *testing.T) {
		cases := []struct {
			name  string
			input any
		}{
			{name: "Nil", input: nil},
			{name: "EmptyString", input: ""},
			{name: "WhitespaceString", input: "   \t\n"},
			{name: "Bool", input: true},
			{name: "Slice", input: []any{"a"}},
			{name: "Map", input: map[string]any{"k": "v"}},
			{name: "NaN", input: math.NaN()},
			{name: "PosInf", input: math.Inf(1)},
			{name: "NegInf", input: math.Inf(-1)},
			{name: "JSONNumberMalformed", input: json.Number("abc")},
			{name: "JSONNumberNaN", input: json.Number("NaN")},
			{name: "JSONNumberInf", input: json.Number("Inf")},
			{name: "JSONNumberOverflow", input: json.Number("1e309")},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := FormatFixedRowItemValue(tc.input)
				assert.False(t, ok, "Should reject invalid fixed-row item value")
				assert.Empty(t, got, "Rejected value should return empty string")
			})
		}
	})
}
