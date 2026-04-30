package csv

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

// baseDynamicSpecs mirrors the sample in the implementation plan: the schema
// covers an int id, a string name, a time birthday, and a bool flag.
func baseDynamicSpecs() []tabular.ColumnSpec {
	return []tabular.ColumnSpec{
		{Key: "id", Name: "用户ID", Type: reflect.TypeFor[int](), Required: true},
		{Key: "name", Name: "姓名", Type: reflect.TypeFor[string](), Required: true},
		{Key: "birthday", Name: "生日", Type: reflect.TypeFor[time.Time](), Format: "2006-01-02"},
		{Key: "active", Name: "激活", Type: reflect.TypeFor[bool](), Default: "false"},
	}
}

// TestDynamicCSVRoundTrip exercises the full export -> import cycle with
// []map[string]any rows.
func TestDynamicCSVRoundTrip(t *testing.T) {
	exp, err := NewMapExporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapExporter should accept valid specs")

	imp, err := NewMapImporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	birthday := time.Date(2000, 1, 15, 0, 0, 0, 0, time.Local)

	rows := []map[string]any{
		{"id": 1, "name": "张三", "birthday": birthday, "active": true},
		{"id": 2, "name": "李四", "birthday": birthday.Add(24 * time.Hour), "active": false},
	}

	buf, err := exp.Export(rows)
	require.NoError(t, err, "Export should succeed for map rows")

	csvContent := buf.String()
	assert.Contains(t, csvContent, "用户ID", "Header should include Chinese names")
	assert.Contains(t, csvContent, "2000-01-15", "Birthday should be formatted via Format template")

	result, importErrors, err := imp.Import(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err, "Import should succeed")
	assert.Empty(t, importErrors, "Round trip should not produce import errors")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Dynamic importer should return []map[string]any")
	require.Len(t, imported, 2, "Both rows should be imported")

	assert.Equal(t, 1, imported[0]["id"], "id should be parsed as int")
	assert.Equal(t, "张三", imported[0]["name"], "name should round-trip")
	assert.Equal(t, true, imported[0]["active"], "active should be parsed as bool")
	assert.Equal(t, birthday, imported[0]["birthday"], "birthday should parse back using the Format template")
}

// TestDynamicCSVRequiredMissing ensures Required specs enforce non-empty cells
// and the error is surfaced via ImportError rather than aborting the import.
func TestDynamicCSVRequiredMissing(t *testing.T) {
	imp, err := NewMapImporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	content := "用户ID,姓名,生日,激活\n,张三,2000-01-15,true\n"

	result, importErrors, err := imp.Import(strings.NewReader(content))
	require.NoError(t, err, "Import should not return a top-level error for validation failures")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return []map[string]any")
	assert.Empty(t, imported, "Row with missing required field should be rejected")
	require.NotEmpty(t, importErrors, "Import should report validation errors")
	assert.ErrorIs(t, importErrors[0], tabular.ErrRequiredMissing,
		"ImportError should wrap ErrRequiredMissing for the missing id column")
}

// TestDynamicCSVCustomFormatterAndParser confirms that FormatterFn / ParserFn
// are honored for both export and import.
func TestDynamicCSVCustomFormatterAndParser(t *testing.T) {
	formatter := tabular.Formatter(FormatterFunc(func(v any) (string, error) {
		if v == nil {
			return "", nil
		}

		return "prefix:" + v.(string), nil
	}))

	parser := tabular.ValueParser(ParserFunc(func(s string, _ reflect.Type) (any, error) {
		return strings.TrimPrefix(s, "prefix:"), nil
	}))

	specs := []tabular.ColumnSpec{
		{Key: "label", Name: "Label", Type: reflect.TypeFor[string](), FormatterFn: formatter, ParserFn: parser},
	}

	exp, err := NewMapExporter(specs)
	require.NoError(t, err, "NewMapExporter should accept valid specs")

	imp, err := NewMapImporter(specs)
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	buf, err := exp.Export([]map[string]any{{"label": "hello"}})
	require.NoError(t, err, "Export should succeed")

	assert.Contains(t, buf.String(), "prefix:hello", "FormatterFn should transform the cell")

	result, importErrors, err := imp.Import(buf)
	require.NoError(t, err, "Import should succeed")
	assert.Empty(t, importErrors, "Import should not produce errors")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	require.Len(t, imported, 1, "Exactly one row should be imported")
	assert.Equal(t, "hello", imported[0]["label"], "ParserFn should strip the prefix")
}

