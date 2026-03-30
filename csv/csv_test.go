package csv

import (
	"bytes"
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

type TestUser struct {
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

// TestCSVExportImport tests CSV export import functionality.
func TestCSVExportImport(t *testing.T) {
	users := []TestUser{
		{
			ID:       "1",
			Name:     "张三",
			Email:    "zhangsan@example.com",
			Age:      30,
			Salary:   10000.50,
			Birthday: time.Date(1994, 1, 15, 0, 0, 0, 0, time.UTC),
			Active:   true,
			Remark:   new("测试用户1"),
		},
		{
			ID:       "2",
			Name:     "李四",
			Email:    "lisi@example.com",
			Age:      25,
			Salary:   8000.75,
			Birthday: time.Date(1999, 5, 20, 0, 0, 0, 0, time.UTC),
			Active:   false,
			Remark:   nil,
		},
	}

	exporter := NewExporterFor[TestUser]()
	buf, err := exporter.Export(users)
	require.NoError(t, err, "Should not return error")
	require.NotNil(t, buf, "Should not be nil")

	csvContent := buf.String()
	t.Logf("Exported CSV:\n%s", csvContent)

	assert.Contains(t, csvContent, "用户ID", "Should contain expected value")
	assert.Contains(t, csvContent, "姓名", "Should contain expected value")
	assert.Contains(t, csvContent, "邮箱", "Should contain expected value")

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.Import(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	importedUsers, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	require.Len(t, importedUsers, 2, "Length should be 2")

	assert.Equal(t, "1", importedUsers[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", importedUsers[0].Name, "Should equal expected value")
	assert.Equal(t, "zhangsan@example.com", importedUsers[0].Email, "Should equal expected value")
	assert.Equal(t, 30, importedUsers[0].Age, "Should equal expected value")
	assert.InDelta(t, 10000.50, importedUsers[0].Salary, 0.01, "Salary should be within delta of expected value")
	assert.Equal(t, "1994-01-15", importedUsers[0].Birthday.Format("2006-01-02"), "Should equal expected value")
	assert.True(t, importedUsers[0].Active, "Should be true")
	assert.True(t, importedUsers[0].Remark != nil, "Should be valid")
	assert.Equal(t, "测试用户1", *importedUsers[0].Remark, "Should equal expected value")

	assert.Equal(t, "2", importedUsers[1].ID, "Should equal expected value")
	assert.Equal(t, "李四", importedUsers[1].Name, "Should equal expected value")
	assert.Equal(t, "lisi@example.com", importedUsers[1].Email, "Should equal expected value")
	assert.Equal(t, 25, importedUsers[1].Age, "Should equal expected value")
	assert.InDelta(t, 8000.75, importedUsers[1].Salary, 0.01, "Salary should be within delta of expected value")
	assert.Equal(t, "1999-05-20", importedUsers[1].Birthday.Format("2006-01-02"), "Should equal expected value")
	assert.False(t, importedUsers[1].Active, "Should be false")
	assert.False(t, importedUsers[1].Remark != nil, "Should not be valid")
}

// TestCSVImportWithCustomDelimiter tests CSV import with custom delimiter functionality.
func TestCSVImportWithCustomDelimiter(t *testing.T) {
	csvContent := `用户ID;姓名;邮箱
1;张三;zhangsan@example.com
2;李四;lisi@example.com`

	type SimpleUser struct {
		ID    int    `tabular:"用户ID"`
		Name  string `tabular:"姓名"`
		Email string `tabular:"邮箱"`
	}

	importer := NewImporterFor[SimpleUser](WithImportDelimiter(';'))
	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	users, ok := result.([]SimpleUser)
	require.True(t, ok, "Should be ok")
	require.Len(t, users, 2, "Length should be 2")

	assert.Equal(t, 1, users[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", users[0].Name, "Should equal expected value")
}

// TestCSVImportWithoutHeader tests CSV import without header functionality.
func TestCSVImportWithoutHeader(t *testing.T) {
	csvContent := `1,张三,zhangsan@example.com
2,李四,lisi@example.com`

	type SimpleUser struct {
		ID    int    `tabular:"用户ID"`
		Name  string `tabular:"姓名"`
		Email string `tabular:"邮箱"`
	}

	importer := NewImporterFor[SimpleUser](WithoutHeader())
	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	users, ok := result.([]SimpleUser)
	require.True(t, ok, "Should be ok")
	require.Len(t, users, 2, "Length should be 2")

	assert.Equal(t, 1, users[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", users[0].Name, "Should equal expected value")
}

// TestCSVExportWithoutHeader tests CSV export without header functionality.
func TestCSVExportWithoutHeader(t *testing.T) {
	type SimpleUser struct {
		ID    int    `tabular:"用户ID"`
		Name  string `tabular:"姓名"`
		Email string `tabular:"邮箱"`
	}

	users := []SimpleUser{
		{ID: 1, Name: "张三", Email: "zhangsan@example.com"},
	}

	exporter := NewExporterFor[SimpleUser](WithoutWriteHeader())
	buf, err := exporter.Export(users)
	require.NoError(t, err, "Should not return error")

	csvContent := buf.String()
	assert.NotContains(t, csvContent, "用户ID", "Should not contain value")
	assert.Contains(t, csvContent, "1,张三,zhangsan@example.com", "Should contain expected value")
}

// TestCSVImportWithSkipRows tests CSV import with skip rows functionality.
func TestCSVImportWithSkipRows(t *testing.T) {
	csvContent := `用户数据表,,,
用户ID,姓名,邮箱
1,张三,zhangsan@example.com`

	type SimpleUser struct {
		ID    int    `tabular:"用户ID"`
		Name  string `tabular:"姓名"`
		Email string `tabular:"邮箱"`
	}

	importer := NewImporterFor[SimpleUser](WithSkipRows(1))
	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	users, ok := result.([]SimpleUser)
	require.True(t, ok, "Should be ok")
	require.Len(t, users, 1, "Length should be 1")

	assert.Equal(t, 1, users[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", users[0].Name, "Should equal expected value")
}

// TestSchemaParseTags tests schema parse tags functionality.
func TestSchemaParseTags(t *testing.T) {
	schema := tabular.NewSchemaFor[TestUser]()

	columns := schema.Columns()
	assert.NotEmpty(t, columns, "Should not be empty")

	var idCol, nameCol, passwordCol *tabular.Column

	for i := range columns {
		col := columns[i]
		switch col.Name {
		case "用户ID":
			idCol = col
		case "姓名":
			nameCol = col
		case "Password":
			passwordCol = col
		}
	}

	require.NotNil(t, idCol, "Should not be nil")
	assert.Equal(t, "用户ID", idCol.Name, "Should equal expected value")

	require.NotNil(t, nameCol, "Should not be nil")
	assert.Equal(t, "姓名", nameCol.Name, "Should equal expected value")

	assert.Nil(t, passwordCol, "Should be nil")
}

type TestNoTagStruct struct {
	ID   string
	Name string
	Age  int
}

// TestSchemaNoTags tests schema no tags functionality.
func TestSchemaNoTags(t *testing.T) {
	schema := tabular.NewSchemaFor[TestNoTagStruct]()

	columns := schema.Columns()
	assert.Len(t, columns, 3, "Length should be 3")

	assert.Equal(t, "ID", columns[0].Name, "Should equal expected value")
	assert.Equal(t, "Name", columns[1].Name, "Should equal expected value")
	assert.Equal(t, "Age", columns[2].Name, "Should equal expected value")
}

// TestExportImportNoTags tests export import no tags functionality.
func TestExportImportNoTags(t *testing.T) {
	data := []TestNoTagStruct{
		{ID: "1", Name: "Alice", Age: 30},
		{ID: "2", Name: "Bob", Age: 25},
	}

	exporter := NewExporterFor[TestNoTagStruct]()
	buf, err := exporter.Export(data)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestNoTagStruct]()
	result, importErrors, err := importer.Import(strings.NewReader(buf.String()))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestNoTagStruct)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, 2, "Length should be 2")

	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "Alice", imported[0].Name, "Should equal expected value")
	assert.Equal(t, 30, imported[0].Age, "Should equal expected value")
}

// TestImporterValidationErrors tests importer validation errors functionality.
func TestImporterValidationErrors(t *testing.T) {
	csvContent := `用户ID,姓名,邮箱,年龄
1,张三,invalid-email,200`

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Empty(t, imported, "Should be empty")
	assert.NotEmpty(t, importErrors, "Should not be empty")
}

type PrefixFormatter struct {
	prefix string
}

func (f *PrefixFormatter) Format(value any) (string, error) {
	if value == nil {
		return "", nil
	}

	return f.prefix + " " + fmt.Sprint(value), nil
}

// TestExportCustomFormatter tests export custom formatter functionality.
func TestExportCustomFormatter(t *testing.T) {
	users := []TestUser{
		{
			ID:       "1",
			Name:     "张三",
			Email:    "zhang@example.com",
			Age:      30,
			Salary:   10000.50,
			Birthday: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Active:   true,
		},
	}

	exporter := NewExporterFor[TestUser]()
	exporter.RegisterFormatter("prefix", &PrefixFormatter{prefix: "ID:"})

	buf, err := exporter.Export(users)
	require.NoError(t, err, "Should not return error")
	assert.NotNil(t, buf, "Should not be nil")
}

type PrefixParser struct{}

func (*PrefixParser) Parse(cellValue string, _ reflect.Type) (any, error) {
	if cellValue == "" {
		return "", nil
	}

	if len(cellValue) > 4 {
		return cellValue[4:], nil
	}

	return cellValue, nil
}

// TestImportCustomParser tests import custom parser functionality.
func TestImportCustomParser(t *testing.T) {
	csvContent := `用户ID,姓名,邮箱
ID: 1,张三,zhang@example.com`

	importer := NewImporterFor[TestUser]()
	importer.RegisterParser("prefix_parser", &PrefixParser{})

	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, 1, "Length should be 1")
}

// TestExportEmptyData tests export empty data functionality.
func TestExportEmptyData(t *testing.T) {
	var emptyUsers []TestUser

	exporter := NewExporterFor[TestUser]()
	buf, err := exporter.Export(emptyUsers)
	require.NoError(t, err, "Should not return error")

	csvContent := buf.String()
	assert.Contains(t, csvContent, "用户ID", "Should contain expected value")
	assert.Contains(t, csvContent, "姓名", "Should contain expected value")
}

// TestExportToFile tests export to file functionality.
func TestExportToFile(t *testing.T) {
	users := []TestUser{
		{
			ID:       "1",
			Name:     "张三",
			Email:    "zhang@example.com",
			Age:      30,
			Salary:   10000.50,
			Birthday: time.Now(),
			Active:   true,
		},
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_csv_export_*.csv")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	_, err = os.Stat(filename)
	assert.NoError(t, err, "Should not return error")
}

// TestImportFromFile tests import from file functionality.
func TestImportFromFile(t *testing.T) {
	users := []TestUser{
		{
			ID:       "1",
			Name:     "张三",
			Email:    "zhang@example.com",
			Age:      30,
			Salary:   10000.50,
			Birthday: time.Now(),
			Active:   true,
		},
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_csv_import_*.csv")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.ImportFromFile(filename)
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, 1, "Length should be 1")
	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", imported[0].Name, "Should equal expected value")
}

// TestImportEmptyRows tests import empty rows functionality.
func TestImportEmptyRows(t *testing.T) {
	csvContent := `用户ID,姓名,邮箱
1,张三,zhang@example.com

2,李四,li@example.com`

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, 2, "Length should be 2")
}

// TestImportMissingColumns tests import missing columns functionality.
func TestImportMissingColumns(t *testing.T) {
	csvContent := `用户ID,姓名,邮箱,年龄
1,张三,zhang@example.com,30`

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, 1, "Length should be 1")

	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", imported[0].Name, "Should equal expected value")
	assert.Equal(t, 0.0, imported[0].Salary, "Should equal expected value")
	assert.False(t, imported[0].Remark != nil, "Should not be valid")
}

// TestImportInvalidData tests import invalid data functionality.
func TestImportInvalidData(t *testing.T) {
	csvContent := `用户ID,姓名,邮箱,年龄
1,张三,invalid-email,not-a-number`

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.Import(strings.NewReader(csvContent))
	require.NoError(t, err, "Should not return error")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Empty(t, imported, "Should be empty")
	assert.NotEmpty(t, importErrors, "Should not be empty")
}

// TestImportLargeFile tests import large file functionality.
func TestImportLargeFile(t *testing.T) {
	count := 1000
	users := make([]TestUser, count)

	for i := range count {
		users[i] = TestUser{
			ID:       fmt.Sprintf("%d", i+1),
			Name:     fmt.Sprintf("用户%d", i+1),
			Email:    fmt.Sprintf("user%d@example.com", i+1),
			Age:      20 + (i % 50),
			Salary:   5000.0 + float64(i*100),
			Birthday: time.Now(),
			Active:   i%2 == 0,
		}
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_csv_large_*.csv")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.ImportFromFile(filename)
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, count, "Length should be count")

	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "用户1", imported[0].Name, "Should equal expected value")
	assert.Equal(t, fmt.Sprintf("%d", count), imported[count-1].ID, "Should equal expected value")
}

// TestExportNullValues tests export null values functionality.
func TestExportNullValues(t *testing.T) {
	users := []TestUser{
		{
			ID:       "1",
			Name:     "张三",
			Email:    "zhang@example.com",
			Age:      30,
			Salary:   10000.50,
			Birthday: time.Now(),
			Active:   true,
			Remark:   nil,
		},
		{
			ID:       "2",
			Name:     "李四",
			Email:    "li@example.com",
			Age:      25,
			Salary:   8000.00,
			Birthday: time.Now(),
			Active:   false,
			Remark:   new("有备注"),
		},
	}

	exporter := NewExporterFor[TestUser]()
	buf, err := exporter.Export(users)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.Import(strings.NewReader(buf.String()))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, 2, "Length should be 2")

	assert.False(t, imported[0].Remark != nil, "Should not be valid")
	assert.True(t, imported[1].Remark != nil, "Should be valid")
	assert.Equal(t, "有备注", *imported[1].Remark, "Should equal expected value")
}

// TestRoundTrip tests round trip functionality.
func TestRoundTrip(t *testing.T) {
	original := []TestUser{
		{
			ID:       "1",
			Name:     "张三",
			Email:    "zhang@example.com",
			Age:      30,
			Salary:   10000.50,
			Birthday: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Active:   true,
			Remark:   new("测试用户1"),
		},
		{
			ID:       "2",
			Name:     "李四",
			Email:    "li@example.com",
			Age:      25,
			Salary:   8000.75,
			Birthday: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			Active:   false,
			Remark:   nil,
		},
	}

	exporter := NewExporterFor[TestUser]()
	buf, err := exporter.Export(original)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	result, importErrors, err := importer.Import(strings.NewReader(buf.String()))
	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")

	imported, ok := result.([]TestUser)
	require.True(t, ok, "Should be ok")
	assert.Len(t, imported, len(original), "Length should be len(original)")

	for i := range original {
		assert.Equal(t, original[i].ID, imported[i].ID, "Should equal expected value")
		assert.Equal(t, original[i].Name, imported[i].Name, "Should equal expected value")
		assert.Equal(t, original[i].Email, imported[i].Email, "Should equal expected value")
		assert.Equal(t, original[i].Age, imported[i].Age, "Should equal expected value")
		assert.InDelta(t, original[i].Salary, imported[i].Salary, 0.01, "Salary should be within delta of original value")
		assert.Equal(t, original[i].Active, imported[i].Active, "Should equal expected value")
		assert.Equal(t, original[i].Remark != nil, imported[i].Remark != nil, "Should equal expected value")

		if original[i].Remark != nil {
			assert.Equal(t, *original[i].Remark, *imported[i].Remark, "Should equal expected value")
		}
	}
}
