package csv

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

	"github.com/coldsmirk/vef-framework-go/tabular"
)

type ImporterTestUser struct {
	ID       string    `tabular:"用户ID"                 validate:"required"`
	Name     string    `tabular:"姓名"                   validate:"required"`
	Email    string    `tabular:"邮箱"                   validate:"email"`
	Age      int       `tabular:"年龄"                   validate:"gte=0,lte=150"`
	Salary   float64   `tabular:"薪资,format=%.2f"`
	Birthday time.Time `tabular:"生日,format=2006-01-02"`
	Active   bool      `tabular:"激活状态"`
	Remark   *string   `tabular:"备注"`
	Password string    `tabular:"-"` // Ignored field
}

type ImporterSimpleUser struct {
	ID    int    `tabular:"用户ID"`
	Name  string `tabular:"姓名"`
	Email string `tabular:"邮箱"`
}

type ImporterNoTagStruct struct {
	ID   string
	Name string
	Age  int
}

type prefixParser struct{}

func (*prefixParser) Parse(cellValue string, _ reflect.Type) (any, error) {
	if cellValue == "" {
		return "", nil
	}

	if len(cellValue) > 4 {
		return cellValue[4:], nil
	}

	return cellValue, nil
}

// TestImporter exercises the CSV importer end to end against struct-typed
// adapters, including all option permutations and error-handling branches.
func TestImporter(t *testing.T) {
	t.Run("ImportRoundTripWithExporter", func(t *testing.T) {
		users := []ImporterTestUser{
			{ID: "1", Name: "张三", Email: "zhangsan@example.com", Age: 30, Salary: 10000.50,
				Birthday: time.Date(1994, 1, 15, 0, 0, 0, 0, time.UTC), Active: true, Remark: new("测试用户1")},
			{ID: "2", Name: "李四", Email: "lisi@example.com", Age: 25, Salary: 8000.75,
				Birthday: time.Date(1999, 5, 20, 0, 0, 0, 0, time.UTC), Active: false, Remark: nil},
		}

		exporter := NewExporterFor[ImporterTestUser]()
		buf, err := exporter.Export(users)
		require.NoError(t, err, "Export should succeed for valid struct slice")

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.Import(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err, "Import should succeed for exporter output")
		assert.Empty(t, importErrors, "Round trip should not surface any per-row errors")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Import should return []T for struct adapter")
		require.Len(t, imported, 2, "Both exported rows should be re-imported")

		assert.Equal(t, "1", imported[0].ID, "First row ID should round-trip")
		assert.Equal(t, "张三", imported[0].Name, "First row Name should round-trip")
		assert.True(t, imported[0].Active, "Active flag should round-trip as true")
		require.NotNil(t, imported[0].Remark, "Non-nil pointer should round-trip as non-nil")
		assert.Equal(t, "测试用户1", *imported[0].Remark, "Pointer value should round-trip")
		assert.Nil(t, imported[1].Remark, "Nil pointer should round-trip as nil")
	})

	t.Run("CustomDelimiter", func(t *testing.T) {
		csvContent := "用户ID;姓名;邮箱\n1;张三;zhangsan@example.com\n2;李四;lisi@example.com"

		importer := NewImporterFor[ImporterSimpleUser](WithImportDelimiter(';'))
		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should succeed for ';'-delimited input")
		assert.Empty(t, importErrors, "Custom delimiter should not produce per-row errors")

		users, ok := result.([]ImporterSimpleUser)
		require.True(t, ok, "Result should be []ImporterSimpleUser")
		require.Len(t, users, 2, "Both rows should be imported")
		assert.Equal(t, 1, users[0].ID, "First row ID should parse correctly")
		assert.Equal(t, "张三", users[0].Name, "First row Name should parse correctly")
	})

	t.Run("WithoutHeader", func(t *testing.T) {
		csvContent := "1,张三,zhangsan@example.com\n2,李四,lisi@example.com"

		importer := NewImporterFor[ImporterSimpleUser](WithoutHeader())
		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should succeed when header is disabled")
		assert.Empty(t, importErrors, "WithoutHeader should not produce per-row errors")

		users, ok := result.([]ImporterSimpleUser)
		require.True(t, ok, "Result should be []ImporterSimpleUser")
		require.Len(t, users, 2, "Both rows should be imported")
		assert.Equal(t, 1, users[0].ID, "First row ID should parse via positional mapping")
	})

	t.Run("WithSkipRows", func(t *testing.T) {
		csvContent := "用户数据表,,,\n用户ID,姓名,邮箱\n1,张三,zhangsan@example.com"

		importer := NewImporterFor[ImporterSimpleUser](WithSkipRows(1))
		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should succeed when skipping the title row")
		assert.Empty(t, importErrors, "Skipping rows should not surface per-row errors")

		users, ok := result.([]ImporterSimpleUser)
		require.True(t, ok, "Result should be []ImporterSimpleUser")
		require.Len(t, users, 1, "After skipping the title row, one data row should remain")
		assert.Equal(t, 1, users[0].ID, "Data row ID should be parsed after the skipped header")
	})

	t.Run("ValidationErrorsCollected", func(t *testing.T) {
		csvContent := "用户ID,姓名,邮箱,年龄\n1,张三,invalid-email,200"

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should not return a top-level error for validation failures")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		assert.Empty(t, imported, "Rows that fail validation should not be committed")
		assert.NotEmpty(t, importErrors, "Validation failures should be reported via importErrors")
	})

	t.Run("CustomParserRegistration", func(t *testing.T) {
		csvContent := "用户ID,姓名,邮箱\nID: 1,张三,zhang@example.com"

		importer := NewImporterFor[ImporterTestUser]()
		importer.RegisterParser("prefix_parser", &prefixParser{})

		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should succeed when a custom parser is registered")
		assert.Empty(t, importErrors, "Custom parser should not produce per-row errors")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		assert.Len(t, imported, 1, "One row should be imported")
	})

	t.Run("ImportFromFile", func(t *testing.T) {
		users := []ImporterTestUser{
			{ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				Birthday: time.Now(), Active: true},
		}

		exporter := NewExporterFor[ImporterTestUser]()
		tmpFile, err := os.CreateTemp("", "test_csv_import_*.csv")
		require.NoError(t, err, "CreateTemp should succeed")

		filename := tmpFile.Name()
		require.NoError(t, tmpFile.Close(), "Closing temp file should succeed")
		defer os.Remove(filename)

		require.NoError(t, exporter.ExportToFile(users, filename), "Seeding the temp file should succeed")

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "ImportFromFile should succeed for valid input")
		assert.Empty(t, importErrors, "ImportFromFile should not produce per-row errors")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		require.Len(t, imported, 1, "Exactly one row should be imported")
		assert.Equal(t, "1", imported[0].ID, "Imported ID should match seeded value")
	})

	t.Run("EmptyRowsSkipped", func(t *testing.T) {
		csvContent := "用户ID,姓名,邮箱\n1,张三,zhang@example.com\n\n2,李四,li@example.com"

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should succeed when there are blank rows")
		assert.Empty(t, importErrors, "Blank rows should not produce per-row errors")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		assert.Len(t, imported, 2, "Blank rows should be skipped, leaving the two data rows")
	})

	t.Run("MissingColumnsTolerated", func(t *testing.T) {
		csvContent := "用户ID,姓名,邮箱,年龄\n1,张三,zhang@example.com,30"

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should succeed when only a subset of columns is present")
		assert.Empty(t, importErrors, "Missing optional columns should not produce per-row errors")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		require.Len(t, imported, 1, "One row should be imported")
		assert.Equal(t, "1", imported[0].ID, "ID should parse from the present column")
		assert.Equal(t, 0.0, imported[0].Salary, "Absent Salary column should leave the field at zero")
		assert.Nil(t, imported[0].Remark, "Absent pointer column should remain nil")
	})

	t.Run("InvalidDataReportsErrors", func(t *testing.T) {
		csvContent := "用户ID,姓名,邮箱,年龄\n1,张三,invalid-email,not-a-number"

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.Import(strings.NewReader(csvContent))
		require.NoError(t, err, "Import should not return a top-level error for parse failures")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		assert.Empty(t, imported, "Row with invalid Age should not be committed")
		assert.NotEmpty(t, importErrors, "Parse failures should surface via importErrors")
	})

	t.Run("LargeFileRoundTrip", func(t *testing.T) {
		count := 1000
		users := make([]ImporterTestUser, count)

		for i := range count {
			users[i] = ImporterTestUser{
				ID: fmt.Sprintf("%d", i+1), Name: fmt.Sprintf("用户%d", i+1),
				Email: fmt.Sprintf("user%d@example.com", i+1), Age: 20 + (i % 50),
				Salary: 5000.0 + float64(i*100), Birthday: time.Now(), Active: i%2 == 0,
			}
		}

		exporter := NewExporterFor[ImporterTestUser]()
		tmpFile, err := os.CreateTemp("", "test_csv_large_*.csv")
		require.NoError(t, err, "CreateTemp should succeed")

		filename := tmpFile.Name()
		require.NoError(t, tmpFile.Close(), "Closing temp file should succeed")
		defer os.Remove(filename)

		require.NoError(t, exporter.ExportToFile(users, filename), "ExportToFile should succeed for 1k rows")

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "ImportFromFile should succeed for 1k rows")
		assert.Empty(t, importErrors, "Large round trip should not produce per-row errors")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		assert.Len(t, imported, count, "All rows should be re-imported")
		assert.Equal(t, fmt.Sprintf("%d", count), imported[count-1].ID, "Last row should round-trip")
	})

	t.Run("RoundTripPreservesAllFields", func(t *testing.T) {
		original := []ImporterTestUser{
			{ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				Birthday: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Active: true, Remark: new("测试用户1")},
			{ID: "2", Name: "李四", Email: "li@example.com", Age: 25, Salary: 8000.75,
				Birthday: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), Active: false, Remark: nil},
		}

		exporter := NewExporterFor[ImporterTestUser]()
		buf, err := exporter.Export(original)
		require.NoError(t, err, "Export should succeed for the round-trip fixture")

		importer := NewImporterFor[ImporterTestUser]()
		result, importErrors, err := importer.Import(strings.NewReader(buf.String()))
		require.NoError(t, err, "Import should succeed for exporter output")
		assert.Empty(t, importErrors, "Round trip should not produce per-row errors")

		imported, ok := result.([]ImporterTestUser)
		require.True(t, ok, "Result should be []ImporterTestUser")
		require.Len(t, imported, len(original), "Round trip should preserve row count")

		for i := range original {
			assert.Equal(t, original[i].ID, imported[i].ID, "Row %d ID should round-trip", i)
			assert.Equal(t, original[i].Name, imported[i].Name, "Row %d Name should round-trip", i)
			assert.Equal(t, original[i].Email, imported[i].Email, "Row %d Email should round-trip", i)
			assert.Equal(t, original[i].Age, imported[i].Age, "Row %d Age should round-trip", i)
			assert.InDelta(t, original[i].Salary, imported[i].Salary, 0.01, "Row %d Salary should round-trip within delta", i)
			assert.Equal(t, original[i].Active, imported[i].Active, "Row %d Active should round-trip", i)

			if original[i].Remark != nil {
				require.NotNil(t, imported[i].Remark, "Row %d Remark should round-trip as non-nil", i)
				assert.Equal(t, *original[i].Remark, *imported[i].Remark, "Row %d Remark value should round-trip", i)
			} else {
				assert.Nil(t, imported[i].Remark, "Row %d nil Remark should round-trip as nil", i)
			}
		}
	})

	t.Run("RoundTripWithoutTags", func(t *testing.T) {
		data := []ImporterNoTagStruct{
			{ID: "1", Name: "Alice", Age: 30},
			{ID: "2", Name: "Bob", Age: 25},
		}

		exporter := NewExporterFor[ImporterNoTagStruct]()
		buf, err := exporter.Export(data)
		require.NoError(t, err, "Export should succeed for fields without tabular tags")

		importer := NewImporterFor[ImporterNoTagStruct]()
		result, importErrors, err := importer.Import(strings.NewReader(buf.String()))
		require.NoError(t, err, "Import should succeed for tag-less round trip")
		assert.Empty(t, importErrors, "Tag-less round trip should not produce per-row errors")

		imported, ok := result.([]ImporterNoTagStruct)
		require.True(t, ok, "Result should be []ImporterNoTagStruct")
		require.Len(t, imported, 2, "Both rows should round-trip")
		assert.Equal(t, "Alice", imported[0].Name, "Tag-less Name field should round-trip")
	})
}

