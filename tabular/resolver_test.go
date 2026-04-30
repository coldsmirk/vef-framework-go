package tabular

import (
	"reflect"
	"testing"

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

// TestResolveFormatterPrefersDirectFn validates precedence rule 1.
func TestResolveFormatterPrefersDirectFn(t *testing.T) {
	col := &Column{
		FormatterFn: &MarkerFormatter{tag: "direct"},
		Formatter:   "named",
	}
	registry := map[string]Formatter{"named": &MarkerFormatter{tag: "named"}}

	result, err := ResolveFormatter(col, registry).Format(nil)
	require.NoError(t, err, "Format should succeed for stub formatter")
	assert.Equal(t, "direct", result, "FormatterFn should win over the named registry")
}

// TestResolveFormatterFallsBackToNamedRegistry validates precedence rule 2.
func TestResolveFormatterFallsBackToNamedRegistry(t *testing.T) {
	col := &Column{Formatter: "named"}
	registry := map[string]Formatter{"named": &MarkerFormatter{tag: "named"}}

	result, err := ResolveFormatter(col, registry).Format(nil)
	require.NoError(t, err, "Format should succeed")
	assert.Equal(t, "named", result, "Named registry formatter should be used")
}

// TestResolveFormatterUnknownNameFallsBackToDefault validates that an unknown
// name still yields the default formatter (not a nil-pointer panic).
func TestResolveFormatterUnknownNameFallsBackToDefault(t *testing.T) {
	col := &Column{Formatter: "unknown"}

	result, err := ResolveFormatter(col, nil).Format("hello")
	require.NoError(t, err, "Default formatter should still work when name is unknown")
	assert.Equal(t, "hello", result, "Default formatter should stringify the value")
}

// TestResolveFormatterDefaultsToFormatTemplate validates precedence rule 3.
func TestResolveFormatterDefaultsToFormatTemplate(t *testing.T) {
	col := &Column{Format: "%.2f"}

	result, err := ResolveFormatter(col, nil).Format(3.14159)
	require.NoError(t, err, "Default formatter with format template should succeed")
	assert.Equal(t, "3.14", result, "Default formatter should apply the Format template")
}

// TestResolveParserPrefersDirectFn mirrors the formatter precedence test for
// parsers.
func TestResolveParserPrefersDirectFn(t *testing.T) {
	col := &Column{
		ParserFn: &MarkerParser{tag: "direct"},
		Parser:   "named",
	}
	registry := map[string]ValueParser{"named": &MarkerParser{tag: "named"}}

	result, err := ResolveParser(col, registry).Parse("", reflect.TypeFor[string]())
	require.NoError(t, err, "Parse should succeed")
	assert.Equal(t, "direct", result, "ParserFn should win over the named registry")
}

// TestResolveParserFallsBackToNamedRegistry mirrors the formatter named
// registry test.
func TestResolveParserFallsBackToNamedRegistry(t *testing.T) {
	col := &Column{Parser: "named"}
	registry := map[string]ValueParser{"named": &MarkerParser{tag: "named"}}

	result, err := ResolveParser(col, registry).Parse("", reflect.TypeFor[string]())
	require.NoError(t, err, "Parse should succeed")
	assert.Equal(t, "named", result, "Named registry parser should be used")
}

// TestResolveParserDefaultsToFormatTemplate checks that the default parser is
// used when no name / instance is set and respects Format.
func TestResolveParserDefaultsToFormatTemplate(t *testing.T) {
	col := &Column{Format: "2006-01-02"}

	result, err := ResolveParser(col, nil).Parse("2024-01-15", reflect.TypeFor[string]())
	require.NoError(t, err, "Default parser should succeed for string target")
	assert.Equal(t, "2024-01-15", result, "Default parser should return the raw string")
}
