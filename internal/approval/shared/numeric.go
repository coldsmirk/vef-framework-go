package shared

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// ToFloat64 normalizes any JSON-decoded or Go-native numeric value to
// float64. It is the shared numeric bridge for form-data handling: condition
// evaluation and form validation both compare user-supplied numbers, and both
// must accept the full spread of types a decoder or caller may produce.
func ToFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()

		return number, err == nil
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

// FormatFixedRowItemValue normalizes a fixed-row item value (non-blank string
// or finite number, including json.Number) to its canonical string key so
// schema projection, deploy validation, and runtime submission validation
// agree on item identity regardless of how form data was decoded.
func FormatFixedRowItemValue(raw any) (string, bool) {
	if text, ok := raw.(string); ok {
		if strings.TrimSpace(text) == "" {
			return "", false
		}

		return text, true
	}

	number, ok := ToFloat64(raw)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return "", false
	}

	if number == 0 {
		number = 0
	}

	return strconv.FormatFloat(number, 'f', -1, 64), true
}
