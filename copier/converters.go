package copier

import (
	"time"

	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// makeValueToPtrConverter creates a converter from value type to pointer type.
func makeValueToPtrConverter[T any]() TypeConverter {
	return TypeConverter{
		SrcType: lo.Empty[T](),
		DstType: lo.Empty[*T](),
		Fn: func(src any) (any, error) {
			v := src.(T)

			return &v, nil
		},
	}
}

// makePtrToValueConverter creates a converter from pointer type to value type.
// If the pointer is nil, the zero value of T is returned.
func makePtrToValueConverter[T any]() TypeConverter {
	return TypeConverter{
		SrcType: lo.Empty[*T](),
		DstType: lo.Empty[T](),
		Fn: func(src any) (any, error) {
			if p := src.(*T); p != nil {
				return *p, nil
			}

			return lo.Empty[T](), nil
		},
	}
}

var (
	// String converters.
	stringToStringPtrConverter = makeValueToPtrConverter[string]()
	stringPtrToStringConverter = makePtrToValueConverter[string]()

	// Bool converters.
	boolToBoolPtrConverter = makeValueToPtrConverter[bool]()
	boolPtrToBoolConverter = makePtrToValueConverter[bool]()

	// Int converters.
	intToIntPtrConverter = makeValueToPtrConverter[int]()
	intPtrToIntConverter = makePtrToValueConverter[int]()

	// Int8 converters.
	int8ToInt8PtrConverter = makeValueToPtrConverter[int8]()
	int8PtrToInt8Converter = makePtrToValueConverter[int8]()

	// Int16 converters.
	int16ToInt16PtrConverter = makeValueToPtrConverter[int16]()
	int16PtrToInt16Converter = makePtrToValueConverter[int16]()

	// Int32 converters.
	int32ToInt32PtrConverter = makeValueToPtrConverter[int32]()
	int32PtrToInt32Converter = makePtrToValueConverter[int32]()

	// Int64 converters.
	int64ToInt64PtrConverter = makeValueToPtrConverter[int64]()
	int64PtrToInt64Converter = makePtrToValueConverter[int64]()

	// Uint converters.
	uintToUintPtrConverter = makeValueToPtrConverter[uint]()
	uintPtrToUintConverter = makePtrToValueConverter[uint]()

	// Uint8 converters.
	uint8ToUint8PtrConverter = makeValueToPtrConverter[uint8]()
	uint8PtrToUint8Converter = makePtrToValueConverter[uint8]()

	// Uint16 converters.
	uint16ToUint16PtrConverter = makeValueToPtrConverter[uint16]()
	uint16PtrToUint16Converter = makePtrToValueConverter[uint16]()

	// Uint32 converters.
	uint32ToUint32PtrConverter = makeValueToPtrConverter[uint32]()
	uint32PtrToUint32Converter = makePtrToValueConverter[uint32]()

	// Uint64 converters.
	uint64ToUint64PtrConverter = makeValueToPtrConverter[uint64]()
	uint64PtrToUint64Converter = makePtrToValueConverter[uint64]()

	// Float32 converters.
	float32ToFloat32PtrConverter = makeValueToPtrConverter[float32]()
	float32PtrToFloat32Converter = makePtrToValueConverter[float32]()

	// Float64 converters.
	float64ToFloat64PtrConverter = makeValueToPtrConverter[float64]()
	float64PtrToFloat64Converter = makePtrToValueConverter[float64]()

	// Decimal.Decimal converters.
	decimalToDecimalPtrConverter = makeValueToPtrConverter[decimal.Decimal]()
	decimalPtrToDecimalConverter = makePtrToValueConverter[decimal.Decimal]()

	// Time.Time converters.
	timeToTimePtrConverter = makeValueToPtrConverter[time.Time]()
	timePtrToTimeConverter = makePtrToValueConverter[time.Time]()

	// Timex.DateTime converters.
	dateTimeToDateTimePtrConverter = makeValueToPtrConverter[timex.DateTime]()
	dateTimePtrToDateTimeConverter = makePtrToValueConverter[timex.DateTime]()

	// Timex.Date converters.
	dateToDatePtrConverter = makeValueToPtrConverter[timex.Date]()
	datePtrToDateConverter = makePtrToValueConverter[timex.Date]()

	// Timex.Time converters.
	timexTimeToTimexTimePtrConverter = makeValueToPtrConverter[timex.Time]()
	timexTimePtrToTimexTimeConverter = makePtrToValueConverter[timex.Time]()
)