// TestDynamicCSVIgnoresUnknownAndMissingColumns verifies tolerance of missing
// schema columns (cell remains zero) and extra source columns (ignored).
func TestDynamicCSVIgnoresUnknownAndMissingColumns(t *testing.T) {
	imp, err := NewMapImporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	// Header drops birthday/active but includes an unknown Extra column.
	content := "用户ID,姓名,Extra\n1,张三,ignored\n"

	result, importErrors, err := imp.Import(strings.NewReader(content))
	require.NoError(t, err, "Import should succeed despite missing / extra columns")
	assert.Empty(t, importErrors, "No import errors should be produced")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	require.Len(t, imported, 1, "One row should be imported")

	row := imported[0]
	assert.Equal(t, 1, row["id"], "id should parse")
	assert.Equal(t, "张三", row["name"], "name should parse")
	_, hasExtra := row["Extra"]
	assert.False(t, hasExtra, "Unknown columns should not leak into the row")
	// Schema columns not present in the source header stay out of the map.
	_, hasBirthday := row["birthday"]
	assert.False(t, hasBirthday, "Absent schema columns should not appear in the map")

	_, hasActive := row["active"]
	assert.False(t, hasActive, "Defaults only apply to empty cells within mapped columns")
}

// TestDynamicCSVRowValidatorReportsError verifies that injected RowValidator
// failures are surfaced as ImportError entries.
func TestDynamicCSVRowValidatorReportsError(t *testing.T) {
	imp, err := NewMapImporterWithOptions(
		baseDynamicSpecs(),
		[]tabular.MapOption{tabular.WithRowValidator(func(row map[string]any) error {
			if row["name"] == "BAD" {
				return errors.New("blocked name")
			}

			return nil
		})},
	)
	require.NoError(t, err, "NewMapImporterWithOptions should accept valid specs")

	content := "用户ID,姓名,生日,激活\n1,BAD,2000-01-15,true\n"

	result, importErrors, err := imp.Import(strings.NewReader(content))
	require.NoError(t, err, "Import should not return a top-level error")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	assert.Empty(t, imported, "Row rejected by validator should not be committed")
	require.NotEmpty(t, importErrors, "RowValidator failure should surface as ImportError")
	assert.Contains(t, importErrors[0].Error(), "blocked name",
		"ImportError message should include validator message")
}

// TestDynamicCSVMapExporterRejectsBadSpecs documents that spec validation is
// performed eagerly so the caller sees a clear error.
func TestDynamicCSVMapExporterRejectsBadSpecs(t *testing.T) {
	_, err := NewMapExporter([]tabular.ColumnSpec{{Key: "id"}})
	require.Error(t, err, "NewMapExporter should reject specs with missing type")
	assert.ErrorIs(t, err, tabular.ErrMissingColumnType, "Error should wrap ErrMissingColumnType")
}

// TestDynamicCSVParseErrorReportedPerCell ensures each parse failure becomes
// a dedicated ImportError with column / field populated.
func TestDynamicCSVParseErrorReportedPerCell(t *testing.T) {
	imp, err := NewMapImporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	content := "用户ID,姓名,生日,激活\nabc,张三,not-a-date,not-a-bool\n"

	result, importErrors, err := imp.Import(strings.NewReader(content))
	require.NoError(t, err, "Import should not return a top-level error")

	_, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	require.NotEmpty(t, importErrors, "Parse errors should surface per cell")

	columns := map[string]bool{}
	for _, ie := range importErrors {
		columns[ie.Column] = true
	}

	assert.Contains(t, columns, "用户ID", "Invalid id should produce an ImportError")
	assert.Contains(t, columns, "生日", "Invalid birthday should produce an ImportError")
	assert.Contains(t, columns, "激活", "Invalid active flag should produce an ImportError")
}

// FormatterFunc adapts a plain function to the Formatter interface.
type FormatterFunc func(any) (string, error)

// Format calls the wrapped function.
func (f FormatterFunc) Format(value any) (string, error) { return f(value) }

// ParserFunc adapts a plain function to the ValueParser interface.
type ParserFunc func(string, reflect.Type) (any, error)

// Parse calls the wrapped function.
func (p ParserFunc) Parse(cellValue string, targetType reflect.Type) (any, error) {
	return p(cellValue, targetType)
}
