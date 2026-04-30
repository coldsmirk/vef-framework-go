package excel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

type excelPrefixParser struct{}

func (*excelPrefixParser) Parse(cellValue string, _ reflect.Type) (any, error) {
	if cellValue == "" {
		return "", nil
	}

	if len(cellValue) > 4 {
		return cellValue[4:], nil
	}

	return cellValue, nil
}

// TestImporter exercises the Excel importer end to end against struct-typed
// adapters, including all option permutations and error-handling branches.
func TestImporter(t *testing.T) {
	t.Run("ImportFromFile", func(t *testing.T) {
		now := time.Now()
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: now, Status: 1, Remark: new("测试用户1"),
			},
			{
				ID: "2", Name: "李四", Email: "li@example.com", Age: 25, Salary: 8000.75,
				CreatedAt: now, Status: 2, Remark: nil,
			},
		}

		filename := exportToTemp(t, NewExporterFor[ExcelTestUser](), users, "test_import_users_*.xlsx")

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "ImportFromFile should succeed for exporter output")
		assert.Empty(t, importErrors, "Round trip should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		require.Len(t, imported, 2, "Both rows should be imported")

		assert.Equal(t, "1", imported[0].ID, "First row ID should round-trip")
		assert.Equal(t, "张三", imported[0].Name, "First row Name should round-trip")
		assert.Equal(t, 1, imported[0].Status, "First row Status should round-trip")
		require.NotNil(t, imported[0].Remark, "Non-nil Remark should round-trip as non-nil")
		assert.Equal(t, "测试用户1", *imported[0].Remark, "Pointer value should round-trip")
		assert.Nil(t, imported[1].Remark, "Nil Remark should round-trip as nil")
	})

	t.Run("ImportFromReader", func(t *testing.T) {
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: time.Now(), Status: 1,
			},
		}

		exporter := NewExporterFor[ExcelTestUser]()
		buf, err := exporter.Export(users)
		require.NoError(t, err, "Export should succeed for the round-trip fixture")

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.Import(buf)
		require.NoError(t, err, "Import should succeed for an in-memory reader")
		assert.Empty(t, importErrors, "Round trip should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		require.Len(t, imported, 1, "Exactly one row should be imported")
		assert.Equal(t, "张三", imported[0].Name, "Imported Name should match seeded value")
	})

	t.Run("ValidationErrorsCollected", func(t *testing.T) {
		invalidUsers := []ExcelTestUser{
			{ID: "1", Name: "张三", Email: "invalid-email", Age: 200, Salary: 10000},
		}

		filename := exportToTemp(t, NewExporterFor[ExcelTestUser](), invalidUsers, "test_invalid_users_*.xlsx")

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should not return a top-level error for validation failures")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		assert.Empty(t, imported, "Rows that fail validation should not be committed")
		assert.NotEmpty(t, importErrors, "Validation failures should be reported via importErrors")
	})

	t.Run("CustomParserRegistration", func(t *testing.T) {
		type PrefixUser struct {
			ID   string `tabular:"ID,parser=prefix_parser"`
			Name string `tabular:"姓名"`
		}

		filename := buildSheet(t, "test_custom_parser_*.xlsx", func(f *excelize.File) {
			sheet := "Sheet1"
			require.NoError(t, f.SetCellValue(sheet, "A1", "ID"), "Setting header A1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "A2", "PFX:1"), "Setting prefixed ID should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting Name should succeed")
		})

		importer := NewImporterFor[PrefixUser]()
		importer.RegisterParser("prefix_parser", &excelPrefixParser{})

		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should succeed when a custom parser is registered")
		assert.Empty(t, importErrors, "Custom parser should not produce per-row errors")

		imported, ok := importedAny.([]PrefixUser)
		require.True(t, ok, "Result should be []PrefixUser")
		require.Len(t, imported, 1, "One row should be imported")
		assert.Equal(t, "1", imported[0].ID, "Custom parser should strip the 4-char prefix from the cell value")
	})

	t.Run("WithImportSheetName", func(t *testing.T) {
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: time.Now(), Status: 1,
			},
		}

		exporter := NewExporterFor[ExcelTestUser](WithSheetName("用户数据"))
		filename := exportToTemp(t, exporter, users, "test_import_options_*.xlsx")

		importer := NewImporterFor[ExcelTestUser](WithImportSheetName("用户数据"))
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should locate the named sheet")
		assert.Empty(t, importErrors, "Named-sheet import should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		assert.Len(t, imported, 1, "Exactly one row should be imported")
	})

	t.Run("EmptyRowsSkipped", func(t *testing.T) {
		filename := buildSheet(t, "test_empty_rows_*.xlsx", func(f *excelize.File) {
			sheet := "Sheet1"
			require.NoError(t, f.SetCellValue(sheet, "A1", "ID"), "Setting header A1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C1", "邮箱"), "Setting header C1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "A2", "1"), "Setting row 2 ID should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting row 2 Name should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C2", "zhang@example.com"), "Setting row 2 Email should succeed")
			// Row 3 intentionally blank.
			require.NoError(t, f.SetCellValue(sheet, "A4", "2"), "Setting row 4 ID should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B4", "李四"), "Setting row 4 Name should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C4", "li@example.com"), "Setting row 4 Email should succeed")
		})

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should succeed when there are blank rows")
		assert.Empty(t, importErrors, "Blank rows should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		assert.Len(t, imported, 2, "Blank rows should be skipped, leaving the two data rows")
	})

	t.Run("MissingColumnsTolerated", func(t *testing.T) {
		filename := buildSheet(t, "test_missing_columns_*.xlsx", func(f *excelize.File) {
			sheet := "Sheet1"
			require.NoError(t, f.SetCellValue(sheet, "A1", "ID"), "Setting header A1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C1", "邮箱"), "Setting header C1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "D1", "年龄"), "Setting header D1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "A2", "1"), "Setting ID should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting Name should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C2", "zhang@example.com"), "Setting Email should succeed")
			require.NoError(t, f.SetCellValue(sheet, "D2", "30"), "Setting Age should succeed")
		})

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should succeed when only a subset of columns is present")
		assert.Empty(t, importErrors, "Missing optional columns should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		require.Len(t, imported, 1, "One row should be imported")
		assert.Equal(t, 30, imported[0].Age, "Age should parse from the present column")
		assert.Equal(t, 0.0, imported[0].Salary, "Absent Salary column should leave the field at zero")
		assert.Nil(t, imported[0].Remark, "Absent pointer column should remain nil")
	})

	t.Run("InvalidDataReportsErrors", func(t *testing.T) {
		filename := buildSheet(t, "test_invalid_data_*.xlsx", func(f *excelize.File) {
			sheet := "Sheet1"
			require.NoError(t, f.SetCellValue(sheet, "A1", "ID"), "Setting header A1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C1", "邮箱"), "Setting header C1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "D1", "年龄"), "Setting header D1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "A2", "1"), "Setting ID should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting Name should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C2", "invalid-email"), "Setting invalid Email should succeed")
			require.NoError(t, f.SetCellValue(sheet, "D2", "not-a-number"), "Setting invalid Age should succeed")
		})

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should not return a top-level error for parse failures")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		assert.Empty(t, imported, "Row with invalid Age should not be committed")
		assert.NotEmpty(t, importErrors, "Parse failures should surface via importErrors")
	})

	t.Run("LargeFileRoundTrip", func(t *testing.T) {
		count := 1000
		users := make([]ExcelTestUser, count)
		now := time.Now()

		for i := range count {
			users[i] = ExcelTestUser{
				ID: fmt.Sprintf("%d", i+1), Name: fmt.Sprintf("用户%d", i+1),
				Email: fmt.Sprintf("user%d@example.com", i+1), Age: 20 + (i % 50),
				Salary: 5000.0 + float64(i*100), CreatedAt: now, Status: i % 3,
			}
		}

		filename := exportToTemp(t, NewExporterFor[ExcelTestUser](), users, "test_large_*.xlsx")

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "ImportFromFile should succeed for 1k rows")
		assert.Empty(t, importErrors, "Large round trip should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		require.Len(t, imported, count, "All rows should be re-imported")
		assert.Equal(t, fmt.Sprintf("%d", count), imported[count-1].ID, "Last row should round-trip")
	})

	t.Run("RoundTripPreservesAllFields", func(t *testing.T) {
		now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local)
		original := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: now, Status: 1, Remark: new("测试用户1"),
			},
			{
				ID: "2", Name: "李四", Email: "li@example.com", Age: 25, Salary: 8000.75,
				CreatedAt: now, Status: 2, Remark: nil,
			},
		}

		filename := exportToTemp(t, NewExporterFor[ExcelTestUser](), original, "test_roundtrip_*.xlsx")

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "ImportFromFile should succeed for round-trip fixture")
		assert.Empty(t, importErrors, "Round trip should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		require.Len(t, imported, len(original), "Round trip should preserve row count")

		for i := range original {
			assert.Equal(t, original[i].ID, imported[i].ID, "Row %d ID should round-trip", i)
			assert.Equal(t, original[i].Name, imported[i].Name, "Row %d Name should round-trip", i)
			assert.Equal(t, original[i].Email, imported[i].Email, "Row %d Email should round-trip", i)
			assert.Equal(t, original[i].Age, imported[i].Age, "Row %d Age should round-trip", i)
			assert.InDelta(t, original[i].Salary, imported[i].Salary, 0.01, "Row %d Salary should round-trip within delta", i)
			assert.Equal(t, original[i].Status, imported[i].Status, "Row %d Status should round-trip", i)

			if original[i].Remark != nil {
				require.NotNil(t, imported[i].Remark, "Row %d Remark should round-trip as non-nil", i)
				assert.Equal(t, *original[i].Remark, *imported[i].Remark, "Row %d Remark value should round-trip", i)
			} else {
				assert.Nil(t, imported[i].Remark, "Row %d nil Remark should round-trip as nil", i)
			}
		}
	})

	t.Run("RoundTripWithoutTags", func(t *testing.T) {
		data := []ExcelNoTagStruct{
			{ID: "1", Name: "Alice", Age: 30},
			{ID: "2", Name: "Bob", Age: 25},
		}

		filename := exportToTemp(t, NewExporterFor[ExcelNoTagStruct](), data, "test_no_tags_*.xlsx")

		importer := NewImporterFor[ExcelNoTagStruct]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should succeed for tag-less round trip")
		assert.Empty(t, importErrors, "Tag-less round trip should not produce per-row errors")

		imported, ok := importedAny.([]ExcelNoTagStruct)
		require.True(t, ok, "Result should be []ExcelNoTagStruct")
		require.Len(t, imported, 2, "Both rows should round-trip")
		assert.Equal(t, "Alice", imported[0].Name, "Tag-less Name field should round-trip")
	})
}

