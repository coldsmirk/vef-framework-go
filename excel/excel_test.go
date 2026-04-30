package excel

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

// ExcelTestUser is the shared struct fixture used across the importer,
// exporter and factory tests in this package.
type ExcelTestUser struct {
	ID        string    `tabular:"width=15"                                      validate:"required"`
	Name      string    `tabular:"姓名,width=20"                                   validate:"required"`
	Email     string    `tabular:"邮箱,width=25"                                   validate:"email"`
	Age       int       `tabular:"name=年龄,width=10"                              validate:"gte=0,lte=150"`
	Salary    float64   `tabular:"name=薪资,width=15,format=%.2f"`
	CreatedAt time.Time `tabular:"name=创建时间,width=20,format=2006-01-02 15:04:05"`
	Status    int       `tabular:"name=状态,width=10"`
	Remark    *string   `tabular:"name=备注,width=30"`
	Password  string    `tabular:"-"` // Ignored field
}

// ExcelNoTagStruct exercises auto-derived schemas where struct fields lack
// any `tabular` tag.
type ExcelNoTagStruct struct {
	ID   string
	Name string
	Age  int
}

// baseDynamicSpecs returns the shared dynamic schema used by the map-based
// importer/exporter tests.
func baseDynamicSpecs() []tabular.ColumnSpec {
	return []tabular.ColumnSpec{
		{Key: "id", Name: "用户ID", Type: reflect.TypeFor[int](), Required: true, Width: 12},
		{Key: "name", Name: "姓名", Type: reflect.TypeFor[string](), Required: true, Width: 18},
		{Key: "birthday", Name: "生日", Type: reflect.TypeFor[time.Time](), Format: "2006-01-02", Width: 20},
		{Key: "active", Name: "激活", Type: reflect.TypeFor[bool](), Default: "false", Width: 8},
	}
}

// TestNewImporterFor verifies the struct-typed importer factory wires the
// StructAdapter correctly and returns a tabular.Importer.
func TestNewImporterFor(t *testing.T) {
	importer := NewImporterFor[ExcelTestUser]()
	require.NotNil(t, importer, "Factory should return a non-nil importer")
	assert.Implements(t, (*tabular.Importer)(nil), importer, "Result should implement tabular.Importer")
}

// TestNewExporterFor verifies the struct-typed exporter factory wires the
// StructAdapter correctly and returns a tabular.Exporter.
func TestNewExporterFor(t *testing.T) {
	exporter := NewExporterFor[ExcelTestUser]()
	require.NotNil(t, exporter, "Factory should return a non-nil exporter")
	assert.Implements(t, (*tabular.Exporter)(nil), exporter, "Result should implement tabular.Exporter")
}

// TestNewTypedImporterFor verifies the typed wrapper factory returns a
// generic TypedImporter[T].
func TestNewTypedImporterFor(t *testing.T) {
	typed := NewTypedImporterFor[ExcelTestUser]()
	require.NotNil(t, typed, "Factory should return a non-nil typed importer")
}

// TestNewTypedExporterFor verifies the typed wrapper factory returns a
// generic TypedExporter[T].
func TestNewTypedExporterFor(t *testing.T) {
	typed := NewTypedExporterFor[ExcelTestUser]()
	require.NotNil(t, typed, "Factory should return a non-nil typed exporter")
}

// TestNewMapImporter verifies that the dynamic importer factory propagates
// schema validation errors and accepts MapAdapter options.
func TestNewMapImporter(t *testing.T) {
	t.Run("AcceptsValidSpecs", func(t *testing.T) {
		imp, err := NewMapImporter([]tabular.ColumnSpec{
			{Key: "id", Name: "ID", Type: reflect.TypeFor[int]()},
		}, nil)
		require.NoError(t, err, "NewMapImporter should accept valid specs")
		require.NotNil(t, imp, "Factory should return a non-nil importer")
	})

	t.Run("PropagatesSpecValidationError", func(t *testing.T) {
		_, err := NewMapImporter([]tabular.ColumnSpec{{Key: "id"}}, nil)
		require.Error(t, err, "NewMapImporter should reject specs with missing type")
		assert.ErrorIs(t, err, tabular.ErrMissingColumnType, "Error should wrap ErrMissingColumnType")
	})

	t.Run("AcceptsMapOptions", func(t *testing.T) {
		imp, err := NewMapImporter(
			[]tabular.ColumnSpec{{Key: "id", Name: "ID", Type: reflect.TypeFor[int]()}},
			[]tabular.MapOption{tabular.WithRowValidator(func(map[string]any) error { return nil })},
		)
		require.NoError(t, err, "NewMapImporter should accept MapOption values")
		require.NotNil(t, imp, "Factory should return a non-nil importer")
	})
}

// TestNewMapExporter verifies that the dynamic exporter factory propagates
// schema validation errors.
func TestNewMapExporter(t *testing.T) {
	t.Run("AcceptsValidSpecs", func(t *testing.T) {
		exp, err := NewMapExporter([]tabular.ColumnSpec{
			{Key: "ts", Name: "Timestamp", Type: reflect.TypeFor[time.Time](), Format: time.RFC3339},
		})
		require.NoError(t, err, "NewMapExporter should accept valid specs")
		require.NotNil(t, exp, "Factory should return a non-nil exporter")
	})

	t.Run("PropagatesSpecValidationError", func(t *testing.T) {
		_, err := NewMapExporter([]tabular.ColumnSpec{{Key: "id"}})
		require.Error(t, err, "NewMapExporter should reject specs with missing type")
		assert.ErrorIs(t, err, tabular.ErrMissingColumnType, "Error should wrap ErrMissingColumnType")
	})
}
