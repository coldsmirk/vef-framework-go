package csv

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

type csvFactoryUser struct {
	ID   int    `tabular:"用户ID"`
	Name string `tabular:"姓名"`
}

// TestNewImporterFor verifies the struct-typed importer factory wires the
// StructAdapter correctly and returns a tabular.Importer.
func TestNewImporterFor(t *testing.T) {
	importer := NewImporterFor[csvFactoryUser]()
	require.NotNil(t, importer, "Factory should return a non-nil importer")
	assert.Implements(t, (*tabular.Importer)(nil), importer, "Result should implement tabular.Importer")
}

// TestNewExporterFor verifies the struct-typed exporter factory wires the
// StructAdapter correctly and returns a tabular.Exporter.
func TestNewExporterFor(t *testing.T) {
	exporter := NewExporterFor[csvFactoryUser]()
	require.NotNil(t, exporter, "Factory should return a non-nil exporter")
	assert.Implements(t, (*tabular.Exporter)(nil), exporter, "Result should implement tabular.Exporter")
}

// TestNewTypedImporterFor verifies the typed wrapper factory returns a
// generic TypedImporter[T] that re-wraps the underlying importer.
func TestNewTypedImporterFor(t *testing.T) {
	typed := NewTypedImporterFor[csvFactoryUser]()
	require.NotNil(t, typed, "Factory should return a non-nil typed importer")
}

// TestNewTypedExporterFor verifies the typed wrapper factory returns a
// generic TypedExporter[T] that re-wraps the underlying exporter.
func TestNewTypedExporterFor(t *testing.T) {
	typed := NewTypedExporterFor[csvFactoryUser]()
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
