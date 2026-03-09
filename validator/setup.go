package validator

import (
	"reflect"

	"github.com/coldsmirk/vef-framework-go/null"
)

func setup() {
	if err := RegisterValidationRules(presetValidationRules...); err != nil {
		panic(err)
	}

	RegisterTypeFunc(
		func(field reflect.Value) any {
			switch v := field.Interface().(type) {
			case null.String:
				return nullValue(v.Valid, v.String)
			case null.Int:
				return nullValue(v.Valid, v.Int64)
			case null.Int16:
				return nullValue(v.Valid, v.Int16)
			case null.Int32:
				return nullValue(v.Valid, v.Int32)
			case null.Float:
				return nullValue(v.Valid, v.Float64)
			case null.Bool:
				return nullValue(v.Valid, v.Bool)
			case null.Byte:
				return nullValue(v.Valid, v.Byte)
			case null.DateTime:
				return nullValue(v.Valid, v.V)
			case null.Date:
				return nullValue(v.Valid, v.V)
			case null.Time:
				return nullValue(v.Valid, v.V)
			case null.Decimal:
				return nullValue(v.Valid, v.Decimal)
			default:
				logger.Warnf("Unsupported null type: %T", field.Interface())

				return nil
			}
		},
		null.String{},
		null.Int{},
		null.Int16{},
		null.Int32{},
		null.Float{},
		null.Bool{},
		null.Byte{},
		null.DateTime{},
		null.Date{},
		null.Time{},
		null.Decimal{},
	)
}

func nullValue[T any](valid bool, value T) any {
	if valid {
		return value
	}

	return nil
}
