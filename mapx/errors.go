package mapx

import "errors"

var (
	// ErrInvalidToMapValue indicates the value passed to ToMap is not a struct.
	ErrInvalidToMapValue = errors.New("the value of ToMap function must be a struct")
	// ErrInvalidFromMapType indicates the type parameter of FromMap is not a struct.
	ErrInvalidFromMapType = errors.New("the type parameter of FromMap function must be a struct")

	// ErrCollectionSetNilElement indicates a nil element was found while
	// decoding a slice into a collections set.
	ErrCollectionSetNilElement = errors.New("nil element cannot be added to collections set")
	// ErrCollectionSetIncompatibleKind indicates the source value's kind is
	// incompatible with the target set element type (e.g. string into int set).
	ErrCollectionSetIncompatibleKind = errors.New("incompatible source kind for collections set element")
	// ErrCollectionSetOverflow indicates a numeric value would overflow the
	// target set element type.
	ErrCollectionSetOverflow = errors.New("value overflows collections set element type")
	// ErrCollectionSetNonInteger indicates a fractional float cannot be
	// converted into an integer set element without losing information.
	ErrCollectionSetNonInteger = errors.New("non-integer value cannot be converted to integer set element")
	// ErrCollectionSetNotFinite indicates NaN or infinite values cannot be
	// converted into integer set elements.
	ErrCollectionSetNotFinite = errors.New("non-finite value cannot be converted to integer set element")
	// ErrCollectionSetNegative indicates a negative value cannot be converted
	// into an unsigned integer set element.
	ErrCollectionSetNegative = errors.New("negative value cannot be converted to unsigned set element")
	// ErrCollectionSetUnsupportedTarget indicates the target type kind has no
	// registered conversion strategy.
	ErrCollectionSetUnsupportedTarget = errors.New("unsupported target kind for collections set")

	// ErrJSONNumberNotInteger indicates a JSON number with a fractional or
	// exponent form cannot be converted to an integer target.
	ErrJSONNumberNotInteger = errors.New("non-integer JSON number cannot be converted to integer target")
	// ErrJSONNumberOverflow indicates a JSON number does not fit the numeric
	// target type.
	ErrJSONNumberOverflow = errors.New("JSON number overflows target numeric type")
)
