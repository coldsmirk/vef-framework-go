package timex

import (
	"fmt"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/spf13/cast"
)

const (
	jsonNull  = "null"
	jsonQuote = '"'
)

var (
	// Layout constants for different time formats.
	dateTimeLayout = time.DateTime // "2006-01-02 15:04:05"
	dateLayout     = time.DateOnly // "2006-01-02"
	timeLayout     = time.TimeOnly // "15:04:05"

	// Pattern length constants for efficient JSON processing.
	dateTimePatternLength = len(time.DateTime)
	datePatternLength     = len(time.DateOnly)
	timePatternLength     = len(time.TimeOnly)
)

// scanTimeValue is a generic helper function for scanning time values from database sources.
// It handles various input types including []byte, string, time.Time and their pointer variants.
func scanTimeValue(src any, parseString func(string) (any, error), convertTime func(time.Time) any, typeName string, dest any) error {
	switch v := src.(type) {
	case []byte:
		return parseAndAssign(string(v), parseString, dest)
	case *[]byte:
		if v == nil {
			return nil
		}

		return parseAndAssign(string(*v), parseString, dest)

	case string:
		return parseAndAssign(v, parseString, dest)
	case *string:
		if v == nil {
			return nil
		}

		return parseAndAssign(*v, parseString, dest)

	case time.Time:
		return assignValue(dest, convertTime(v))
	case *time.Time:
		if v == nil {
			return nil
		}

		return assignValue(dest, convertTime(*v))

	default:
		// All string-like sources are handled above; anything else (numbers, complex,
		// arbitrary structs) is not a valid wire form for a date/time column. Reject it
		// directly instead of stringifying it into a guaranteed parse failure.
		return fmt.Errorf("%w: %s value: %v", ErrFailedScan, typeName, src)
	}
}

// parseAndAssign parses a string value and assigns the result to the destination.
func parseAndAssign(s string, parseString func(string) (any, error), dest any) error {
	parsed, err := parseString(s)
	if err != nil {
		return err
	}

	return assignValue(dest, parsed)
}

// assignValue assigns the parsed value to the destination pointer using type assertion.
func assignValue(dest, value any) error {
	switch d := dest.(type) {
	case *DateTime:
		v, ok := value.(DateTime)
		if !ok {
			return fmt.Errorf("%w: expected DateTime, got %T", ErrUnsupportedDestType, value)
		}

		*d = v

	case *Date:
		v, ok := value.(Date)
		if !ok {
			return fmt.Errorf("%w: expected Date, got %T", ErrUnsupportedDestType, value)
		}

		*d = v

	case *Time:
		v, ok := value.(Time)
		if !ok {
			return fmt.Errorf("%w: expected Time, got %T", ErrUnsupportedDestType, value)
		}

		*d = v

	default:
		return fmt.Errorf("%w: %T", ErrUnsupportedDestType, dest)
	}

	return nil
}

// parseTimeWithFallback provides a standardized way to parse time strings with fallback support.
// It first tries the provided layout in the local timezone, then falls back to the cast library
// for common formats (RFC3339, ISO-8601, etc.). Used by the lenient public Parse* entry points.
func parseTimeWithFallback(value, layout string) (time.Time, error) {
	// Primary: try with the specified layout in the local timezone.
	parsed, err := time.ParseInLocation(layout, value, time.Local)
	if err == nil {
		return parsed, nil
	}

	// Fallback: try cast library for common time formats.
	if castTime, castErr := cast.ToTimeE(value); castErr == nil {
		return castTime, nil
	}

	// Return original error if both methods fail.
	return time.Time{}, err
}

// appendQuotedFormat renders t with layout, wrapped in JSON string quotes, into a freshly
// sized buffer. Shared by the MarshalJSON implementations of all timex types.
func appendQuotedFormat(t time.Time, layout string, length int) []byte {
	bs := make([]byte, 0, length+2)
	bs = append(bs, jsonQuote)
	bs = t.AppendFormat(bs, layout)
	bs = append(bs, jsonQuote)

	return bs
}

// unquoteJSON reports whether bs is a JSON string literal and, if so, returns its unquoted
// contents. The caller supplies its own typed error when ok is false. Deserialization is strict:
// the contents must match the type's canonical layout, which the caller enforces by parsing with
// that layout (no lenient fallback).
func unquoteJSON(bs []byte) (value string, ok bool) {
	if len(bs) < 2 || bs[0] != jsonQuote || bs[len(bs)-1] != jsonQuote {
		return "", false
	}

	return utils.UnsafeString(bs[1 : len(bs)-1]), true
}
