package excel

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

type excelPrefixFormatter struct {
	prefix string
}

func (f *excelPrefixFormatter) Format(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	return f.prefix + " " + fmt.Sprint(value), nil
}

// TestExporter exercises the Excel exporter end to end against struct-typed
// adapters, including all option permutations.
func TestExporter(t *testing.T) {
	t.Run("ExportToFile", func(t *testing.T) {
		now := time.Now()
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: now, Status: 1, Remark: new("测试用户1"), Password: "secret123",
			},
			{
				ID: "2", Name: "李四", Email: "li@example.com", Age: 25, Salary: 8000.75,
				CreatedAt: now, Status: 2, Remark: nil, Password: "secret456",
			},
		}

		filename := exportToTemp(t, NewExporterFor[ExcelTestUser](), users, "test_users_*.xlsx")

		_, err := os.Stat(filename)
		assert.NoError(t, err, "Output file should exist after ExportToFile")
	})

	t.Run("ExportToBuffer", func(t *testing.T) {
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: time.Now(), Status: 1, Remark: new("测试"),
			},
		}

		exporter := NewExporterFor[ExcelTestUser]()
		buf, err := exporter.Export(users)
		require.NoError(t, err, "Export should succeed for valid struct slice")
		require.NotNil(t, buf, "Export should return a non-nil buffer")
		assert.Greater(t, buf.Len(), 0, "Buffer should contain serialized workbook bytes")
	})

	t.Run("EmptyDataStillWritesFile", func(t *testing.T) {
		var emptyUsers []ExcelTestUser

		filename := exportToTemp(t, NewExporterFor[ExcelTestUser](), emptyUsers, "test_empty_*.xlsx")

		_, err := os.Stat(filename)
		assert.NoError(t, err, "Output file should exist even for an empty slice")
	})

	t.Run("CustomFormatterRegistration", func(t *testing.T) {
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local), Status: 1, Remark: new("测试用户"),
			},
		}

		exporter := NewExporterFor[ExcelTestUser]()
		exporter.RegisterFormatter("prefix", &excelPrefixFormatter{prefix: "ID:"})

		filename := exportToTemp(t, exporter, users, "test_custom_formatter_*.xlsx")

		_, err := os.Stat(filename)
		assert.NoError(t, err, "Output file should exist after exporting with a custom formatter")
	})

	t.Run("WithSheetName", func(t *testing.T) {
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: time.Now(), Status: 1,
			},
		}

		exporter := NewExporterFor[ExcelTestUser](WithSheetName("用户数据"))
		filename := exportToTemp(t, exporter, users, "test_options_*.xlsx")

		f, err := excelize.OpenFile(filename)
		require.NoError(t, err, "Opening the exported workbook should succeed")
		t.Cleanup(func() {
			_ = f.Close()
		})

		sheets := f.GetSheetList()
		assert.Contains(t, sheets, "用户数据", "WithSheetName should rename the default sheet")
	})

	t.Run("NullPointerValuesRoundTripAsNil", func(t *testing.T) {
		users := []ExcelTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30, Salary: 10000.50,
				CreatedAt: time.Now(), Status: 1, Remark: nil,
			},
			{
				ID: "2", Name: "李四", Email: "li@example.com", Age: 25, Salary: 8000.00,
				CreatedAt: time.Now(), Status: 2, Remark: new("有备注"),
			},
		}

		filename := exportToTemp(t, NewExporterFor[ExcelTestUser](), users, "test_null_values_*.xlsx")

		importer := NewImporterFor[ExcelTestUser]()
		importedAny, importErrors, err := importer.ImportFromFile(filename)
		require.NoError(t, err, "Import should succeed for nil-pointer round trip")
		assert.Empty(t, importErrors, "Nil pointer round trip should not produce per-row errors")

		imported, ok := importedAny.([]ExcelTestUser)
		require.True(t, ok, "Result should be []ExcelTestUser")
		require.Len(t, imported, 2, "Both rows should be imported")
		assert.Nil(t, imported[0].Remark, "Nil Remark should round-trip as nil")
		require.NotNil(t, imported[1].Remark, "Non-nil Remark should round-trip as non-nil")
		assert.Equal(t, "有备注", *imported[1].Remark, "Remark value should round-trip")
	})
}

// TestMapExporter covers the dynamic []map[string]any exporter path including
// width propagation and schema validation failures.
func TestMapExporter(t *testing.T) {
	t.Run("RejectsBadSpecs", func(t *testing.T) {
		_, err := NewMapExporter([]tabular.ColumnSpec{{Key: "id"}})
		require.Error(t, err, "NewMapExporter should reject specs with missing type")
		assert.ErrorIs(t, err, tabular.ErrMissingColumnType, "Error should wrap ErrMissingColumnType")
	})

	t.Run("PropagatesColumnWidth", func(t *testing.T) {
		exp, err := NewMapExporter(baseDynamicSpecs(), WithSheetName("Users"))
		require.NoError(t, err, "NewMapExporter should accept valid specs")

		buf, err := exp.Export([]map[string]any{{"id": 1, "name": "张三"}})
		require.NoError(t, err, "Export should succeed")

		f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err, "Opening the exported workbook should succeed")
		t.Cleanup(func() {
			_ = f.Close()
		})

		widthA, err := f.GetColWidth("Users", "A")
		require.NoError(t, err, "Reading column A width should succeed")
		assert.InDelta(t, 12.0, widthA, 0.1, "Column A should reflect the configured Width")

		widthB, err := f.GetColWidth("Users", "B")
		require.NoError(t, err, "Reading column B width should succeed")
		assert.InDelta(t, 18.0, widthB, 0.1, "Column B should reflect the configured Width")
	})
}
