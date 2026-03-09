package mapx

import (
	"encoding/json"
	"mime/multipart"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// nullTypeMapping holds the type relationships for null type conversions.
type nullTypeMapping struct {
	nullType reflect.Type
	baseType reflect.Type
	ptrType  reflect.Type
}

var (
	// Type mappings for all null types.
	nullTypeMappings = []nullTypeMapping{
		{reflect.TypeFor[null.Bool](), reflect.TypeFor[bool](), reflect.TypeFor[*bool]()},
		{reflect.TypeFor[null.String](), reflect.TypeFor[string](), reflect.TypeFor[*string]()},
		{reflect.TypeFor[null.Int](), reflect.TypeFor[int64](), reflect.TypeFor[*int64]()},
		{reflect.TypeFor[null.Int16](), reflect.TypeFor[int16](), reflect.TypeFor[*int16]()},
		{reflect.TypeFor[null.Int32](), reflect.TypeFor[int32](), reflect.TypeFor[*int32]()},
		{reflect.TypeFor[null.Float](), reflect.TypeFor[float64](), reflect.TypeFor[*float64]()},
		{reflect.TypeFor[null.Byte](), reflect.TypeFor[byte](), reflect.TypeFor[*byte]()},
		{reflect.TypeFor[null.DateTime](), reflect.TypeFor[timex.DateTime](), reflect.TypeFor[*timex.DateTime]()},
		{reflect.TypeFor[null.Date](), reflect.TypeFor[timex.Date](), reflect.TypeFor[*timex.Date]()},
		{reflect.TypeFor[null.Time](), reflect.TypeFor[timex.Time](), reflect.TypeFor[*timex.Time]()},
		{reflect.TypeFor[null.Decimal](), reflect.TypeFor[decimal.Decimal](), reflect.TypeFor[*decimal.Decimal]()},
	}

	// Null.Value method index for reflection calls.
	valueOrZeroMethodIndex int

	// Multipart.FileHeader types.
	jsonRawMessageType     = reflect.TypeFor[json.RawMessage]()
	fileHeaderPtrType      = reflect.TypeFor[*multipart.FileHeader]()
	fileHeaderPtrSliceType = reflect.TypeFor[[]*multipart.FileHeader]()
)

func init() {
	method, _ := reflect.TypeFor[null.Value[any]]().MethodByName("ValueOrZero")
	valueOrZeroMethodIndex = method.Index
}

// convertNullBool handles bidirectional conversion between bool types and null.Bool.
func convertNullBool(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[0], from, to, value,
		func(v bool) any { return null.BoolFrom(v) },
		func(v *bool) any { return null.BoolFromPtr(v) },
		func(v null.Bool) any { return v.ValueOrZero() },
		func(v null.Bool) any { return v.Ptr() },
	)
}

// convertNullString handles bidirectional conversion between string types and null.String.
func convertNullString(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[1], from, to, value,
		func(v string) any { return null.StringFrom(v) },
		func(v *string) any { return null.StringFromPtr(v) },
		func(v null.String) any { return v.ValueOrZero() },
		func(v null.String) any { return v.Ptr() },
	)
}

// convertNullInt handles bidirectional conversion between int64 types and null.Int.
func convertNullInt(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[2], from, to, value,
		func(v int64) any { return null.IntFrom(v) },
		func(v *int64) any { return null.IntFromPtr(v) },
		func(v null.Int) any { return v.ValueOrZero() },
		func(v null.Int) any { return v.Ptr() },
	)
}

// convertNullInt16 handles bidirectional conversion between int16 types and null.Int16.
func convertNullInt16(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[3], from, to, value,
		func(v int16) any { return null.Int16From(v) },
		func(v *int16) any { return null.Int16FromPtr(v) },
		func(v null.Int16) any { return v.ValueOrZero() },
		func(v null.Int16) any { return v.Ptr() },
	)
}

// convertNullInt32 handles bidirectional conversion between int32 types and null.Int32.
func convertNullInt32(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[4], from, to, value,
		func(v int32) any { return null.Int32From(v) },
		func(v *int32) any { return null.Int32FromPtr(v) },
		func(v null.Int32) any { return v.ValueOrZero() },
		func(v null.Int32) any { return v.Ptr() },
	)
}

// convertNullFloat handles bidirectional conversion between float64 types and null.Float.
func convertNullFloat(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[5], from, to, value,
		func(v float64) any { return null.FloatFrom(v) },
		func(v *float64) any { return null.FloatFromPtr(v) },
		func(v null.Float) any { return v.ValueOrZero() },
		func(v null.Float) any { return v.Ptr() },
	)
}

