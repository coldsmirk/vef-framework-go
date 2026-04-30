package csv

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

type ExporterTestUser struct {
	ID       string    `tabular:"用户ID"`
	Name     string    `tabular:"姓名"`
	Email    string    `tabular:"邮箱"`
	Age      int       `tabular:"年龄"`
	Salary   float64   `tabular:"薪资,format=%.2f"`
	Birthday time.Time `tabular:"生日,format=2006-01-02"`
	Active   bool      `tabular:"激活状态"`
	Remark   *string   `tabular:"备注"`
}

type ExporterSimpleUser struct {
	ID    int    `tabular:"用户ID"`
	Name  string `tabular:"姓名"`
	Email string `tabular:"邮箱"`
}

type prefixFormatter struct {
	prefix string
}

func (f *prefixFormatter) Format(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	return f.prefix + " " + fmt.Sprint(value), nil
}

// TestExporter exercises the CSV exporter end to end against struct-typed
// adapters, including all option permutations.
func TestExporter(t *testing.T) {
	t.Run("DefaultExportContainsHeader", func(t *testing.T) {
		users := []ExporterTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30,
				Salary: 10000.50, Birthday: time.Now(), Active: true,
			},
		}

		exporter := NewExporterFor[ExporterTestUser]()
		buf, err := exporter.Export(users)
		require.NoError(t, err, "Export should succeed for valid struct slice")
		require.NotNil(t, buf, "Export should return a non-nil buffer")

		csvContent := buf.String()
		assert.Contains(t, csvContent, "用户ID", "Header should include the ID column name")
		assert.Contains(t, csvContent, "姓名", "Header should include the Name column name")
		assert.Contains(t, csvContent, "邮箱", "Header should include the Email column name")
	})

	t.Run("WithoutHeader", func(t *testing.T) {
		users := []ExporterSimpleUser{
			{ID: 1, Name: "张三", Email: "zhangsan@example.com"},
		}

		exporter := NewExporterFor[ExporterSimpleUser](WithoutWriteHeader())
		buf, err := exporter.Export(users)
		require.NoError(t, err, "Export should succeed when header is suppressed")

		csvContent := buf.String()
		assert.NotContains(t, csvContent, "用户ID", "WithoutWriteHeader should omit the column header")
		assert.Contains(t, csvContent, "1,张三,zhangsan@example.com", "Data row should still be emitted")
	})

	t.Run("EmptyDataStillEmitsHeader", func(t *testing.T) {
		var emptyUsers []ExporterTestUser

		exporter := NewExporterFor[ExporterTestUser]()
		buf, err := exporter.Export(emptyUsers)
		require.NoError(t, err, "Export should succeed for an empty slice")

		csvContent := buf.String()
		assert.Contains(t, csvContent, "用户ID", "Header should still be written for an empty slice")
		assert.Contains(t, csvContent, "姓名", "Header should still be written for an empty slice")
	})

	t.Run("ExportToFile", func(t *testing.T) {
		users := []ExporterTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30,
				Salary: 10000.50, Birthday: time.Now(), Active: true,
			},
		}

		exporter := NewExporterFor[ExporterTestUser]()
		tmpFile, err := os.CreateTemp("", "test_csv_export_*.csv")
		require.NoError(t, err, "CreateTemp should succeed")

		filename := tmpFile.Name()
		require.NoError(t, tmpFile.Close(), "Closing temp file should succeed")

		defer os.Remove(filename)

		require.NoError(t, exporter.ExportToFile(users, filename), "ExportToFile should succeed")

		_, err = os.Stat(filename)
		assert.NoError(t, err, "Output file should exist after ExportToFile")
	})

	t.Run("CustomFormatterRegistration", func(t *testing.T) {
		users := []ExporterTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30,
				Salary: 10000.50, Birthday: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Active: true,
			},
		}

		exporter := NewExporterFor[ExporterTestUser]()
		exporter.RegisterFormatter("prefix", &prefixFormatter{prefix: "ID:"})

		buf, err := exporter.Export(users)
		require.NoError(t, err, "Export should succeed with a registered custom formatter")
		assert.NotNil(t, buf, "Export should return a non-nil buffer")
	})

	t.Run("NullPointerValuesEmitEmptyCells", func(t *testing.T) {
		users := []ExporterTestUser{
			{
				ID: "1", Name: "张三", Email: "zhang@example.com", Age: 30,
				Salary: 10000.50, Birthday: time.Now(), Active: true, Remark: nil,
			},
		}

		exporter := NewExporterFor[ExporterTestUser]()
		buf, err := exporter.Export(users)
		require.NoError(t, err, "Export should succeed with nil pointer fields")

		// Round-trip via importer to confirm the nil cell did not become "<nil>".
		importer := NewImporterFor[ExporterTestUser]()
		result, importErrors, err := importer.Import(strings.NewReader(buf.String()))
		require.NoError(t, err, "Import should succeed for nil-pointer round trip")
		assert.Empty(t, importErrors, "Nil pointer round trip should not produce per-row errors")

		imported, ok := result.([]ExporterTestUser)
		require.True(t, ok, "Result should be []ExporterTestUser")
		require.Len(t, imported, 1, "Exactly one row should be imported")
		assert.Nil(t, imported[0].Remark, "Nil Remark should round-trip as nil, not as empty string pointer")
	})
}

// TestMapExporter covers the dynamic []map[string]any exporter path including
// schema validation failures.
func TestMapExporter(t *testing.T) {
	t.Run("RejectsBadSpecs", func(t *testing.T) {
		_, err := NewMapExporter([]tabular.ColumnSpec{{Key: "id"}})
		require.Error(t, err, "NewMapExporter should reject specs with missing type")
		assert.ErrorIs(t, err, tabular.ErrMissingColumnType, "Error should wrap ErrMissingColumnType")
	})
}
