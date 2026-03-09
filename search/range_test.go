package search

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/monad"
)

// TestGetRangeValue tests getRangeValue with various input types.
func TestGetRangeValue(t *testing.T) {
	t.Run("MonadRange", func(t *testing.T) {
		r := monad.Range[int]{Start: 10, End: 20}
		start, end, ok := getRangeValue(r, nil)

		require.True(t, ok, "Should recognize monad.Range")
		assert.Equal(t, 10, start, "Start should be 10")
		assert.Equal(t, 20, end, "End should be 20")
	})

	t.Run("StringRangeInt", func(t *testing.T) {
		params := map[string]string{ParamType: TypeInt}
		start, end, ok := getRangeValue("1,100", params)

		require.True(t, ok, "Should parse string range")
		assert.Equal(t, 1, start, "Start should be 1")
		assert.Equal(t, 100, end, "End should be 100")
	})

	t.Run("SliceRange", func(t *testing.T) {
		slice := []int{5, 15}
		start, end, ok := getRangeValue(slice, nil)

		require.True(t, ok, "Should parse slice range")
		assert.Equal(t, 5, start, "Start should be 5")
		assert.Equal(t, 15, end, "End should be 15")
	})

	t.Run("UnsupportedType", func(t *testing.T) {
		_, _, ok := getRangeValue(42, nil)
		assert.False(t, ok, "Should return false for unsupported type")
	})

	t.Run("PointerToMonadRange", func(t *testing.T) {
		r := monad.Range[int]{Start: 1, End: 5}
		start, end, ok := getRangeValue(&r, nil)

		require.True(t, ok, "Should handle pointer to monad.Range")
		assert.Equal(t, 1, start, "Start should be 1")
		assert.Equal(t, 5, end, "End should be 5")
	})
}

// TestParseStringRange tests parseStringRange with various inputs.
func TestParseStringRange(t *testing.T) {
	t.Run("EmptyValue", func(t *testing.T) {
		_, _, ok := parseStringRange("", nil)
		assert.False(t, ok, "Empty string should return false")
	})

	t.Run("MissingDelimiter", func(t *testing.T) {
		_, _, ok := parseStringRange("single_value", map[string]string{ParamType: TypeInt})
		assert.False(t, ok, "String without delimiter should return false")
	})

	t.Run("CustomDelimiter", func(t *testing.T) {
		params := map[string]string{ParamDelimiter: "-", ParamType: TypeInt}
		start, end, ok := parseStringRange("10-20", params)

		require.True(t, ok, "Should parse with custom delimiter")
		assert.Equal(t, 10, start, "Start should be 10")
		assert.Equal(t, 20, end, "End should be 20")
	})

	t.Run("DefaultDelimiter", func(t *testing.T) {
		params := map[string]string{ParamType: TypeInt}
		start, end, ok := parseStringRange("5,15", params)

		require.True(t, ok, "Should parse with default comma delimiter")
		assert.Equal(t, 5, start, "Start should be 5")
		assert.Equal(t, 15, end, "End should be 15")
	})

	t.Run("UnknownType", func(t *testing.T) {
		params := map[string]string{ParamType: "unknown"}
		_, _, ok := parseStringRange("1,2", params)
		assert.False(t, ok, "Unknown type should return false")
	})

	t.Run("NoType", func(t *testing.T) {
		_, _, ok := parseStringRange("1,2", map[string]string{})
		assert.False(t, ok, "Missing type should return false")
	})

	t.Run("DecimalType", func(t *testing.T) {
		params := map[string]string{ParamType: TypeDecimal}
		start, end, ok := parseStringRange("1.5,3.5", params)

		require.True(t, ok, "Should parse decimal range")
		assert.NotNil(t, start, "Start should not be nil")
		assert.NotNil(t, end, "End should not be nil")
	})

	t.Run("DateType", func(t *testing.T) {
		params := map[string]string{ParamType: TypeDate}
		start, end, ok := parseStringRange("2024-01-01,2024-12-31", params)

		require.True(t, ok, "Should parse date range")
		assert.IsType(t, time.Time{}, start, "Start should be time.Time")
		assert.IsType(t, time.Time{}, end, "End should be time.Time")
	})

	t.Run("TimeType", func(t *testing.T) {
		params := map[string]string{ParamType: TypeTime}
		start, end, ok := parseStringRange("08:00:00,17:00:00", params)

		require.True(t, ok, "Should parse time range")
		assert.IsType(t, time.Time{}, start, "Start should be time.Time")
		assert.IsType(t, time.Time{}, end, "End should be time.Time")
	})

	t.Run("DateTimeType", func(t *testing.T) {
		params := map[string]string{ParamType: TypeDateTime}
		start, end, ok := parseStringRange("2024-01-01 08:00:00,2024-12-31 17:00:00", params)

		require.True(t, ok, "Should parse datetime range")
		assert.IsType(t, time.Time{}, start, "Start should be time.Time")
		assert.IsType(t, time.Time{}, end, "End should be time.Time")
	})
}

// TestParseSliceRange tests parseSliceRange with various inputs.
func TestParseSliceRange(t *testing.T) {
	t.Run("EmptySlice", func(t *testing.T) {
		v := reflect.ValueOf([]int{})
		_, _, ok := parseSliceRange(v)
		assert.False(t, ok, "Empty slice should return false")
	})

	t.Run("SingleElement", func(t *testing.T) {
		v := reflect.ValueOf([]int{1})
		_, _, ok := parseSliceRange(v)
		assert.False(t, ok, "Single-element slice should return false")
	})

	t.Run("TwoElements", func(t *testing.T) {
		v := reflect.ValueOf([]int{10, 20})
		start, end, ok := parseSliceRange(v)

		require.True(t, ok, "Two-element slice should return true")
		assert.Equal(t, 10, start, "Start should be 10")
		assert.Equal(t, 20, end, "End should be 20")
	})

	t.Run("ThreeElements", func(t *testing.T) {
		v := reflect.ValueOf([]int{1, 2, 3})
		_, _, ok := parseSliceRange(v)
		assert.False(t, ok, "Three-element slice should return false")
	})

	t.Run("StringSlice", func(t *testing.T) {
		v := reflect.ValueOf([]string{"a", "z"})
		start, end, ok := parseSliceRange(v)

		require.True(t, ok, "String slice should work")
		assert.Equal(t, "a", start, "Start should be 'a'")
		assert.Equal(t, "z", end, "End should be 'z'")
	})
}

