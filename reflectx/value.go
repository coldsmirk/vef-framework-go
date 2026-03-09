package reflectx

import (
	"reflect"
	"strings"
)

// IsEmpty checks if a value is considered empty.
// Returns true for: nil, zero values of basic types (0, "", false),
// empty collections (len == 0), and nil pointers/interfaces/channels/functions.
// Special case: *string is dereferenced — a pointer to an empty string is considered empty.
func IsEmpty(value any) bool {
	if value == nil {
		return true
	}

	rv := toValue(value)
	if !rv.IsValid() {
		return true
	}

	return isEmpty(rv)
}

// IsNotEmpty returns true if the value is not empty.
func IsNotEmpty(value any) bool {
	return !IsEmpty(value)
}

// IsNumeric checks if a value is a numeric type (signed integer, unsigned integer, or float).
func IsNumeric(value any) bool {
	return kindMatches(value, isSignedInt, isUnsignedInt, isFloat)
}

// IsInteger checks if a value is any integer type (signed or unsigned).
func IsInteger(value any) bool {
	return kindMatches(value, isSignedInt, isUnsignedInt)
}

// IsSignedInt checks if a value is a signed integer type (int, int8, int16, int32, int64).
func IsSignedInt(value any) bool {
	return kindMatches(value, isSignedInt)
}

// IsUnsignedInt checks if a value is an unsigned integer type (uint, uint8, uint16, uint32, uint64, uintptr).
func IsUnsignedInt(value any) bool {
	return kindMatches(value, isUnsignedInt)
}

// IsFloat checks if a value is a floating-point type (float32, float64).
func IsFloat(value any) bool {
	return kindMatches(value, isFloat)
}

// Equal performs a shallow comparison of two values.
// Numeric types of different kinds are compared within three categories:
// signed integers (int64), unsigned integers (uint64), and floats (float64).
// Nil values are considered equal regardless of type.
func Equal(a, b any) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	va, vb := toValue(a), toValue(b)

	if !va.IsValid() && !vb.IsValid() {
		return true
	}

	if !va.IsValid() || !vb.IsValid() {
		return false
	}

	ka, kb := va.Kind(), vb.Kind()

	if isSignedInt(ka) && isSignedInt(kb) {
		return va.Int() == vb.Int()
	}

	if isUnsignedInt(ka) && isUnsignedInt(kb) {
		return va.Uint() == vb.Uint()
	}

	if isFloat(ka) && isFloat(kb) {
		return va.Float() == vb.Float()
	}

	if va.Type() != vb.Type() {
		return false
	}

	if va.Type().Comparable() {
		return va.Interface() == vb.Interface()
	}

	// Same type, non-comparable: only equal if both nil
	if canNil(ka) {
		return va.IsNil() && vb.IsNil()
	}

	return false
}

// Contains checks if a collection contains the given element.
// For slices and arrays: checks if any element is Equal to the target.
// For maps: checks if the key exists.
// For strings: checks if the element (must be a string) is a substring.
func Contains(collection, element any) bool {
	if collection == nil {
		return false
	}

	rv := toValue(collection)
	if !rv.IsValid() {
		return false
	}

	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false
		}

		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		ev := toValue(element)
		if ev.Kind() == reflect.String {
			return strings.Contains(rv.String(), ev.String())
		}

		return false

	case reflect.Slice, reflect.Array:
		for i := range rv.Len() {
			if Equal(rv.Index(i).Interface(), element) {
				return true
			}
		}

		return false

	case reflect.Map:
		ev := toValue(element)
		if !ev.IsValid() {
			return false
		}

		mapKeyType := rv.Type().Key()
		if ev.Type() != mapKeyType {
			if !ev.Type().ConvertibleTo(mapKeyType) {
				return false
			}

			ev = ev.Convert(mapKeyType)
		}

		return rv.MapIndex(ev).IsValid()

	default:
		return false
	}
}

func isEmpty(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			return true
		}

		// *string: dereference and check emptiness
		elem := rv.Elem()
		if elem.Kind() == reflect.String {
			return elem.Len() == 0
		}

		return false

	case reflect.String, reflect.Array:
		return rv.Len() == 0

	case reflect.Bool:
		return !rv.Bool()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0

	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0

	case reflect.Slice, reflect.Map:
		return rv.IsNil() || rv.Len() == 0

	case reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()

	case reflect.Struct:
		return rv.IsZero()

	default:
		return false
	}
}

func toValue(value any) reflect.Value {
	if v, ok := value.(reflect.Value); ok {
		return v
	}

	return reflect.ValueOf(value)
}

func isSignedInt(k reflect.Kind) bool {
	return k >= reflect.Int && k <= reflect.Int64
}

func isUnsignedInt(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}

func isFloat(k reflect.Kind) bool {
	return k == reflect.Float32 || k == reflect.Float64
}

// kindMatches returns true if the value's kind satisfies any of the given predicates.
// Returns false for nil or invalid values.
func kindMatches(value any, predicates ...func(reflect.Kind) bool) bool {
	if value == nil {
		return false
	}

	rv := toValue(value)
	if !rv.IsValid() {
		return false
	}

	k := rv.Kind()
	for _, p := range predicates {
		if p(k) {
			return true
		}
	}

	return false
}

func canNil(k reflect.Kind) bool {
	switch k {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}
