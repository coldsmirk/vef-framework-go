package reflectx

import (
	"reflect"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/decimal"
)

// Common type conversions delegated to cast.
// E suffix versions return errors; non-E versions return zero values on failure.
var (
	ToString  = cast.ToString
	ToStringE = cast.ToStringE

	ToInt    = cast.ToInt
	ToIntE   = cast.ToIntE
	ToInt8   = cast.ToInt8
	ToInt8E  = cast.ToInt8E
	ToInt16  = cast.ToInt16
	ToInt16E = cast.ToInt16E
	ToInt32  = cast.ToInt32
	ToInt32E = cast.ToInt32E
	ToInt64  = cast.ToInt64
	ToInt64E = cast.ToInt64E

	ToUint    = cast.ToUint
	ToUintE   = cast.ToUintE
	ToUint8   = cast.ToUint8
	ToUint8E  = cast.ToUint8E
	ToUint16  = cast.ToUint16
	ToUint16E = cast.ToUint16E
	ToUint32  = cast.ToUint32
	ToUint32E = cast.ToUint32E
	ToUint64  = cast.ToUint64
	ToUint64E = cast.ToUint64E

	ToFloat32  = cast.ToFloat32
	ToFloat32E = cast.ToFloat32E
	ToFloat64  = cast.ToFloat64
	ToFloat64E = cast.ToFloat64E

	ToBool  = cast.ToBool
	ToBoolE = cast.ToBoolE
)

// ToDecimalE converts an arbitrary value to decimal.Decimal.
// Handles nil and pointer/interface dereferencing, then delegates to decimal.NewFromAny.
func ToDecimalE(value any) (decimal.Decimal, error) {
	if value == nil {
		return decimal.Zero, nil
	}

	rv := toValue(value)

	kind := rv.Kind()
	if kind == reflect.Pointer || kind == reflect.Interface {
		if rv.IsNil() {
			return decimal.Zero, nil
		}

		return ToDecimalE(rv.Elem().Interface())
	}

	return decimal.NewFromAny(value)
}

// ToDecimal converts an arbitrary value to decimal.Decimal, returning Zero on failure.
func ToDecimal(value any) decimal.Decimal {
	v, _ := ToDecimalE(value)

	return v
}
