package tabular

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// TestDefaultFormatterFormatBasicTypes tests DefaultFormatter Format basic types scenarios.
func TestDefaultFormatterFormatBasicTypes(t *testing.T) {
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
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatNilValue tests DefaultFormatter Format nil value scenarios.
func TestDefaultFormatterFormatNilValue(t *testing.T) {
	formatter := NewDefaultFormatter("")

	result, err := formatter.Format(nil)
	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, "", result, "Should equal expected value")
}

// TestDefaultFormatterFormatPointerTypes tests DefaultFormatter Format pointer types scenarios.
func TestDefaultFormatterFormatPointerTypes(t *testing.T) {
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
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatNullTypes tests DefaultFormatter Format null types scenarios.
func TestDefaultFormatterFormatNullTypes(t *testing.T) {
	formatter := NewDefaultFormatter("")

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"NullStringValid", null.StringFrom("test"), "test"},
		{"NullStringInvalid", null.String{}, ""},
		{"NullIntValid", null.IntFrom(42), "42"},
		{"NullIntInvalid", null.Int{}, ""},
		{"NullInt16Valid", null.Int16From(123), "123"},
		{"NullInt16Invalid", null.Int16{}, ""},
		{"NullInt32Valid", null.Int32From(456), "456"},
		{"NullInt32Invalid", null.Int32{}, ""},
		{"NullFloatValid", null.FloatFrom(3.14), "3.14"},
		{"NullFloatInvalid", null.Float{}, ""},
		{"NullBoolValid", null.BoolFrom(true), "true"},
		{"NullBoolInvalid", null.Bool{}, ""},
		{"NullByteValid", null.ByteFrom(255), "255"},
		{"NullByteInvalid", null.Byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.input)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatTimeTypes tests DefaultFormatter Format time types scenarios.
func TestDefaultFormatterFormatTimeTypes(t *testing.T) {
	formatter := NewDefaultFormatter("")

	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.Local)

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"TimeTime", testTime, "2024-01-15 14:30:45"},
		{"NullDateTimeValid", null.DateTimeFrom(timex.DateTime(testTime)), "2024-01-15 14:30:45"},
		{"NullDateTimeInvalid", null.DateTime{}, ""},
		{"NullDateValid", null.DateFrom(timex.Date(testTime)), "2024-01-15"},
		{"NullDateInvalid", null.Date{}, ""},
		{"NullTimeValid", null.TimeFrom(timex.Time(testTime)), "14:30:45"},
		{"NullTimeInvalid", null.Time{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.input)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatTimeTypesWithFormat tests DefaultFormatter Format time types with format scenarios.
func TestDefaultFormatterFormatTimeTypesWithFormat(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.Local)

	tests := []struct {
		name     string
		format   string
		input    any
		expected string
	}{
		{"TimeTimeCustomFormat", "2006-01-02", testTime, "2024-01-15"},
		{"TimeTimeRFC3339", time.RFC3339, testTime, "2024-01-15T14:30:45+08:00"},
		{"DateTimeCustomFormat", "2006/01/02 15:04", timex.DateTime(testTime), "2024/01/15 14:30"},
		{"DateCustomFormat", "2006年01月02日", timex.Date(testTime), "2024年01月15日"},
		{"TimeCustomFormat", "15:04", timex.Time(testTime), "14:30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewDefaultFormatter(tt.format)
			result, err := formatter.Format(tt.input)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatFloatWithFormat tests DefaultFormatter Format float with format scenarios.
func TestDefaultFormatterFormatFloatWithFormat(t *testing.T) {
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
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatDecimalType tests DefaultFormatter Format decimal type scenarios.
func TestDefaultFormatterFormatDecimalType(t *testing.T) {
	formatter := NewDefaultFormatter("")

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"DecimalInteger", decimal.NewFromInt(100), "100"},
		{"DecimalFloat", decimal.NewFromFloat(3.14), "3.14"},
		{"DecimalString", decimal.RequireFromString("123.456"), "123.456"},
		{"NullDecimalValid", null.DecimalFrom(decimal.NewFromFloat(99.99)), "99.99"},
		{"NullDecimalInvalid", null.Decimal{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.input)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatPointerToPointer tests DefaultFormatter Format pointer to pointer scenarios.
func TestDefaultFormatterFormatPointerToPointer(t *testing.T) {
	formatter := NewDefaultFormatter("")

	str := "test"
	strPtr := &str
	strPtrPtr := &strPtr

	result, err := formatter.Format(strPtrPtr)
	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, "test", result, "Should equal expected value")
}

// TestDefaultFormatterFormatEdgeCases tests DefaultFormatter Format edge cases scenarios.
func TestDefaultFormatterFormatEdgeCases(t *testing.T) {
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
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatUnicodeStrings tests DefaultFormatter Format unicode strings scenarios.
func TestDefaultFormatterFormatUnicodeStrings(t *testing.T) {
	formatter := NewDefaultFormatter("")

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"ChineseCharacters", "你好世界", "你好世界"},
		{"EmojiCharacters", "👍🎉", "👍🎉"},
		{"MixedUnicode", "Hello世界🌍", "Hello世界🌍"},
		{"NullStringUnicode", null.StringFrom("测试数据"), "测试数据"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.input)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultFormatterFormatTimeZero tests DefaultFormatter Format time zero scenarios.
func TestDefaultFormatterFormatTimeZero(t *testing.T) {
	formatter := NewDefaultFormatter("")

	zeroTime := time.Time{}

	result, err := formatter.Format(zeroTime)
	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, "0001-01-01 00:00:00", result, "Should equal expected value")
}

// TestDefaultFormatterNewDefaultFormatter tests DefaultFormatter NewDefaultFormatter scenarios.
func TestDefaultFormatterNewDefaultFormatter(t *testing.T) {
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
			assert.NotNil(t, formatter, "Should not be nil")
		})
	}
}