// TestParseIntRange tests parseIntRange with various inputs.
func TestParseIntRange(t *testing.T) {
	t.Run("ValidInts", func(t *testing.T) {
		start, end, ok := parseIntRange([]string{"1", "100"})

		require.True(t, ok, "Should parse valid integers")
		assert.Equal(t, 1, start, "Start should be 1")
		assert.Equal(t, 100, end, "End should be 100")
	})

	t.Run("InvalidStart", func(t *testing.T) {
		_, _, ok := parseIntRange([]string{"abc", "100"})
		assert.False(t, ok, "Invalid start should return false")
	})

	t.Run("InvalidEnd", func(t *testing.T) {
		_, _, ok := parseIntRange([]string{"1", "xyz"})
		assert.False(t, ok, "Invalid end should return false")
	})

	t.Run("NegativeValues", func(t *testing.T) {
		start, end, ok := parseIntRange([]string{"-10", "-1"})

		require.True(t, ok, "Should parse negative integers")
		assert.Equal(t, -10, start, "Start should be -10")
		assert.Equal(t, -1, end, "End should be -1")
	})

	t.Run("ZeroValues", func(t *testing.T) {
		start, end, ok := parseIntRange([]string{"0", "0"})

		require.True(t, ok, "Should parse zero values")
		assert.Equal(t, 0, start, "Start should be 0")
		assert.Equal(t, 0, end, "End should be 0")
	})
}

// TestParseDecimalRange tests parseDecimalRange with various inputs.
func TestParseDecimalRange(t *testing.T) {
	t.Run("ValidDecimals", func(t *testing.T) {
		start, end, ok := parseDecimalRange([]string{"1.5", "99.99"})

		require.True(t, ok, "Should parse valid decimals")
		assert.NotNil(t, start, "Start should not be nil")
		assert.NotNil(t, end, "End should not be nil")
	})

	t.Run("InvalidStart", func(t *testing.T) {
		_, _, ok := parseDecimalRange([]string{"Not_a_number", "99.99"})
		assert.False(t, ok, "Invalid start should return false")
	})

	t.Run("InvalidEnd", func(t *testing.T) {
		_, _, ok := parseDecimalRange([]string{"1.5", "not_a_number"})
		assert.False(t, ok, "Invalid end should return false")
	})
}

// TestParseDateRange tests parseDateRange with various inputs.
func TestParseDateRange(t *testing.T) {
	t.Run("ValidDates", func(t *testing.T) {
		start, end, ok := parseDateRange([]string{"2024-01-01", "2024-12-31"})

		require.True(t, ok, "Should parse valid dates")
		assert.IsType(t, time.Time{}, start, "Start should be time.Time")
		assert.IsType(t, time.Time{}, end, "End should be time.Time")
	})

	t.Run("InvalidStart", func(t *testing.T) {
		_, _, ok := parseDateRange([]string{"not-a-date", "2024-12-31"})
		assert.False(t, ok, "Invalid start date should return false")
	})

	t.Run("InvalidEnd", func(t *testing.T) {
		_, _, ok := parseDateRange([]string{"2024-01-01", "not-a-date"})
		assert.False(t, ok, "Invalid end date should return false")
	})
}

// TestParseTimeRange tests parseTimeRange with various inputs.
func TestParseTimeRange(t *testing.T) {
	t.Run("ValidTimes", func(t *testing.T) {
		start, end, ok := parseTimeRange([]string{"08:00:00", "17:30:00"})

		require.True(t, ok, "Should parse valid times")
		assert.IsType(t, time.Time{}, start, "Start should be time.Time")
		assert.IsType(t, time.Time{}, end, "End should be time.Time")
	})

	t.Run("InvalidStart", func(t *testing.T) {
		_, _, ok := parseTimeRange([]string{"bad", "17:30:00"})
		assert.False(t, ok, "Invalid start time should return false")
	})

	t.Run("InvalidEnd", func(t *testing.T) {
		_, _, ok := parseTimeRange([]string{"08:00:00", "bad"})
		assert.False(t, ok, "Invalid end time should return false")
	})
}

// TestParseDateTimeRange tests parseDateTimeRange with various inputs.
func TestParseDateTimeRange(t *testing.T) {
	t.Run("ValidDateTimes", func(t *testing.T) {
		start, end, ok := parseDateTimeRange([]string{"2024-01-01 08:00:00", "2024-12-31 17:30:00"})

		require.True(t, ok, "Should parse valid datetimes")
		assert.IsType(t, time.Time{}, start, "Start should be time.Time")
		assert.IsType(t, time.Time{}, end, "End should be time.Time")
	})

	t.Run("InvalidStart", func(t *testing.T) {
		_, _, ok := parseDateTimeRange([]string{"bad", "2024-12-31 17:30:00"})
		assert.False(t, ok, "Invalid start datetime should return false")
	})

	t.Run("InvalidEnd", func(t *testing.T) {
		_, _, ok := parseDateTimeRange([]string{"2024-01-01 08:00:00", "bad"})
		assert.False(t, ok, "Invalid end datetime should return false")
	})
}
