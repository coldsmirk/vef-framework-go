package copier

import (
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/null"
)

// Nullable defines the interface for null wrapper types.
type Nullable[T any] interface {
	// ValueOrZero returns the underlying value, or the zero value of T if null.
	ValueOrZero() T
	// Ptr returns a pointer to the underlying value, or nil if null.
	Ptr() *T
}

// makeNullToValueConverter creates a converter from null type to value type.
func makeNullToValueConverter[N Nullable[T], T any]() TypeConverter {
	return TypeConverter{
		SrcType: lo.Empty[N](),
		DstType: lo.Empty[T](),
		Fn: func(src any) (any, error) {
			return src.(N).ValueOrZero(), nil
		},
	}
}

// makeNullToPtrConverter creates a converter from null type to pointer type.
func makeNullToPtrConverter[N Nullable[T], T any]() TypeConverter {
	return TypeConverter{
		SrcType: lo.Empty[N](),
		DstType: lo.Empty[*T](),
		Fn: func(src any) (any, error) {
			return src.(N).Ptr(), nil
		},
	}
}

// makeValueToNullConverter creates a converter from value type to null type.
func makeValueToNullConverter[T, N any](fromFn func(T) N) TypeConverter {
	return TypeConverter{
		SrcType: lo.Empty[T](),
		DstType: lo.Empty[N](),
		Fn: func(src any) (any, error) {
			return fromFn(src.(T)), nil
		},
	}
}

// makePtrToNullConverter creates a converter from pointer type to null type.
func makePtrToNullConverter[T, N any](fromPtrFn func(*T) N) TypeConverter {
	return TypeConverter{
		SrcType: lo.Empty[*T](),
		DstType: lo.Empty[N](),
		Fn: func(src any) (any, error) {
			return fromPtrFn(src.(*T)), nil
		},
	}
}

var (
	// Null.String converters.
	nullStringToStringConverter    = makeNullToValueConverter[null.String]()
	nullStringToStringPtrConverter = makeNullToPtrConverter[null.String]()
	stringToNullStringConverter    = makeValueToNullConverter(null.StringFrom)
	stringPtrToNullStringConverter = makePtrToNullConverter(null.StringFromPtr)

	// Null.Int converters.
	nullIntToIntConverter    = makeNullToValueConverter[null.Int]()
	nullIntToIntPtrConverter = makeNullToPtrConverter[null.Int]()
	intToNullIntConverter    = makeValueToNullConverter(null.IntFrom)
	intPtrToNullIntConverter = makePtrToNullConverter(null.IntFromPtr)

	// Null.Int16 converters.
	nullInt16ToInt16Converter    = makeNullToValueConverter[null.Int16]()
	nullInt16ToInt16PtrConverter = makeNullToPtrConverter[null.Int16]()
	int16ToNullInt16Converter    = makeValueToNullConverter(null.Int16From)
	int16PtrToNullInt16Converter = makePtrToNullConverter(null.Int16FromPtr)

	// Null.Int32 converters.
	nullInt32ToInt32Converter    = makeNullToValueConverter[null.Int32]()
	nullInt32ToInt32PtrConverter = makeNullToPtrConverter[null.Int32]()
	int32ToNullInt32Converter    = makeValueToNullConverter(null.Int32From)
	int32PtrToNullInt32Converter = makePtrToNullConverter(null.Int32FromPtr)

	// Null.Float converters.
	nullFloatToFloatConverter    = makeNullToValueConverter[null.Float]()
	nullFloatToFloatPtrConverter = makeNullToPtrConverter[null.Float]()
	floatToNullFloatConverter    = makeValueToNullConverter(null.FloatFrom)
	floatPtrToNullFloatConverter = makePtrToNullConverter(null.FloatFromPtr)

	// Null.Byte converters.
	nullByteToByteConverter    = makeNullToValueConverter[null.Byte]()
	nullByteToBytePtrConverter = makeNullToPtrConverter[null.Byte]()
	byteToNullByteConverter    = makeValueToNullConverter(null.ByteFrom)
	bytePtrToNullByteConverter = makePtrToNullConverter(null.ByteFromPtr)

	// Null.Bool converters.
	nullBoolToBoolConverter    = makeNullToValueConverter[null.Bool]()
	nullBoolToBoolPtrConverter = makeNullToPtrConverter[null.Bool]()
	boolToNullBoolConverter    = makeValueToNullConverter(null.BoolFrom)
	boolPtrToNullBoolConverter = makePtrToNullConverter(null.BoolFromPtr)

	// Null.DateTime converters.
	nullDateTimeToDateTimeConverter    = makeNullToValueConverter[null.DateTime]()
	nullDateTimeToDateTimePtrConverter = makeNullToPtrConverter[null.DateTime]()
	dateTimeToNullDateTimeConverter    = makeValueToNullConverter(null.DateTimeFrom)
	dateTimePtrToNullDateTimeConverter = makePtrToNullConverter(null.DateTimeFromPtr)

	// Null.Date converters.
	nullDateToDateConverter    = makeNullToValueConverter[null.Date]()
	nullDateToDatePtrConverter = makeNullToPtrConverter[null.Date]()
	dateToNullDateConverter    = makeValueToNullConverter(null.DateFrom)
	datePtrToNullDateConverter = makePtrToNullConverter(null.DateFromPtr)

	// Null.Time converters.
	nullTimeToTimeConverter    = makeNullToValueConverter[null.Time]()
	nullTimeToTimePtrConverter = makeNullToPtrConverter[null.Time]()
	timeToNullTimeConverter    = makeValueToNullConverter(null.TimeFrom)
	timePtrToNullTimeConverter = makePtrToNullConverter(null.TimeFromPtr)

	// Null.Decimal converters.
	nullDecimalToDecimalConverter    = makeNullToValueConverter[null.Decimal]()
	nullDecimalToDecimalPtrConverter = makeNullToPtrConverter[null.Decimal]()
	decimalToNullDecimalConverter    = makeValueToNullConverter(null.DecimalFrom)
	decimalPtrToNullDecimalConverter = makePtrToNullConverter(null.DecimalFromPtr)
)