// convertNullByte handles bidirectional conversion between byte types and null.Byte.
func convertNullByte(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[6], from, to, value,
		func(v byte) any { return null.ByteFrom(v) },
		func(v *byte) any { return null.ByteFromPtr(v) },
		func(v null.Byte) any { return v.ValueOrZero() },
		func(v null.Byte) any { return v.Ptr() },
	)
}

// convertNullDateTime handles bidirectional conversion between timex.DateTime types and null.DateTime.
func convertNullDateTime(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[7], from, to, value,
		func(v timex.DateTime) any { return null.DateTimeFrom(v) },
		func(v *timex.DateTime) any { return null.DateTimeFromPtr(v) },
		func(v null.DateTime) any { return v.ValueOrZero() },
		func(v null.DateTime) any { return v.Ptr() },
	)
}

// convertNullDate handles bidirectional conversion between timex.Date types and null.Date.
func convertNullDate(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[8], from, to, value,
		func(v timex.Date) any { return null.DateFrom(v) },
		func(v *timex.Date) any { return null.DateFromPtr(v) },
		func(v null.Date) any { return v.ValueOrZero() },
		func(v null.Date) any { return v.Ptr() },
	)
}

// convertNullTime handles bidirectional conversion between timex.Time types and null.Time.
func convertNullTime(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[9], from, to, value,
		func(v timex.Time) any { return null.TimeFrom(v) },
		func(v *timex.Time) any { return null.TimeFromPtr(v) },
		func(v null.Time) any { return v.ValueOrZero() },
		func(v null.Time) any { return v.Ptr() },
	)
}

// convertNullDecimal handles bidirectional conversion between decimal.Decimal types and null.Decimal.
func convertNullDecimal(from, to reflect.Type, value any) (any, error) {
	return convertNullType(nullTypeMappings[10], from, to, value,
		func(v decimal.Decimal) any { return null.DecimalFrom(v) },
		func(v *decimal.Decimal) any { return null.DecimalFromPtr(v) },
		func(v null.Decimal) any { return v.ValueOrZero() },
		func(v null.Decimal) any { return v.Ptr() },
	)
}

// convertNullType is a generic helper for bidirectional null type conversions.
func convertNullType[T any, P *T, N any](
	mapping nullTypeMapping,
	from, to reflect.Type,
	value any,
	fromBase func(T) any,
	fromPtr func(P) any,
	toBase func(N) any,
	toPtr func(N) any,
) (any, error) {
	// Convert base/ptr type to null type
	if (from == mapping.baseType || from == mapping.ptrType) && to == mapping.nullType {
		if from == mapping.baseType {
			return fromBase(value.(T)), nil
		}

		return fromPtr(value.(P)), nil
	}

	// Convert null type to base/ptr type
	if from == mapping.nullType && (to == mapping.baseType || to == mapping.ptrType) {
		if to == mapping.baseType {
			return toBase(value.(N)), nil
		}

		return toPtr(value.(N)), nil
	}

	return value, nil
}

// convertNullValue handles bidirectional conversion between any type and null.Value[T].
func convertNullValue(from, to reflect.Type, value any) (any, error) {
	if isNullValue(from) {
		method := reflect.ValueOf(value).Method(valueOrZeroMethodIndex)
		if !method.IsValid() {
			return nil, ErrValueOrZeroMethodNotFound
		}

		return method.Call(nil)[0].Interface(), nil
	}

	if isNullValue(to) {
		return null.ValueFrom(value), nil
	}

	return value, nil
}

// isNullValue checks if a reflect.Type is a null.Value generic type.
func isNullValue(t reflect.Type) bool {
	pkgPath := t.PkgPath()
	if pkgPath != "github.com/coldsmirk/vef-framework-go/null" && pkgPath != "github.com/guregu/null/v6" {
		return false
	}

	name := t.Name()

	return len(name) >= 5 && name[:5] == "Value"
}

// convertJSONRawMessage handles conversion of arbitrary data to json.RawMessage.
// When the target type is json.RawMessage ([]byte), it re-marshals the source value to JSON bytes.
func convertJSONRawMessage(_, to reflect.Type, value any) (any, error) {
	if to != jsonRawMessageType {
		return value, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

// convertFileHeader handles conversion from []*multipart.FileHeader to *multipart.FileHeader.
func convertFileHeader(from, to reflect.Type, value any) (any, error) {
	if from == fileHeaderPtrSliceType && to == fileHeaderPtrType {
		if files := value.([]*multipart.FileHeader); len(files) == 1 {
			return files[0], nil
		}
	}

	return value, nil
}
