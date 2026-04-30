package tabular

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultParser exercises the default ValueParser implementation across
// every value family it advertises support for.
func TestDefaultParser(t *testing.T) {
	t.Run("EmptyStringYieldsZero", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name       string
			targetType reflect.Type
		}{
			{"String", reflect.TypeFor[string]()},
			{"Int", reflect.TypeFor[int]()},
			{"Float", reflect.TypeFor[float64]()},
			{"Bool", reflect.TypeFor[bool]()},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse("", tt.targetType)
				require.NoError(t, err, "Empty string should be a valid input for %s and yield the zero value", tt.targetType)
				assert.Equal(t, reflect.Zero(tt.targetType).Interface(), result,
					"Empty string should produce the zero value of the target type")
			})
		}
	})

	t.Run("EmptyStringForPointerYieldsNil", func(t *testing.T) {
		parser := NewDefaultParser("")

		result, err := parser.Parse("", reflect.TypeFor[*string]())
		require.NoError(t, err, "Empty string should be a valid input for pointer types")
		assert.Nil(t, result, "Empty string should map to nil for pointer types so callers can detect 'absent'")
	})

	t.Run("BasicTypes", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name       string
			cellValue  string
			targetType reflect.Type
			expected   any
		}{
			{"String", "hello", reflect.TypeFor[string](), "hello"},
			{"Int", "42", reflect.TypeFor[int](), 42},
			{"Int8", "127", reflect.TypeFor[int8](), int8(127)},
			{"Int16", "32767", reflect.TypeFor[int16](), int16(32767)},
			{"Int32", "2147483647", reflect.TypeFor[int32](), int32(2147483647)},
			{"Int64", "9223372036854775807", reflect.TypeFor[int64](), int64(9223372036854775807)},
			{"Uint", "42", reflect.TypeFor[uint](), uint(42)},
			{"Uint8", "255", reflect.TypeFor[uint8](), uint8(255)},
			{"Uint16", "65535", reflect.TypeFor[uint16](), uint16(65535)},
			{"Uint32", "4294967295", reflect.TypeFor[uint32](), uint32(4294967295)},
			{"Uint64", "18446744073709551615", reflect.TypeFor[uint64](), uint64(18446744073709551615)},
			{"Float32", "3.14", reflect.TypeFor[float32](), float32(3.14)},
			{"Float64", "3.14159265359", reflect.TypeFor[float64](), float64(3.14159265359)},
			{"BoolTrue", "true", reflect.TypeFor[bool](), true},
			{"BoolFalse", "false", reflect.TypeFor[bool](), false},
			{"Bool1", "1", reflect.TypeFor[bool](), true},
			{"Bool0", "0", reflect.TypeFor[bool](), false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse(tt.cellValue, tt.targetType)
				require.NoError(t, err, "Parse should succeed for valid %s input %q", tt.targetType, tt.cellValue)
				assert.Equal(t, tt.expected, result, "Parse should produce the canonical %s value", tt.targetType)
			})
		}
	})

	t.Run("InvalidBasicTypes", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name       string
			cellValue  string
			targetType reflect.Type
		}{
			{"InvalidInt", "not_a_number", reflect.TypeFor[int]()},
			{"InvalidFloat", "abc", reflect.TypeFor[float64]()},
			{"InvalidBool", "maybe", reflect.TypeFor[bool]()},
			{"InvalidUint", "-1", reflect.TypeFor[uint]()},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := parser.Parse(tt.cellValue, tt.targetType)
				require.Error(t, err, "Parse should reject %q for target type %s", tt.cellValue, tt.targetType)
			})
		}
	})

	t.Run("Pointers", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name       string
			cellValue  string
			targetType reflect.Type
			validate   func(*testing.T, any)
		}{
			{
				name:       "StringPointer",
				cellValue:  "test",
				targetType: reflect.TypeFor[*string](),
				validate: func(t *testing.T, result any) {
					ptr, ok := result.(*string)
					require.True(t, ok, "Parse should return *string when target is *string")
					require.NotNil(t, ptr, "Pointer result should not be nil for non-empty input")
					assert.Equal(t, "test", *ptr, "Pointer should reference the parsed value")
				},
			},
			{
				name:       "IntPointer",
				cellValue:  "42",
				targetType: reflect.TypeFor[*int](),
				validate: func(t *testing.T, result any) {
					ptr, ok := result.(*int)
					require.True(t, ok, "Parse should return *int when target is *int")
					require.NotNil(t, ptr, "Pointer result should not be nil for non-empty input")
					assert.Equal(t, 42, *ptr, "Pointer should reference the parsed value")
				},
			},
			{
				name:       "BoolPointer",
				cellValue:  "true",
				targetType: reflect.TypeFor[*bool](),
				validate: func(t *testing.T, result any) {
					ptr, ok := result.(*bool)
					require.True(t, ok, "Parse should return *bool when target is *bool")
					require.NotNil(t, ptr, "Pointer result should not be nil for non-empty input")
					assert.True(t, *ptr, "Pointer should reference the parsed value")
				},
			},
			{
				name:       "Float64Pointer",
				cellValue:  "3.14",
				targetType: reflect.TypeFor[*float64](),
				validate: func(t *testing.T, result any) {
					ptr, ok := result.(*float64)
					require.True(t, ok, "Parse should return *float64 when target is *float64")
					require.NotNil(t, ptr, "Pointer result should not be nil for non-empty input")
					assert.Equal(t, 3.14, *ptr, "Pointer should reference the parsed value")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse(tt.cellValue, tt.targetType)
				require.NoError(t, err, "Parse should succeed for pointer target %s", tt.targetType)
				tt.validate(t, result)
			})
		}
	})

	t.Run("TimeDefaultLayout", func(t *testing.T) {
		parser := NewDefaultParser("")

		result, err := parser.Parse("2024-01-15 14:30:45", typeTime)
		require.NoError(t, err, "Parse should accept the default time layout")

		parsed, ok := result.(time.Time)
		require.True(t, ok, "Result should be time.Time when target is time.Time")

		expected := time.Date(2024, 1, 15, 14, 30, 45, 0, time.Local)
		assert.Equal(t, expected, parsed, "Parsed time should equal the source instant in time.Local")
	})

	t.Run("TimeWithFormat", func(t *testing.T) {
		tests := []struct {
			name      string
			format    string
			cellValue string
			validate  func(*testing.T, any)
		}{
			{
				name:      "CustomDateOnly",
				format:    "2006-01-02",
				cellValue: "2024-01-15",
				validate: func(t *testing.T, result any) {
					parsed, ok := result.(time.Time)
					require.True(t, ok, "Result should be time.Time")

					expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)
					assert.Equal(t, expected, parsed, "Date-only layout should yield midnight in time.Local")
				},
			},
			{
				name:      "RFC3339",
				format:    time.RFC3339,
				cellValue: "2024-01-15T14:30:45+08:00",
				validate: func(t *testing.T, result any) {
					parsed, ok := result.(time.Time)
					require.True(t, ok, "Result should be time.Time")
					assert.Equal(t, 2024, parsed.Year(), "RFC3339 input should preserve year")
					assert.Equal(t, time.January, parsed.Month(), "RFC3339 input should preserve month")
					assert.Equal(t, 15, parsed.Day(), "RFC3339 input should preserve day")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				parser := NewDefaultParser(tt.format)
				result, err := parser.Parse(tt.cellValue, typeTime)
				require.NoError(t, err, "Parse should succeed for layout %q", tt.format)
				tt.validate(t, result)
			})
		}
	})

	t.Run("InvalidTime", func(t *testing.T) {
		parser := NewDefaultParser("")

		_, err := parser.Parse("not_a_time", typeTime)
		require.Error(t, err, "Parse should reject input that does not match the configured time layout")
	})

	t.Run("Decimal", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name      string
			cellValue string
			expected  string
		}{
			{"Integer", "100", "100"},
			{"Float", "3.14", "3.14"},
			{"Scientific", "1.23e+2", "123"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse(tt.cellValue, typeDecimal)
				require.NoError(t, err, "Parse should accept decimal input %q", tt.cellValue)

				d, ok := result.(decimal.Decimal)
				require.True(t, ok, "Result should be decimal.Decimal when target is decimal.Decimal")
				assert.Equal(t, tt.expected, d.String(), "Decimal should round-trip via its canonical String() form")
			})
		}
	})

	t.Run("InvalidDecimal", func(t *testing.T) {
		parser := NewDefaultParser("")

		_, err := parser.Parse("not_a_number", typeDecimal)
		require.Error(t, err, "Parse should reject decimal input that decimal.Decimal cannot accept")
	})

	t.Run("UnsupportedTypes", func(t *testing.T) {
		parser := NewDefaultParser("")

		type CustomStruct struct {
			Field string
		}

		tests := []struct {
			name       string
			cellValue  string
			targetType reflect.Type
		}{
			{"UnsupportedStruct", "data", reflect.TypeFor[CustomStruct]()},
			{"UnsupportedSlice", "[1,2,3]", reflect.TypeFor[[]int]()},
			{"UnsupportedMap", "{}", reflect.TypeFor[map[string]string]()},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := parser.Parse(tt.cellValue, tt.targetType)
				require.Error(t, err, "Parse should reject unsupported target type %s", tt.targetType)
				assert.ErrorIs(t, err, ErrUnsupportedType,
					"Error should wrap ErrUnsupportedType so callers can branch on the cause")
			})
		}
	})

	t.Run("EdgeCases", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name       string
			cellValue  string
			targetType reflect.Type
			expected   any
		}{
			{"ZeroInt", "0", reflect.TypeFor[int](), 0},
			{"ZeroFloat", "0.0", reflect.TypeFor[float64](), 0.0},
			{"NegativeInt", "-42", reflect.TypeFor[int](), -42},
			{"NegativeFloat", "-3.14", reflect.TypeFor[float64](), -3.14},
			{"LeadingZeros", "007", reflect.TypeFor[int](), 7},
			{"WhitespaceString", "  test  ", reflect.TypeFor[string](), "  test  "},
			{"MaxInt64", "9223372036854775807", reflect.TypeFor[int64](), int64(9223372036854775807)},
			{"MinInt64", "-9223372036854775808", reflect.TypeFor[int64](), int64(-9223372036854775808)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse(tt.cellValue, tt.targetType)
				require.NoError(t, err, "Parse should succeed for boundary input %q targeting %s", tt.cellValue, tt.targetType)
				assert.Equal(t, tt.expected, result, "Boundary input should round-trip without overflow or truncation")
			})
		}
	})

	t.Run("UnicodeStrings", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name      string
			cellValue string
			expected  string
		}{
			{"ChineseCharacters", "你好世界", "你好世界"},
			{"EmojiCharacters", "👍🎉", "👍🎉"},
			{"MixedUnicode", "Hello世界🌍", "Hello世界🌍"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse(tt.cellValue, reflect.TypeFor[string]())
				require.NoError(t, err, "Parse should accept arbitrary UTF-8 strings")
				assert.Equal(t, tt.expected, result, "UTF-8 strings should round-trip without modification")
			})
		}
	})

	t.Run("BooleanVariants", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name      string
			cellValue string
			expected  bool
		}{
			{"True", "true", true},
			{"False", "false", false},
			{"One", "1", true},
			{"Zero", "0", false},
			{"TrueUpperCase", "True", true},
			{"TrueAllCaps", "TRUE", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse(tt.cellValue, reflect.TypeFor[bool]())
				require.NoError(t, err, "Parse should accept boolean spelling %q", tt.cellValue)
				assert.Equal(t, tt.expected, result, "Parse should resolve %q to %t", tt.cellValue, tt.expected)
			})
		}
	})

	t.Run("FloatPrecision", func(t *testing.T) {
		parser := NewDefaultParser("")

		tests := []struct {
			name       string
			cellValue  string
			targetType reflect.Type
			validate   func(*testing.T, any)
		}{
			{
				name:       "Float32",
				cellValue:  "3.14159265359",
				targetType: reflect.TypeFor[float32](),
				validate: func(t *testing.T, result any) {
					f, ok := result.(float32)
					require.True(t, ok, "Result should be float32 when target is float32")
					assert.InDelta(t, 3.14159, f, 0.00001, "float32 should preserve roughly 7 significant digits")
				},
			},
			{
				name:       "Float64",
				cellValue:  "3.14159265359",
				targetType: reflect.TypeFor[float64](),
				validate: func(t *testing.T, result any) {
					f, ok := result.(float64)
					require.True(t, ok, "Result should be float64 when target is float64")
					assert.InDelta(t, 3.14159265359, f, 0.000000001, "float64 should preserve full input precision")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := parser.Parse(tt.cellValue, tt.targetType)
				require.NoError(t, err, "Parse should succeed for %s precision check", tt.targetType)
				tt.validate(t, result)
			})
		}
	})

	t.Run("Constructor", func(t *testing.T) {
		tests := []struct {
			name   string
			format string
		}{
			{"EmptyFormat", ""},
			{"DateFormat", "2006-01-02"},
			{"TimeFormat", time.RFC3339},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				parser := NewDefaultParser(tt.format)
				assert.NotNil(t, parser, "NewDefaultParser should always return a non-nil parser")
			})
		}
	})
}
