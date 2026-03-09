package reflectx

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/samber/lo"
)

// Indirect returns the underlying type of pointer type.
func Indirect(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		return t.Elem()
	}

	return t
}

// IsPointerToStruct checks if the given type is a pointer to a struct.
func IsPointerToStruct(t reflect.Type) bool {
	return t != nil && t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct
}

// IsSimilarType checks if two types are similar (identical or same generic base type).
func IsSimilarType(t1, t2 reflect.Type) bool {
	if t1 == t2 {
		return true
	}

	if t1.PkgPath() != t2.PkgPath() {
		return false
	}

	name1, name2 := t1.Name(), t2.Name()
	index1 := strings.IndexByte(name1, '[')
	index2 := strings.IndexByte(name2, '[')

	return index1 > -1 && index2 > -1 && index1 == index2 && name1[:index1] == name2[:index2]
}

// ApplyIfString applies a function to a string value, returning defaultValue for non-strings.
func ApplyIfString[T any](value any, fn func(string) T, defaultValue ...T) T {
	var rv reflect.Value
	if v, ok := value.(reflect.Value); ok {
		rv = reflect.Indirect(v)
	} else {
		rv = reflect.Indirect(reflect.ValueOf(value))
	}

	if rv.Kind() == reflect.String {
		return fn(rv.String())
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return lo.Empty[T]()
}

// FindMethod finds a method on a target value (includes pointer receiver and promoted methods).
func FindMethod(target reflect.Value, name string) reflect.Value {
	if method := target.MethodByName(name); method.IsValid() {
		return method
	}

	if target.Kind() != reflect.Pointer {
		var ptrValue reflect.Value
		if target.CanAddr() {
			ptrValue = target.Addr()
		} else {
			ptrValue = reflect.New(target.Type())
			ptrValue.Elem().Set(target)
		}

		if method := ptrValue.MethodByName(name); method.IsValid() {
			return method
		}
	}

	return reflect.Value{}
}

// CollectMethods collects all methods from a target value as a name-to-value map.
func CollectMethods(target reflect.Value) map[string]reflect.Value {
	methods := make(map[string]reflect.Value)

	for target.Kind() == reflect.Pointer {
		if target.IsNil() {
			return methods
		}

		target = target.Elem()
	}

	if target.Kind() != reflect.Struct {
		return methods
	}

	targetType := target.Type()

	var ptrTarget reflect.Value
	if target.CanAddr() {
		ptrTarget = target.Addr()
	} else {
		ptrTarget = reflect.New(targetType)
		ptrTarget.Elem().Set(target)
	}

	ptrType := ptrTarget.Type()
	for i := range ptrType.NumMethod() {
		method := ptrType.Method(i)
		methods[method.Name] = ptrTarget.Method(i)
	}

	return methods
}

// IsTypeCompatible checks if sourceType is compatible with targetType.
func IsTypeCompatible(sourceType, targetType reflect.Type) bool {
	if sourceType == targetType || sourceType.AssignableTo(targetType) {
		return true
	}

	if targetType.Kind() == reflect.Interface {
		return sourceType.Implements(targetType)
	}

	if targetType.Kind() == reflect.Pointer && sourceType.Kind() == reflect.Pointer {
		return IsTypeCompatible(sourceType.Elem(), targetType.Elem())
	}

	if targetType.Kind() == reflect.Pointer && sourceType.Kind() != reflect.Pointer {
		return sourceType.AssignableTo(targetType.Elem())
	}

	if sourceType.Kind() == reflect.Pointer && targetType.Kind() != reflect.Pointer {
		return sourceType.Elem().AssignableTo(targetType)
	}

	return false
}

// ConvertValue converts a source value to the target type.
func ConvertValue(sourceValue reflect.Value, targetType reflect.Type) (reflect.Value, error) {
	sourceType := sourceValue.Type()

	if sourceType == targetType {
		return sourceValue, nil
	}

	// *T -> T
	if sourceType.Kind() == reflect.Pointer && targetType.Kind() != reflect.Pointer {
		if sourceValue.IsNil() {
			return reflect.Zero(targetType), nil
		}

		if elemValue := sourceValue.Elem(); elemValue.Type().AssignableTo(targetType) {
			return elemValue, nil
		}
	}

	// T -> *T
	if sourceType.Kind() != reflect.Pointer && targetType.Kind() == reflect.Pointer {
		if sourceType.AssignableTo(targetType.Elem()) {
			ptrValue := reflect.New(targetType.Elem())
			ptrValue.Elem().Set(sourceValue)

			return ptrValue, nil
		}
	}

	// *T -> *U
	if sourceType.Kind() == reflect.Pointer && targetType.Kind() == reflect.Pointer {
		if sourceValue.IsNil() {
			return reflect.Zero(targetType), nil
		}

		if elemValue := sourceValue.Elem(); elemValue.Type().AssignableTo(targetType.Elem()) {
			ptrValue := reflect.New(targetType.Elem())
			ptrValue.Elem().Set(elemValue)

			return ptrValue, nil
		}
	}

	if sourceType.AssignableTo(targetType) {
		return sourceValue, nil
	}

	return reflect.Value{}, fmt.Errorf("%w: %s -> %s", ErrCannotConvertType, sourceType, targetType)
}