// TestMapImporter covers the dynamic []map[string]any importer path including
// validation, parsing, and adapter-level options.
func TestMapImporter(t *testing.T) {
	t.Run("RoundTripWithExporter", func(t *testing.T) {
		exp, err := NewMapExporter(baseDynamicSpecs(), WithSheetName("Users"))
		require.NoError(t, err, "NewMapExporter should accept valid specs")

		imp, err := NewMapImporter(baseDynamicSpecs(), nil, WithImportSheetName("Users"))
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
		assert.Empty(t, importErrors, "Round trip should produce no per-row errors")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Dynamic importer should return []map[string]any")
		require.Len(t, imported, 2, "Both rows should be imported")

		assert.Equal(t, 1, imported[0]["id"], "id should be parsed as int")
		assert.Equal(t, "张三", imported[0]["name"], "name should round-trip")
		assert.Equal(t, true, imported[0]["active"], "active should be parsed as bool")
		assert.Equal(t, birthday, imported[0]["birthday"], "birthday should round-trip via Format")
	})

	t.Run("RequiredMissing", func(t *testing.T) {
		filename := buildSheet(t, "dynamic_required_*.xlsx", func(f *excelize.File) {
			sheet := "Sheet1"
			require.NoError(t, f.SetCellValue(sheet, "A1", "用户ID"), "Setting header A1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")
			// Row 2: empty id, valid name.
			require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting name should succeed")
		})

		imp, err := NewMapImporter(baseDynamicSpecs(), nil)
		require.NoError(t, err, "NewMapImporter should accept valid specs")

		result, importErrors, err := imp.ImportFromFile(filename)
		require.NoError(t, err, "Import should not return a top-level error")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Import should return map rows")
		assert.Empty(t, imported, "Row missing required id should be rejected")
		require.NotEmpty(t, importErrors, "Required miss should be reported")
		assert.ErrorIs(t, importErrors[0], tabular.ErrRequiredMissing,
			"ImportError should wrap ErrRequiredMissing")
	})

	t.Run("CustomFormatterAndParser", func(t *testing.T) {
		formatter := tabular.FormatterFunc(func(v any) (string, error) {
			if v == nil {
				return "", nil
			}

			return "prefix:" + v.(string), nil
		})

		parser := tabular.ParserFunc(func(s string, _ reflect.Type) (any, error) {
			return strings.TrimPrefix(s, "prefix:"), nil
		})

		specs := []tabular.ColumnSpec{
			{Key: "label", Name: "Label", Type: reflect.TypeFor[string](), FormatterFn: formatter, ParserFn: parser},
		}

		exp, err := NewMapExporter(specs)
		require.NoError(t, err, "NewMapExporter should accept valid specs")

		imp, err := NewMapImporter(specs, nil)
		require.NoError(t, err, "NewMapImporter should accept valid specs")

		buf, err := exp.Export([]map[string]any{{"label": "hello"}})
		require.NoError(t, err, "Export should succeed")

		result, importErrors, err := imp.Import(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err, "Import should succeed")
		assert.Empty(t, importErrors, "Import should produce no per-row errors")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Import should return map rows")
		require.Len(t, imported, 1, "One row should be imported")
		assert.Equal(t, "hello", imported[0]["label"], "ParserFn should strip the prefix")
	})

	t.Run("IgnoresUnknownAndMissingColumns", func(t *testing.T) {
		filename := buildSheet(t, "dynamic_missing_*.xlsx", func(f *excelize.File) {
			sheet := "Sheet1"
			require.NoError(t, f.SetCellValue(sheet, "A1", "用户ID"), "Setting header A1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B1", "姓名"), "Setting header B1 should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C1", "Extra"), "Setting Extra header should succeed")
			require.NoError(t, f.SetCellValue(sheet, "A2", "1"), "Setting id value should succeed")
			require.NoError(t, f.SetCellValue(sheet, "B2", "张三"), "Setting name value should succeed")
			require.NoError(t, f.SetCellValue(sheet, "C2", "ignored"), "Setting Extra value should succeed")
		})

		imp, err := NewMapImporter(baseDynamicSpecs(), nil)
		require.NoError(t, err, "NewMapImporter should accept valid specs")

		result, importErrors, err := imp.ImportFromFile(filename)
		require.NoError(t, err, "Import should succeed despite missing / extra columns")
		assert.Empty(t, importErrors, "No per-row errors should be produced")

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
	})

	t.Run("RowValidatorReportsError", func(t *testing.T) {
		exp, err := NewMapExporter(baseDynamicSpecs())
		require.NoError(t, err, "NewMapExporter should accept valid specs")

		buf, err := exp.Export([]map[string]any{
			{"id": 1, "name": "BAD", "birthday": time.Now(), "active": true},
		})
		require.NoError(t, err, "Export should succeed")

		imp, err := NewMapImporter(
			baseDynamicSpecs(),
			[]tabular.MapOption{tabular.WithRowValidator(func(row map[string]any) error {
				if row["name"] == "BAD" {
					return errors.New("blocked name")
				}

				return nil
			})},
		)
		require.NoError(t, err, "NewMapImporter should accept valid specs")

		result, importErrors, err := imp.Import(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err, "Import should not return a top-level error")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Import should return map rows")
		assert.Empty(t, imported, "Row rejected by validator should not be committed")
		require.NotEmpty(t, importErrors, "Validator failure should be reported")
		assert.Contains(t, importErrors[0].Error(), "blocked name",
			"ImportError should include the validator's message")
	})
}

