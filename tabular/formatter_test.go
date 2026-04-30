package tabular

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// TestDefaultFormatter exercises the default Formatter implementation across
// every value family it advertises support for. Subtests are nested so the
// scenario hierarchy is reflected in the test output.
func TestDefaultFormatter(t *testing.T) {
	t.Run("BasicTypes", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		tests := []struct {
			name     string
			input    any
			expected string
		}{
			{"String", "hello", "hello"},
			{"EmptyString", "", ""},
			{"Int", 42, "42"},
			{"Int8", int8(127), "127"},
			{"Int16", int16(32767), "32767"},
			{"Int32", int32(2147483647), "2147483647"},
			{"Int64", int64(9223372036854775807), "9223372036854775807"},
			{"Uint", uint(42), "42"},
			{"Uint8", uint8(255), "255"},
			{"Uint16", uint16(65535), "65535"},
			{"Uint32", uint32(4294967295), "4294967295"},
			{"Uint64", uint64(18446744073709551615), "18446744073709551615"},
			{"Float32", float32(3.14), "3.14"},
			{"Float64", float64(3.14159265359), "3.14159265359"},
			{"BoolTrue", true, "true"},
			{"BoolFalse", false, "false"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := formatter.Format(tt.input)
				require.NoError(t, err, "Format should succeed for value of basic type %T", tt.input)
				assert.Equal(t, tt.expected, result, "Format output should match the canonical %T representation", tt.input)
			})
		}
	})

	t.Run("NilValue", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		result, err := formatter.Format(nil)
		require.NoError(t, err, "Format should treat nil as an empty value, not an error")
		assert.Equal(t, "", result, "Nil input should produce an empty string")
	})

	t.Run("Pointers", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		str := "test"
		num := 42
		flag := true

		tests := []struct {
			name     string
			input    any
			expected string
		}{
			{"StringPointer", &str, "test"},
			{"IntPointer", &num, "42"},
			{"BoolPointer", &flag, "true"},
			{"NilStringPointer", (*string)(nil), ""},
			{"NilIntPointer", (*int)(nil), ""},
			{"NilBoolPointer", (*bool)(nil), ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := formatter.Format(tt.input)
				require.NoError(t, err, "Format should dereference pointers without erroring")
				assert.Equal(t, tt.expected, result, "Dereferenced pointer should format as the underlying value, nil pointer as empty string")
			})
		}
	})

	t.Run("PointerToPointer", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		str := "test"
		strPtr := &str
		strPtrPtr := &strPtr

		result, err := formatter.Format(strPtrPtr)
		require.NoError(t, err, "Format should fully dereference nested pointers")
		assert.Equal(t, "test", result, "Format should walk through pointer-to-pointer to the underlying value")
	})

	t.Run("TimeDefaultLayout", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		testLocation := time.FixedZone("UTC+8", 8*60*60)
		testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, testLocation)

		result, err := formatter.Format(testTime)
		require.NoError(t, err, "Format should succeed for time.Time")
		assert.Equal(t, "2024-01-15 14:30:45", result, "Default time layout should be 2006-01-02 15:04:05")
	})

	t.Run("TimeWithFormat", func(t *testing.T) {
		testLocation := time.FixedZone("UTC+8", 8*60*60)
		testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, testLocation)

		tests := []struct {
			name     string
			format   string
			input    any
			expected string
		}{
			{"TimeTimeCustomFormat", "2006-01-02", testTime, "2024-01-15"},
			{"TimeTimeRFC3339", time.RFC3339, testTime, testTime.Format(time.RFC3339)},
			{"DateTimeCustomFormat", "2006/01/02 15:04", timex.DateTime(testTime), "2024/01/15 14:30"},
			{"DateCustomFormat", "2006年01月02日", timex.Date(testTime), "2024年01月15日"},
			{"TimeCustomFormat", "15:04", timex.Time(testTime), "14:30"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				formatter := NewDefaultFormatter(tt.format)
				result, err := formatter.Format(tt.input)
				require.NoError(t, err, "Format should succeed when a custom layout is supplied")
				assert.Equal(t, tt.expected, result, "Format should apply the configured layout to the input")
			})
		}
	})

	t.Run("FloatWithFormat", func(t *testing.T) {
		tests := []struct {
			name     string
			format   string
			input    any
			expected string
		}{
			{"Float32TwoDecimals", "%.2f", float32(3.14159), "3.14"},
			{"Float64TwoDecimals", "%.2f", float64(3.14159265), "3.14"},
			{"Float32FourDecimals", "%.4f", float32(3.14159), "3.1416"},
			{"Float64SixDecimals", "%.6f", float64(3.14159265), "3.141593"},
			{"Float32Scientific", "%.2e", float32(1234.5678), "1.23e+03"},
			{"Float64Scientific", "%.2e", float64(1234.5678), "1.23e+03"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				formatter := NewDefaultFormatter(tt.format)
				result, err := formatter.Format(tt.input)
				require.NoError(t, err, "Format should accept a printf-style float specifier")
				assert.Equal(t, tt.expected, result, "Float format output should match the printf specifier")
			})
		}
	})

	t.Run("Decimal", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		tests := []struct {
			name     string
			input    any
			expected string
		}{
			{"DecimalInteger", decimal.NewFromInt(100), "100"},
			{"DecimalFloat", decimal.NewFromFloat(3.14), "3.14"},
			{"DecimalString", decimal.RequireFromString("123.456"), "123.456"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := formatter.Format(tt.input)
				require.NoError(t, err, "Format should support decimal.Decimal natively")
				assert.Equal(t, tt.expected, result, "Decimal should format using its String() representation")
			})
		}
	})

	t.Run("EdgeCases", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		tests := []struct {
			name     string
			input    any
			expected string
		}{
			{"ZeroInt", 0, "0"},
			{"ZeroFloat", 0.0, "0"},
			{"EmptyByte", byte(0), "0"},
			{"NegativeInt", -42, "-42"},
			{"NegativeFloat", -3.14, "-3.14"},
			{"VeryLargeInt", int64(9223372036854775807), "9223372036854775807"},
			{"VerySmallInt", int64(-9223372036854775808), "-9223372036854775808"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := formatter.Format(tt.input)
				require.NoError(t, err, "Format should succeed for boundary numeric inputs")
				assert.Equal(t, tt.expected, result, "Boundary values should format without overflow or sign loss")
			})
		}
	})

	t.Run("UnicodeStrings", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		tests := []struct {
			name     string
			input    any
			expected string
		}{
			{"ChineseCharacters", "你好世界", "你好世界"},
			{"EmojiCharacters", "👍🎉", "👍🎉"},
			{"MixedUnicode", "Hello世界🌍", "Hello世界🌍"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := formatter.Format(tt.input)
				require.NoError(t, err, "Format should support arbitrary UTF-8 strings")
				assert.Equal(t, tt.expected, result, "Unicode strings should round-trip without modification")
			})
		}
	})

	t.Run("ZeroTime", func(t *testing.T) {
		formatter := NewDefaultFormatter("")

		result, err := formatter.Format(time.Time{})
		require.NoError(t, err, "Format should handle the zero time without erroring")
		assert.Equal(t, "0001-01-01 00:00:00", result, "Zero time should format as Go's epoch in the default layout")
	})

	t.Run("Constructor", func(t *testing.T) {
		tests := []struct {
			name   string
			format string
		}{
			{"EmptyFormat", ""},
			{"DateFormat", "2006-01-02"},
			{"FloatFormat", "%.2f"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				formatter := NewDefaultFormatter(tt.format)
				assert.NotNil(t, formatter, "NewDefaultFormatter should always return a non-nil formatter")
			})
		}
	})
}
