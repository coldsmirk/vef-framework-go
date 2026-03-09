package excel

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/tabular"
)

// TestUser is a test struct for Excel operations.
type TestUser struct {
	ID        string      `tabular:"width=15"                                      validate:"required"`
	Name      string      `tabular:"姓名,width=20"                                   validate:"required"`
	Email     string      `tabular:"邮箱,width=25"                                   validate:"email"`
	Age       int         `tabular:"name=年龄,width=10"                              validate:"gte=0,lte=150"`
	Salary    float64     `tabular:"name=薪资,width=15,format=%.2f"`
	CreatedAt time.Time   `tabular:"name=创建时间,width=20,format=2006-01-02 15:04:05"`
	Status    int         `tabular:"name=状态,width=10"`
	Remark    null.String `tabular:"name=备注,width=30"`
	Password  string      `tabular:"-"` // Ignored field
}

// TestExporterExportToFile tests exporter export to file functionality.
func TestExporterExportToFile(t *testing.T) {
	now := time.Now()
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: now,
			Status:    1,
			Remark:    null.StringFrom("测试用户1"),
			Password:  "secret123",
		},
		{
			ID:        "2",
			Name:      "李四",
			Email:     "li@example.com",
			Age:       25,
			Salary:    8000.75,
			CreatedAt: now,
			Status:    2,
			Remark:    null.String{},
			Password:  "secret456",
		},
	}

	exporter := NewExporterFor[TestUser]()

	tmpFile, err := os.CreateTemp("", "test_users_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	_, err = os.Stat(filename)
	assert.NoError(t, err, "Should not return error")
}

// TestImporterImportFromFile tests importer import from file functionality.
func TestImporterImportFromFile(t *testing.T) {
	now := time.Now()
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: now,
			Status:    1,
			Remark:    null.StringFrom("测试用户1"),
		},
		{
			ID:        "2",
			Name:      "李四",
			Email:     "li@example.com",
			Age:       25,
			Salary:    8000.75,
			CreatedAt: now,
			Status:    2,
			Remark:    null.String{},
		},
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_import_users_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 2, "Length should be 2")

	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", imported[0].Name, "Should equal expected value")
	assert.Equal(t, "zhang@example.com", imported[0].Email, "Should equal expected value")
	assert.Equal(t, 30, imported[0].Age, "Should equal expected value")
	assert.InDelta(t, 10000.50, imported[0].Salary, 0.01, "Salary should be within delta of expected value")
	assert.Equal(t, 1, imported[0].Status, "Should equal expected value")
	assert.True(t, imported[0].Remark.Valid, "Should be valid")
	assert.Equal(t, "测试用户1", imported[0].Remark.ValueOrZero(), "Should equal expected value")

	assert.Equal(t, "2", imported[1].ID, "Should equal expected value")
	assert.Equal(t, "李四", imported[1].Name, "Should equal expected value")
	assert.False(t, imported[1].Remark.Valid, "Should not be valid")
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
		case "ID":
			idCol = col
		case "姓名":
			nameCol = col
		case "Password":
			passwordCol = col
		}
	}

	require.NotNil(t, idCol, "Should not be nil")
	assert.Equal(t, "ID", idCol.Name, "Should equal expected value")
	assert.Equal(t, 15.0, idCol.Width, "Should equal expected value")

	require.NotNil(t, nameCol, "Should not be nil")
	assert.Equal(t, "姓名", nameCol.Name, "Should equal expected value")
	assert.Equal(t, 20.0, nameCol.Width, "Should equal expected value")

	assert.Nil(t, passwordCol, "Should be nil")
}

// TestImporterValidationErrors tests importer validation errors functionality.
func TestImporterValidationErrors(t *testing.T) {
	invalidUsers := []TestUser{
		{
			ID:     "1",
			Name:   "张三",
			Email:  "invalid-email",
			Age:    200,
			Salary: 10000,
		},
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_invalid_users_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(invalidUsers, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, imported, "Should be empty")
	assert.NotEmpty(t, importErrors, "Should not be empty")
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
	tmpFile, err := os.CreateTemp("", "test_no_tags_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(data, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestNoTagStruct]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestNoTagStruct)
	require.True(t, ok, "Type assertion to []TestNoTagStruct should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 2, "Length should be 2")

	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "Alice", imported[0].Name, "Should equal expected value")
	assert.Equal(t, 30, imported[0].Age, "Should equal expected value")
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
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local),
			Status:    1,
			Remark:    null.StringFrom("测试用户"),
		},
	}

	exporter := NewExporterFor[TestUser]()
	exporter.RegisterFormatter("prefix", &PrefixFormatter{prefix: "ID:"})

	tmpFile, err := os.CreateTemp("", "test_custom_formatter_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	_, err = os.Stat(filename)
	assert.NoError(t, err, "Should not return error")
}

