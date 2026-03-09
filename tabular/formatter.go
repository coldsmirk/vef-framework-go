package tabular

import (
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Formatter defines the interface for custom value formatters.
// Formatters convert Go values to cell strings during export.
type Formatter interface {
	// Format converts a Go value to a cell string
	Format(value any) (string, error)
}

// defaultFormatter is the built-in formatter that handles common Go types.
type defaultFormatter struct {
	format string
}

// Format implements the Formatter interface for common Go types.
func (f *defaultFormatter) Format(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	// Handle null types first - extract underlying value or return empty for invalid
	switch v := value.(type) {
	case null.String:
		return v.ValueOrZero(), nil
	case null.Int:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Int16:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Int32:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Float:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Bool:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Byte:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.DateTime:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Date:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Time:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()

	case null.Decimal:
		if !v.Valid {
			return "", nil
		}

		value = v.ValueOrZero()
	}

	// Handle pointer types
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", nil
		}

		value = rv.Elem().Interface()
	}

	// Handle formatted output for specific types
	if f.format != "" {
		switch v := value.(type) {
		case float32, float64:
			return fmt.Sprintf(f.format, v), nil
		case time.Time:
			return v.Format(f.format), nil
		case timex.DateTime:
			return v.Format(f.format), nil
		case timex.Date:
			return v.Format(f.format), nil
		case timex.Time:
			return v.Format(f.format), nil
		}
	}

	// Default time.Time formatting when no custom format specified
	if v, ok := value.(time.Time); ok {
		return v.Format(time.DateTime), nil
	}

	return cast.ToStringE(value)
}

// NewDefaultFormatter creates a default formatter with optional format template.
func NewDefaultFormatter(format string) Formatter {
	return &defaultFormatter{format: format}
}
