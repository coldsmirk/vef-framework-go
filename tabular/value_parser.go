package tabular

import (
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ValueParser defines the interface for custom value parsers.
// Parsers convert cell strings to Go values during import.
type ValueParser interface {
	// Parse converts a cell string to a Go value
	Parse(cellValue string, targetType reflect.Type) (any, error)
}

var (
	// Cached reflect types for performance.
	typeNullString   = reflect.TypeFor[null.String]()
	typeNullInt      = reflect.TypeFor[null.Int]()
	typeNullInt16    = reflect.TypeFor[null.Int16]()
	typeNullInt32    = reflect.TypeFor[null.Int32]()
	typeNullFloat    = reflect.TypeFor[null.Float]()
	typeNullBool     = reflect.TypeFor[null.Bool]()
	typeNullByte     = reflect.TypeFor[null.Byte]()
	typeNullDateTime = reflect.TypeFor[null.DateTime]()
	typeNullDate     = reflect.TypeFor[null.Date]()
	typeNullTime     = reflect.TypeFor[null.Time]()
	typeNullDecimal  = reflect.TypeFor[null.Decimal]()
	typeTime         = reflect.TypeFor[time.Time]()
	typeDecimal      = reflect.TypeFor[decimal.Decimal]()
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
	if value, ok, err := p.parseNullType(cellValue, targetType); ok {
		return value, err
	}

	if value, ok, err := p.parseStructType(cellValue, targetType); ok {
		return value, err
	}

	return p.parseBasicType(cellValue, targetType)
}

// parseNullType handles all null.* types.
func (p *defaultParser) parseNullType(cellValue string, targetType reflect.Type) (any, bool, error) {
	switch targetType {
	case typeNullString:
		return null.StringFrom(cellValue), true, nil

	case typeNullInt:
		v, err := cast.ToInt64E(cellValue)
		if err != nil {
			return null.Int{}, true, fmt.Errorf("parse int: %w", err)
		}

		return null.IntFrom(v), true, nil

	case typeNullInt16:
		v, err := cast.ToInt16E(cellValue)
		if err != nil {
			return null.Int16{}, true, fmt.Errorf("parse int16: %w", err)
		}

		return null.Int16From(v), true, nil

	case typeNullInt32:
		v, err := cast.ToInt32E(cellValue)
		if err != nil {
			return null.Int32{}, true, fmt.Errorf("parse int32: %w", err)
		}

		return null.Int32From(v), true, nil

	case typeNullFloat:
		v, err := cast.ToFloat64E(cellValue)
		if err != nil {
			return null.Float{}, true, fmt.Errorf("parse float: %w", err)
		}

		return null.FloatFrom(v), true, nil

	case typeNullBool:
		v, err := cast.ToBoolE(cellValue)
		if err != nil {
			return null.Bool{}, true, fmt.Errorf("parse bool: %w", err)
		}

		return null.BoolFrom(v), true, nil

	case typeNullByte:
		v, err := cast.ToUint8E(cellValue)
		if err != nil {
			return null.Byte{}, true, fmt.Errorf("parse byte: %w", err)
		}

		return null.ByteFrom(v), true, nil

	case typeNullDateTime:
		v, err := p.parseTemporalValue(cellValue, time.DateTime)
		if err != nil {
			return null.DateTime{}, true, fmt.Errorf("parse datetime: %w", err)
		}

		return null.DateTimeFrom(timex.DateTime(v)), true, nil

	case typeNullDate:
		v, err := p.parseTemporalValue(cellValue, time.DateOnly)
		if err != nil {
			return null.Date{}, true, fmt.Errorf("parse date: %w", err)
		}

		return null.DateFrom(timex.Date(v)), true, nil

	case typeNullTime:
		v, err := p.parseTemporalValue(cellValue, time.TimeOnly)
		if err != nil {
			return null.Time{}, true, fmt.Errorf("parse time: %w", err)
		}

		return null.TimeFrom(timex.Time(v)), true, nil

	case typeNullDecimal:
		v, err := decimal.NewFromString(cellValue)
		if err != nil {
			return null.Decimal{}, true, fmt.Errorf("parse decimal: %w", err)
		}

		return null.DecimalFrom(v), true, nil

	default:
		return nil, false, nil
	}
}

// parseTemporalValue parses temporal values with format handling.
func (p *defaultParser) parseTemporalValue(cellValue, defaultFormat string) (time.Time, error) {
	format := p.format
	if format == "" {
		format = defaultFormat
	}

	return time.ParseInLocation(format, cellValue, time.Local)
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
