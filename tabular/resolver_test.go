package tabular

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MarkerFormatter struct{ tag string }

func (f *MarkerFormatter) Format(any) (string, error) {
	return f.tag, nil
}

type MarkerParser struct{ tag string }

func (p *MarkerParser) Parse(string, reflect.Type) (any, error) {
	return p.tag, nil
}

// TestResolveFormatter validates the precedence rules between FormatterFn,
// the named registry, and the default formatter.
func TestResolveFormatter(t *testing.T) {
	t.Run("PrefersDirectFn", func(t *testing.T) {
		col := &Column{
			FormatterFn: &MarkerFormatter{tag: "direct"},
			Formatter:   "named",
		}
		registry := map[string]Formatter{"named": &MarkerFormatter{tag: "named"}}

		result, err := ResolveFormatter(col, registry).Format(nil)
		require.NoError(t, err, "Format should succeed for stub formatter")
		assert.Equal(t, "direct", result, "FormatterFn should win over the named registry")
	})

	t.Run("FallsBackToNamedRegistry", func(t *testing.T) {
		col := &Column{Formatter: "named"}
		registry := map[string]Formatter{"named": &MarkerFormatter{tag: "named"}}

		result, err := ResolveFormatter(col, registry).Format(nil)
		require.NoError(t, err, "Format should succeed")
		assert.Equal(t, "named", result, "Named registry formatter should be used")
	})

	t.Run("UnknownNameFallsBackToDefault", func(t *testing.T) {
		col := &Column{Formatter: "unknown"}

		result, err := ResolveFormatter(col, nil).Format("hello")
		require.NoError(t, err, "Default formatter should still work when name is unknown")
		assert.Equal(t, "hello", result, "Default formatter should stringify the value")
	})

	t.Run("DefaultsToFormatTemplate", func(t *testing.T) {
		col := &Column{Format: "%.2f"}

		result, err := ResolveFormatter(col, nil).Format(3.14159)
		require.NoError(t, err, "Default formatter with format template should succeed")
		assert.Equal(t, "3.14", result, "Default formatter should apply the Format template")
	})
}

// TestResolveParser mirrors TestResolveFormatter for the parser side.
func TestResolveParser(t *testing.T) {
	t.Run("PrefersDirectFn", func(t *testing.T) {
		col := &Column{
			ParserFn: &MarkerParser{tag: "direct"},
			Parser:   "named",
		}
		registry := map[string]ValueParser{"named": &MarkerParser{tag: "named"}}

		result, err := ResolveParser(col, registry).Parse("", reflect.TypeFor[string]())
		require.NoError(t, err, "Parse should succeed")
		assert.Equal(t, "direct", result, "ParserFn should win over the named registry")
	})

	t.Run("FallsBackToNamedRegistry", func(t *testing.T) {
		col := &Column{Parser: "named"}
		registry := map[string]ValueParser{"named": &MarkerParser{tag: "named"}}

		result, err := ResolveParser(col, registry).Parse("", reflect.TypeFor[string]())
		require.NoError(t, err, "Parse should succeed")
		assert.Equal(t, "named", result, "Named registry parser should be used")
	})

	t.Run("UnknownNameFallsBackToDefault", func(t *testing.T) {
		col := &Column{Parser: "unknown"}

		result, err := ResolveParser(col, nil).Parse("hello", reflect.TypeFor[string]())
		require.NoError(t, err, "Default parser should still work when name is unknown")
		assert.Equal(t, "hello", result, "Default parser should return the raw string")
	})

	t.Run("DefaultsToFormatTemplate", func(t *testing.T) {
		col := &Column{Format: "2006-01-02", Type: reflect.TypeFor[time.Time]()}

		result, err := ResolveParser(col, nil).Parse("2024-01-15", reflect.TypeFor[time.Time]())
		require.NoError(t, err, "Default parser should succeed for time target with format template")

		parsed, ok := result.(time.Time)
		require.True(t, ok, "Result should be time.Time")
		assert.Equal(t, 2024, parsed.Year(), "Parsed year should match the input")
		assert.Equal(t, time.January, parsed.Month(), "Parsed month should match the input")
		assert.Equal(t, 15, parsed.Day(), "Parsed day should match the input")
	})
}
