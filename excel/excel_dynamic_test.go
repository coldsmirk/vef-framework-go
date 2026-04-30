package excel

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

// baseDynamicSpecs mirrors the sample used in the plan: int id, string name,
// time.Time birthday, bool active.
func baseDynamicSpecs() []tabular.ColumnSpec {
	return []tabular.ColumnSpec{
		{Key: "id", Name: "用户ID", Type: reflect.TypeFor[int](), Required: true, Width: 12},
		{Key: "name", Name: "姓名", Type: reflect.TypeFor[string](), Required: true, Width: 18},
		{Key: "birthday", Name: "生日", Type: reflect.TypeFor[time.Time](), Format: "2006-01-02", Width: 20},
		{Key: "active", Name: "激活", Type: reflect.TypeFor[bool](), Default: "false", Width: 8},
	}
}

// TestDynamicExcelRoundTrip exercises the full export -> import cycle using
// []map[string]any rows and custom sheet name.
func TestDynamicExcelRoundTrip(t *testing.T) {
	exp, err := NewMapExporter(baseDynamicSpecs(), WithSheetName("Users"))
	require.NoError(t, err, "NewMapExporter should accept valid specs")

	imp, err := NewMapImporter(baseDynamicSpecs(), WithImportSheetName("Users"))
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	birthday := time.Date(2000, 1, 15, 0, 0, 0, 0, time.Local)

	rows := []map[string]any{
		{"id": 1, "name": "张三", "birthday": birthday, "active": true},
		{"id": 2, "name": "李四", "birthday": birthday.Add(24 * time.Hour), "active": false},
	}

	buf, err := exp.Export(rows)
	require.NoError(t, err, "Export should succeed for map rows")

	result, importErrors, err := imp.Import(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err, "Import should succeed")
	assert.Empty(t, importErrors, "Round trip should produce no import errors")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Dynamic importer should return []map[string]any")
	require.Len(t, imported, 2, "Both rows should be imported")

	assert.Equal(t, 1, imported[0]["id"], "id should be parsed as int")
	assert.Equal(t, "张三", imported[0]["name"], "name should round-trip")
	assert.Equal(t, true, imported[0]["active"], "active should be parsed as bool")
	assert.Equal(t, birthday, imported[0]["birthday"], "birthday should round-trip via Format")
}

// TestDynamicExcelPropagatesWidth ensures Width specified on ColumnSpec is
// written to the Excel sheet.
func TestDynamicExcelPropagatesWidth(t *testing.T) {
	exp, err := NewMapExporter(baseDynamicSpecs(), WithSheetName("Users"))
	require.NoError(t, err, "NewMapExporter should accept valid specs")

	buf, err := exp.Export([]map[string]any{{"id": 1, "name": "张三"}})
	require.NoError(t, err, "Export should succeed")

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err, "Opening the exported workbook should succeed")

	t.Cleanup(func() {
		_ = f.Close()
	})

	width, err := f.GetColWidth("Users", "A")
	require.NoError(t, err, "Reading column A width should succeed")
	assert.InDelta(t, 12.0, width, 0.1, "Column A should reflect the configured width")

	width, err = f.GetColWidth("Users", "B")
	require.NoError(t, err, "Reading column B width should succeed")
	assert.InDelta(t, 18.0, width, 0.1, "Column B should reflect the configured width")
}

// TestDynamicExcelRequiredMissing verifies Required columns report
// ErrRequiredMissing when the source cell is empty.
func TestDynamicExcelRequiredMissing(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "dynamic_required_*.xlsx")
	require.NoError(t, err, "Creating a temp workbook should succeed")

	require.NoError(t, tmp.Close(), "Closing the placeholder file should succeed")

	f := excelize.NewFile()
	sheet := "Sheet1"

	require.NoError(t, f.SetCellValue(sheet, "A1", "用户ID"), "Setting header A1 should succeed")
	require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")

	// Row 2: empty id, valid name.
	require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting name should succeed")

	require.NoError(t, f.SaveAs(tmp.Name()), "Saving the workbook should succeed")
	require.NoError(t, f.Close(), "Closing the workbook should succeed")

	imp, err := NewMapImporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	result, importErrors, err := imp.ImportFromFile(tmp.Name())
	require.NoError(t, err, "Import should not return a top-level error")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	assert.Empty(t, imported, "Row missing required id should be rejected")
	require.NotEmpty(t, importErrors, "Required miss should be reported")
	assert.ErrorIs(t, importErrors[0], tabular.ErrRequiredMissing,
		"ImportError should wrap ErrRequiredMissing")
}