// baseDynamicSpecs returns the shared dynamic schema used by the map-based
// importer/exporter tests.
func baseDynamicSpecs() []tabular.ColumnSpec {
	return []tabular.ColumnSpec{
		{Key: "id", Name: "用户ID", Type: reflect.TypeFor[int](), Required: true},
		{Key: "name", Name: "姓名", Type: reflect.TypeFor[string](), Required: true},
		{Key: "birthday", Name: "生日", Type: reflect.TypeFor[time.Time](), Format: "2006-01-02"},
		{Key: "active", Name: "激活", Type: reflect.TypeFor[bool](), Default: "false"},
	}
}

// TestMapImporter covers the dynamic []map[string]any importer path including
// validation, parsing, and adapter-level options.
func TestMapImporter(t *testing.T) {
	t.Run("RoundTripWithExporter", func(t *testing.T) {
		exp, err := NewMapExporter(baseDynamicSpecs())
		require.NoError(t, err, "NewMapExporter should accept valid specs")

		imp, err := NewMapImporter(baseDynamicSpecs(), nil)
		require.NoError(t, err, "NewMapImporter should accept valid specs")

		birthday := time.Date(2000, 1, 15, 0, 0, 0, 0, time.Local)

		rows := []map[string]any{
			{"id": 1, "name": "张三", "birthday": birthday, "active": true},
			{"id": 2, "name": "李四", "birthday": birthday.Add(24 * time.Hour), "active": false},
		}

		buf, err := exp.Export(rows)
		require.NoError(t, err, "Export should succeed for map rows")
		assert.Contains(t, buf.String(), "2000-01-15", "Birthday should be formatted via Format template")

		result, importErrors, err := imp.Import(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err, "Import should succeed")
		assert.Empty(t, importErrors, "Round trip should not produce per-row errors")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Dynamic importer should return []map[string]any")
		require.Len(t, imported, 2, "Both rows should be imported")

		assert.Equal(t, 1, imported[0]["id"], "id should be parsed as int")
		assert.Equal(t, "张三", imported[0]["name"], "name should round-trip")
		assert.Equal(t, true, imported[0]["active"], "active should be parsed as bool")
		assert.Equal(t, birthday, imported[0]["birthday"], "birthday should parse back via the Format template")
	})

	t.Run("RequiredMissing", func(t *testing.T) {
		imp, err := NewMapImporter(baseDynamicSpecs(), nil)
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
		assert.Contains(t, buf.String(), "prefix:hello", "FormatterFn should transform the cell during export")

		result, importErrors, err := imp.Import(buf)
		require.NoError(t, err, "Import should succeed")
		assert.Empty(t, importErrors, "Import should not produce errors")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Import should return map rows")
		require.Len(t, imported, 1, "Exactly one row should be imported")
		assert.Equal(t, "hello", imported[0]["label"], "ParserFn should strip the prefix during import")
	})

	t.Run("IgnoresUnknownAndMissingColumns", func(t *testing.T) {
		imp, err := NewMapImporter(baseDynamicSpecs(), nil)
		require.NoError(t, err, "NewMapImporter should accept valid specs")

		content := "用户ID,姓名,Extra\n1,张三,ignored\n"

		result, importErrors, err := imp.Import(strings.NewReader(content))
		require.NoError(t, err, "Import should succeed despite missing / extra columns")
		assert.Empty(t, importErrors, "No per-row errors should be produced")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Import should return map rows")
		require.Len(t, imported, 1, "One row should be imported")

		row := imported[0]
		assert.Equal(t, 1, row["id"], "id should parse")
		assert.Equal(t, "张三", row["name"], "name should parse")
		_, hasExtra := row["Extra"]
		assert.False(t, hasExtra, "Unknown columns should not leak into the row")
		_, hasBirthday := row["birthday"]
		assert.False(t, hasBirthday, "Absent schema columns should not appear in the map")
		_, hasActive := row["active"]
		assert.False(t, hasActive, "Defaults only apply to empty cells within mapped columns")
	})

	t.Run("RowValidatorReportsError", func(t *testing.T) {
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

		content := "用户ID,姓名,生日,激活\n1,BAD,2000-01-15,true\n"

		result, importErrors, err := imp.Import(strings.NewReader(content))
		require.NoError(t, err, "Import should not return a top-level error")

		imported, ok := result.([]map[string]any)
		require.True(t, ok, "Import should return map rows")
		assert.Empty(t, imported, "Row rejected by validator should not be committed")
		require.NotEmpty(t, importErrors, "RowValidator failure should surface as ImportError")
		assert.Contains(t, importErrors[0].Error(), "blocked name",
			"ImportError message should include the validator's message")
	})

	t.Run("ParseErrorReportedPerCell", func(t *testing.T) {
		imp, err := NewMapImporter(baseDynamicSpecs(), nil)
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
	})
}
