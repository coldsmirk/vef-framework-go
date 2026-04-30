package tabular

import (
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ValueParser defines the interface for custom value parsers.
// Parsers convert cell strings to Go values during import.
type ValueParser interface {
	// Parse converts a cell string to a Go value
	Parse(cellValue string, targetType reflect.Type) (any, error)
}

// ParserFunc adapts a plain function to the ValueParser interface.
type ParserFunc func(cellValue string, targetType reflect.Type) (any, error)

// Parse calls the wrapped function.
func (p ParserFunc) Parse(cellValue string, targetType reflect.Type) (any, error) {
	return p(cellValue, targetType)
}

var (
	// Cached reflect types for performance.
	typeTime      = reflect.TypeFor[time.Time]()
	typeDateTime  = reflect.TypeFor[timex.DateTime]()
	typeDate      = reflect.TypeFor[timex.Date]()
	typeTimexTime = reflect.TypeFor[timex.Time]()
	typeDecimal   = reflect.TypeFor[decimal.Decimal]()
)

// defaultParser is the built-in parser that handles common Go types.
type defaultParser struct {
	format string
}

// Parse implements the ValueParser interface for common Go types.
func (p *defaultParser) Parse(cellValue string, targetType reflect.Type) (any, error) {
	if cellValue == "" {
		return reflect.Zero(targetType).Interface(), nil
	}

	if targetType.Kind() == reflect.Pointer {
		elemType := targetType.Elem()

		value, err := p.parseValue(cellValue, elemType)
		if err != nil {
			return nil, err
		}

		ptr := reflect.New(elemType)
		ptr.Elem().Set(reflect.ValueOf(value))

		return ptr.Interface(), nil
	}

	return p.parseValue(cellValue, targetType)
}

// parseValue parses the cell value to the target type.
func (p *defaultParser) parseValue(cellValue string, targetType reflect.Type) (any, error) {
	if value, ok, err := p.parseStructType(cellValue, targetType); ok {
		return value, err
	}

	return p.parseBasicType(cellValue, targetType)
}

// parseStructType handles struct types like time.Time and decimal.Decimal.
func (p *defaultParser) parseStructType(cellValue string, targetType reflect.Type) (any, bool, error) {
	if targetType.Kind() != reflect.Struct {
		return nil, false, nil
	}

	switch targetType {
	case typeTime:
		format := p.format
		if format == "" {
			format = time.DateTime
		}

		v, err := time.ParseInLocation(format, cellValue, time.Local)

		return v, true, err

	case typeDateTime:
		format := p.format
		if format == "" {
			format = time.DateTime
		}

		v, err := time.ParseInLocation(format, cellValue, time.Local)

		return timex.DateTime(v), true, err

	case typeDate:
		format := p.format
		if format == "" {
			format = time.DateOnly
		}

		v, err := time.ParseInLocation(format, cellValue, time.Local)

		return timex.Date(v), true, err

	case typeTimexTime:
		format := p.format
		if format == "" {
			format = time.TimeOnly
		}

		v, err := time.ParseInLocation(format, cellValue, time.Local)

		return timex.Time(v), true, err

	case typeDecimal:
		v, err := decimal.NewFromString(cellValue)

		return v, true, err

	default:
		return nil, false, nil
	}
}

// parseBasicType handles basic Go types by kind.
func (*defaultParser) parseBasicType(cellValue string, targetType reflect.Type) (any, error) {
	switch targetType.Kind() {
	case reflect.String:
		return cellValue, nil
	case reflect.Int:
		return cast.ToIntE(cellValue)
	case reflect.Int8:
		return cast.ToInt8E(cellValue)
	case reflect.Int16:
		return cast.ToInt16E(cellValue)
	case reflect.Int32:
		return cast.ToInt32E(cellValue)
	case reflect.Int64:
		return cast.ToInt64E(cellValue)
	case reflect.Uint:
		return cast.ToUintE(cellValue)
	case reflect.Uint8:
		return cast.ToUint8E(cellValue)
	case reflect.Uint16:
		return cast.ToUint16E(cellValue)
	case reflect.Uint32:
		return cast.ToUint32E(cellValue)
	case reflect.Uint64:
		return cast.ToUint64E(cellValue)
	case reflect.Float32:
		return cast.ToFloat32E(cellValue)
	case reflect.Float64:
		return cast.ToFloat64E(cellValue)
	case reflect.Bool:
		return cast.ToBoolE(cellValue)
	default:
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedType, targetType)
	}
}

// NewDefaultParser creates a default parser with optional format template.
func NewDefaultParser(format string) ValueParser {
	return &defaultParser{format: format}
}