// TestDynamicExcelRowValidatorReportsError ensures that row validators are
// reported via ImportError.
func TestDynamicExcelRowValidatorReportsError(t *testing.T) {
	exp, err := NewMapExporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapExporter should accept valid specs")

	buf, err := exp.Export([]map[string]any{
		{"id": 1, "name": "BAD", "birthday": time.Now(), "active": true},
	})
	require.NoError(t, err, "Export should succeed")

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

	result, importErrors, err := imp.Import(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err, "Import should not return a top-level error")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	assert.Empty(t, imported, "Row rejected by validator should not be committed")
	require.NotEmpty(t, importErrors, "Validator failure should be reported")
	assert.Contains(t, importErrors[0].Error(), "blocked name", "ImportError should include validator message")
}

// TestDynamicExcelCustomFormatterAndParser verifies FormatterFn / ParserFn
// override the named registry on excel.
func TestDynamicExcelCustomFormatterAndParser(t *testing.T) {
	formatter := tabular.Formatter(FormatterFunc(func(v any) (string, error) {
		if v == nil {
			return "", nil
		}

		return "prefix:" + v.(string), nil
	}))

	parser := tabular.ValueParser(ParserFunc(func(s string, _ reflect.Type) (any, error) {
		if len(s) <= len("prefix:") {
			return s, nil
		}

		return s[len("prefix:"):], nil
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

	result, importErrors, err := imp.Import(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err, "Import should succeed")
	assert.Empty(t, importErrors, "Import should produce no errors")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	require.Len(t, imported, 1, "One row should be imported")
	assert.Equal(t, "hello", imported[0]["label"], "ParserFn should strip the prefix")
}

// TestDynamicExcelIgnoresUnknownAndMissingColumns verifies header tolerance:
// unknown extra columns are dropped, schema columns without headers stay out
// of the resulting map.
func TestDynamicExcelIgnoresUnknownAndMissingColumns(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "dynamic_missing_*.xlsx")
	require.NoError(t, err, "Creating a temp workbook should succeed")

	require.NoError(t, tmp.Close(), "Closing the placeholder file should succeed")

	f := excelize.NewFile()
	sheet := "Sheet1"

	require.NoError(t, f.SetCellValue(sheet, "A1", "用户ID"), "Setting header A1 should succeed")
	require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")
	require.NoError(t, f.SetCellValue(sheet, "C1", "Extra"), "Setting Extra header should succeed")

	require.NoError(t, f.SetCellValue(sheet, "A2", "1"), "Setting id value should succeed")
	require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting name value should succeed")
	require.NoError(t, f.SetCellValue(sheet, "C2", "ignored"), "Setting Extra value should succeed")

	require.NoError(t, f.SaveAs(tmp.Name()), "Saving the workbook should succeed")
	require.NoError(t, f.Close(), "Closing the workbook should succeed")

	imp, err := NewMapImporter(baseDynamicSpecs())
	require.NoError(t, err, "NewMapImporter should accept valid specs")

	result, importErrors, err := imp.ImportFromFile(tmp.Name())
	require.NoError(t, err, "Import should succeed")
	assert.Empty(t, importErrors, "Import should not produce errors")

	imported, ok := result.([]map[string]any)
	require.True(t, ok, "Import should return map rows")
	require.Len(t, imported, 1, "One row should be imported")

	row := imported[0]
	assert.Equal(t, 1, row["id"], "id should be parsed as int")
	assert.Equal(t, "张三", row["name"], "name should round-trip")
	_, hasExtra := row["Extra"]
	assert.False(t, hasExtra, "Unknown header should not leak into the map")

	_, hasBirthday := row["birthday"]
	assert.False(t, hasBirthday, "Missing schema columns should stay out of the map")
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
