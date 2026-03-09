package copier

import "github.com/jinzhu/copier"

// defaultConverters contains all built-in type converters for null types.
var defaultConverters = []TypeConverter{
	// null.String converters
	nullStringToStringConverter,
	nullStringToStringPtrConverter,
	stringToNullStringConverter,
	stringPtrToNullStringConverter,

	// null.Int converters
	nullIntToIntConverter,
	nullIntToIntPtrConverter,
	intToNullIntConverter,
	intPtrToNullIntConverter,

	// null.Int16 converters
	nullInt16ToInt16Converter,
	nullInt16ToInt16PtrConverter,
	int16ToNullInt16Converter,
	int16PtrToNullInt16Converter,

	// null.Int32 converters
	nullInt32ToInt32Converter,
	nullInt32ToInt32PtrConverter,
	int32ToNullInt32Converter,
	int32PtrToNullInt32Converter,

	// null.Float converters
	nullFloatToFloatConverter,
	nullFloatToFloatPtrConverter,
	floatToNullFloatConverter,
	floatPtrToNullFloatConverter,

	// null.Byte converters
	nullByteToByteConverter,
	nullByteToBytePtrConverter,
	byteToNullByteConverter,
	bytePtrToNullByteConverter,

	// null.Bool converters
	nullBoolToBoolConverter,
	nullBoolToBoolPtrConverter,
	boolToNullBoolConverter,
	boolPtrToNullBoolConverter,

	// null.DateTime converters
	nullDateTimeToDateTimeConverter,
	nullDateTimeToDateTimePtrConverter,
	dateTimeToNullDateTimeConverter,
	dateTimePtrToNullDateTimeConverter,

	// null.Date converters
	nullDateToDateConverter,
	nullDateToDatePtrConverter,
	dateToNullDateConverter,
	datePtrToNullDateConverter,

	// null.Time converters
	nullTimeToTimeConverter,
	nullTimeToTimePtrConverter,
	timeToNullTimeConverter,
	timePtrToNullTimeConverter,

	// null.Decimal converters
	nullDecimalToDecimalConverter,
	nullDecimalToDecimalPtrConverter,
	decimalToNullDecimalConverter,
	decimalPtrToNullDecimalConverter,
}

type (
	// CopyOption configures the copy behavior.
	CopyOption func(option *copier.Option)

	// TypeConverter is an alias for copier.TypeConverter.
	TypeConverter = copier.TypeConverter

	// FieldNameMapping is an alias for copier.FieldNameMapping.
	FieldNameMapping = copier.FieldNameMapping
)

// WithIgnoreEmpty skips copying fields with zero values.
func WithIgnoreEmpty() CopyOption {
	return func(option *copier.Option) {
		option.IgnoreEmpty = true
	}
}

// WithDeepCopy enables deep copying of nested structures.
func WithDeepCopy() CopyOption {
	return func(option *copier.Option) {
		option.DeepCopy = true
	}
}

// WithCaseInsensitive enables case-insensitive field name matching.
func WithCaseInsensitive() CopyOption {
	return func(option *copier.Option) {
		option.CaseSensitive = false
	}
}

// WithFieldNameMapping adds custom field name mappings.
func WithFieldNameMapping(mappings ...FieldNameMapping) CopyOption {
	return func(option *copier.Option) {
		option.FieldNameMapping = append(option.FieldNameMapping, mappings...)
	}
}

// WithTypeConverters adds custom type converters.
func WithTypeConverters(converters ...TypeConverter) CopyOption {
	return func(option *copier.Option) {
		option.Converters = append(option.Converters, converters...)
	}
}

// Copy copies fields from src to dst with optional configuration.
// The dst parameter must be a pointer to a struct.
func Copy(src, dst any, options ...CopyOption) error {
	opt := copier.Option{
		CaseSensitive: true,
		Converters:    defaultConverters,
	}
	for _, apply := range options {
		apply(&opt)
	}

	return copier.CopyWithOption(dst, src, opt)
}
