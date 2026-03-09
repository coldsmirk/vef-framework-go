package mold

import (
	"reflect"
)

// extractType gets the actual underlying type of field value.
func (t *MoldTransformer) extractType(current reflect.Value) (reflect.Value, reflect.Kind) {
	switch current.Kind() {
	case reflect.Pointer, reflect.Interface:
		if current.IsNil() {
			return current, current.Kind()
		}

		return t.extractType(current.Elem())

	default:
		if fn := t.interceptors[current.Type()]; fn != nil {
			return t.extractType(fn(current))
		}

		return current, current.Kind()
	}
}

// hasValue determines if a reflect.Value is its default value.
func hasValue(field reflect.Value) bool {
	switch field.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface, reflect.Chan, reflect.Func:
		return !field.IsNil()
	default:
		return field.IsValid() && field.Interface() != reflect.Zero(field.Type()).Interface()
	}
}