// TestExportToBuffer tests export to buffer functionality.
func TestExportToBuffer(t *testing.T) {
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: time.Now(),
			Status:    1,
			Remark:    null.StringFrom("测试"),
		},
	}

	exporter := NewExporterFor[TestUser]()
	buf, err := exporter.Export(users)

	require.NoError(t, err, "Should not return error")
	assert.NotNil(t, buf, "Should not be nil")
	assert.Greater(t, buf.Len(), 0, "Should be greater")
}

// TestExportEmptyData tests export empty data functionality.
func TestExportEmptyData(t *testing.T) {
	var emptyUsers []TestUser

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_empty_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(emptyUsers, filename)
	require.NoError(t, err, "Should not return error")

	_, err = os.Stat(filename)
	assert.NoError(t, err, "Should not return error")
}

// TestExportWithOptions tests export with options functionality.
func TestExportWithOptions(t *testing.T) {
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: time.Now(),
			Status:    1,
		},
	}

	exporter := NewExporterFor[TestUser](WithSheetName("用户数据"))
	tmpFile, err := os.CreateTemp("", "test_options_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	_, err = os.Stat(filename)
	assert.NoError(t, err, "Should not return error")
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
	now := time.Now()
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: now,
			Status:    1,
			Remark:    null.StringFrom("测试"),
		},
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_custom_parser_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importer.RegisterParser("prefix_parser", &PrefixParser{})

	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 1, "Length should be 1")
}

// TestImportFromReader tests import from reader functionality.
func TestImportFromReader(t *testing.T) {
	now := time.Now()
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: now,
			Status:    1,
		},
	}

	exporter := NewExporterFor[TestUser]()
	buf, err := exporter.Export(users)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.Import(buf)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 1, "Length should be 1")
	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", imported[0].Name, "Should equal expected value")
}

// TestImportWithOptions tests import with options functionality.
func TestImportWithOptions(t *testing.T) {
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: time.Now(),
			Status:    1,
		},
	}

	exporter := NewExporterFor[TestUser](WithSheetName("用户数据"))
	tmpFile, err := os.CreateTemp("", "test_import_options_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser](WithImportSheetName("用户数据"))
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 1, "Length should be 1")
}

// TestImportEmptyRows tests import empty rows functionality.
func TestImportEmptyRows(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_empty_rows_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	f := excelize.NewFile()
	sheetName := "Sheet1"

	_ = f.SetCellValue(sheetName, "A1", "ID")
	_ = f.SetCellValue(sheetName, "B1", "姓名")
	_ = f.SetCellValue(sheetName, "C1", "邮箱")

	_ = f.SetCellValue(sheetName, "A2", "1")
	_ = f.SetCellValue(sheetName, "B2", "张三")
	_ = f.SetCellValue(sheetName, "C2", "zhang@example.com")

	_ = f.SetCellValue(sheetName, "A4", "2")
	_ = f.SetCellValue(sheetName, "B4", "李四")
	_ = f.SetCellValue(sheetName, "C4", "li@example.com")

	err = f.SaveAs(filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 2, "Length should be 2")
}

// TestImportMissingColumns tests import missing columns functionality.
func TestImportMissingColumns(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_missing_columns_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	f := excelize.NewFile()
	sheetName := "Sheet1"

	_ = f.SetCellValue(sheetName, "A1", "ID")
	_ = f.SetCellValue(sheetName, "B1", "姓名")
	_ = f.SetCellValue(sheetName, "C1", "邮箱")
	_ = f.SetCellValue(sheetName, "D1", "年龄")

	_ = f.SetCellValue(sheetName, "A2", "1")
	_ = f.SetCellValue(sheetName, "B2", "张三")
	_ = f.SetCellValue(sheetName, "C2", "zhang@example.com")
	_ = f.SetCellValue(sheetName, "D2", "30")

	err = f.SaveAs(filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 1, "Length should be 1")
	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "张三", imported[0].Name, "Should equal expected value")
	assert.Equal(t, "zhang@example.com", imported[0].Email, "Should equal expected value")
	assert.Equal(t, 30, imported[0].Age, "Should equal expected value")
	assert.Equal(t, 0.0, imported[0].Salary, "Should equal expected value")
	assert.Equal(t, 0, imported[0].Status, "Should equal expected value")
	assert.False(t, imported[0].Remark.Valid, "Should not be valid")
}