// exportToTemp writes the exporter output to a fresh temp file scoped to the
// test, returning the resulting file path.
func exportToTemp[T any](t *testing.T, exporter tabular.Exporter, rows []T, pattern string) string {
	t.Helper()

	tmp, err := os.CreateTemp(t.TempDir(), pattern)
	require.NoError(t, err, "CreateTemp should succeed for pattern %q", pattern)

	filename := tmp.Name()
	require.NoError(t, tmp.Close(), "Closing temp file should succeed")

	require.NoError(t, exporter.ExportToFile(rows, filename), "ExportToFile should succeed")

	return filename
}

// buildSheet constructs an Excel workbook via the supplied callback and saves
// it to a fresh temp file scoped to the test, returning the resulting path.
func buildSheet(t *testing.T, pattern string, build func(*excelize.File)) string {
	t.Helper()

	tmp, err := os.CreateTemp(t.TempDir(), pattern)
	require.NoError(t, err, "CreateTemp should succeed for pattern %q", pattern)

	filename := tmp.Name()
	require.NoError(t, tmp.Close(), "Closing temp file should succeed")

	f := excelize.NewFile()
	t.Cleanup(func() {
		_ = f.Close()
	})

	build(f)
	require.NoError(t, f.SaveAs(filename), "Saving the workbook should succeed")

	return filename
}
