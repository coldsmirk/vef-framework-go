package decimal

import (
	"errors"
	"fmt"
)

var errUnsupportedType = errors.New("decimal: unsupported type")

// NewFromAny creates a Decimal from an arbitrary value.
// Supported types: Decimal, *Decimal, int/int8/int16/int32/int64,
// uint/uint8/uint16/uint32/uint64, float32, float64, string, []byte,
// bool (true=1, false=0), and fmt.Stringer.
// Returns an error for unsupported types.
func NewFromAny(v any) (Decimal, error) {
	switch val := v.(type) {
	case Decimal:
		return val, nil
	case *Decimal:
		if val == nil {
			return Zero, nil
		}

		return *val, nil

	case int:
		return NewFromInt(int64(val)), nil
	case int8:
		return NewFromInt(int64(val)), nil
	case int16:
		return NewFromInt(int64(val)), nil
	case int32:
		return NewFromInt(int64(val)), nil
	case int64:
		return NewFromInt(val), nil

	case uint:
		return NewFromUint64(uint64(val)), nil
	case uint8:
		return NewFromUint64(uint64(val)), nil
	case uint16:
		return NewFromUint64(uint64(val)), nil
	case uint32:
		return NewFromUint64(uint64(val)), nil
	case uint64:
		return NewFromUint64(val), nil

	case float32:
		return NewFromFloat32(val), nil
	case float64:
		return NewFromFloat(val), nil

	case string:
		return NewFromString(val)
	case []byte:
		return NewFromString(string(val))

	case bool:
		if val {
			return One, nil
		}

		return Zero, nil

	default:
		if s, ok := v.(fmt.Stringer); ok {
			return NewFromString(s.String())
		}

		return Zero, fmt.Errorf("%w: %T", errUnsupportedType, v)
	}
}

// MustFromAny is like NewFromAny but panics if the value cannot be converted.
func MustFromAny(v any) Decimal {
	d, err := NewFromAny(v)
	if err != nil {
		panic(err)
	}

	return d
}
