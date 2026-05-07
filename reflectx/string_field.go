package reflectx

import "reflect"

// IsStringType reports whether t is string or *string.
func IsStringType(t reflect.Type) bool {
	if t.Kind() == reflect.String {
		return true
	}

	return t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.String
}

// IsStringSliceType reports whether t is []string.
func IsStringSliceType(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.String
}

// IsStringMapType reports whether t is map[string]string.
func IsStringMapType(t reflect.Type) bool {
	return t.Kind() == reflect.Map &&
		t.Key().Kind() == reflect.String &&
		t.Elem().Kind() == reflect.String
}

// GetStringValue reads the string from v whose type is string or *string.
// The boolean is false when v is a nil *string or v is of an incompatible type.
func GetStringValue(v reflect.Value) (string, bool) {
	if !IsStringType(v.Type()) {
		return "", false
	}

	if v.Kind() == reflect.String {
		return v.String(), true
	}

	if v.IsNil() {
		return "", false
	}

	return v.Elem().String(), true
}

// SetStringValue writes s into v whose type is string or *string.
// For *string, a fresh pointer is allocated rather than mutating the existing
// pointee, so other holders of the old pointer are not affected.
func SetStringValue(v reflect.Value, s string) {
	if !IsStringType(v.Type()) {
		return
	}

	if v.Kind() == reflect.String {
		v.SetString(s)

		return
	}

	strValue := s
	v.Set(reflect.ValueOf(&strValue))
}

// GetStringSliceValue reads []string from v.
// The boolean is false when v is a nil slice or v is of an incompatible type.
func GetStringSliceValue(v reflect.Value) ([]string, bool) {
	if !IsStringSliceType(v.Type()) {
		return nil, false
	}

	if v.IsNil() {
		return nil, false
	}

	return v.Interface().([]string), true
}

// SetStringSliceValue writes s into v whose type is []string.
func SetStringSliceValue(v reflect.Value, s []string) {
	if !IsStringSliceType(v.Type()) {
		return
	}

	v.Set(reflect.ValueOf(s))
}

// GetStringMapValue reads map[string]string from v.
// The boolean is false when v is a nil map or v is of an incompatible type.
func GetStringMapValue(v reflect.Value) (map[string]string, bool) {
	if !IsStringMapType(v.Type()) {
		return nil, false
	}

	if v.IsNil() {
		return nil, false
	}

	return v.Interface().(map[string]string), true
}

// SetStringMapValue writes m into v whose type is map[string]string.
func SetStringMapValue(v reflect.Value, m map[string]string) {
	if !IsStringMapType(v.Type()) {
		return
	}

	v.Set(reflect.ValueOf(m))
}
