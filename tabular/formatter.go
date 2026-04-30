package tabular

import (
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// Formatter defines the interface for custom value formatters.
// Formatters convert Go values to cell strings during export.
type Formatter interface {
	// Format converts a Go value to a cell string
	Format(value any) (string, error)
}

// FormatterFunc adapts a plain function to the Formatter interface.
type FormatterFunc func(value any) (string, error)

// Format calls the wrapped function.
func (f FormatterFunc) Format(value any) (string, error) {
	return f(value)
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