// TestImportInvalidData tests import invalid data functionality.
func TestImportInvalidData(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_invalid_data_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	f := excelize.NewFile()
	sheetName := "Sheet1"

	_ = f.SetCellValue(sheetName, "A1", "ID")
	_ = f.SetCellValue(sheetName, "B1", "姓名")
	_ = f.SetCellValue(sheetName, "C1", "邮箱")
	_ = f.SetCellValue(sheetName, "D1", "年龄")

	_ = f.SetCellValue(sheetName, "A2", "1")
	_ = f.SetCellValue(sheetName, "B2", "张三")
	_ = f.SetCellValue(sheetName, "C2", "invalid-email")
	_ = f.SetCellValue(sheetName, "D2", "not-a-number")

	err = f.SaveAs(filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, imported, "Should be empty")
	assert.NotEmpty(t, importErrors, "Should not be empty")
}

// TestImportLargeFile tests import large file functionality.
func TestImportLargeFile(t *testing.T) {
	count := 1000
	users := make([]TestUser, count)
	now := time.Now()

	for i := range count {
		users[i] = TestUser{
			ID:        fmt.Sprintf("%d", i+1),
			Name:      fmt.Sprintf("用户%d", i+1),
			Email:     fmt.Sprintf("user%d@example.com", i+1),
			Age:       20 + (i % 50),
			Salary:    5000.0 + float64(i*100),
			CreatedAt: now,
			Status:    i % 3,
		}
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_large_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, count, "Length should be count")

	assert.Equal(t, "1", imported[0].ID, "Should equal expected value")
	assert.Equal(t, "用户1", imported[0].Name, "Should equal expected value")
	assert.Equal(t, fmt.Sprintf("%d", count), imported[count-1].ID, "Should equal expected value")
}

// TestExportNullValues tests export null values functionality.
func TestExportNullValues(t *testing.T) {
	users := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: time.Now(),
			Status:    1,
			Remark:    null.String{},
		},
		{
			ID:        "2",
			Name:      "李四",
			Email:     "li@example.com",
			Age:       25,
			Salary:    8000.00,
			CreatedAt: time.Now(),
			Status:    2,
			Remark:    null.StringFrom("有备注"),
		},
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_null_values_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(users, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, 2, "Length should be 2")

	assert.False(t, imported[0].Remark.Valid, "Should not be valid")
	assert.True(t, imported[1].Remark.Valid, "Should be valid")
	assert.Equal(t, "有备注", imported[1].Remark.ValueOrZero(), "Should equal expected value")
}

// TestRoundTrip tests round trip functionality.
func TestRoundTrip(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local)
	original := []TestUser{
		{
			ID:        "1",
			Name:      "张三",
			Email:     "zhang@example.com",
			Age:       30,
			Salary:    10000.50,
			CreatedAt: now,
			Status:    1,
			Remark:    null.StringFrom("测试用户1"),
		},
		{
			ID:        "2",
			Name:      "李四",
			Email:     "li@example.com",
			Age:       25,
			Salary:    8000.75,
			CreatedAt: now,
			Status:    2,
			Remark:    null.String{},
		},
	}

	exporter := NewExporterFor[TestUser]()
	tmpFile, err := os.CreateTemp("", "test_roundtrip_*.xlsx")
	require.NoError(t, err, "Should not return error")

	filename := tmpFile.Name()
	_ = tmpFile.Close()

	defer os.Remove(filename)

	err = exporter.ExportToFile(original, filename)
	require.NoError(t, err, "Should not return error")

	importer := NewImporterFor[TestUser]()
	importedAny, importErrors, err := importer.ImportFromFile(filename)
	imported, ok := importedAny.([]TestUser)
	require.True(t, ok, "Type assertion to []TestUser should succeed")

	require.NoError(t, err, "Should not return error")
	assert.Empty(t, importErrors, "Should be empty")
	assert.Len(t, imported, len(original), "Length should be len(original)")

	for i := range original {
		assert.Equal(t, original[i].ID, imported[i].ID, "Should equal expected value")
		assert.Equal(t, original[i].Name, imported[i].Name, "Should equal expected value")
		assert.Equal(t, original[i].Email, imported[i].Email, "Should equal expected value")
		assert.Equal(t, original[i].Age, imported[i].Age, "Should equal expected value")
		assert.InDelta(t, original[i].Salary, imported[i].Salary, 0.01, "Salary should be within delta of original value")
		assert.Equal(t, original[i].Status, imported[i].Status, "Should equal expected value")
		assert.Equal(t, original[i].Remark.Valid, imported[i].Remark.Valid, "Should equal expected value")

		if original[i].Remark.Valid {
			assert.Equal(t, original[i].Remark.ValueOrZero(), imported[i].Remark.ValueOrZero(), "Should equal expected value")
		}
	}
}
