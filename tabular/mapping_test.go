package tabular

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMappingSchema(t *testing.T) *Schema {
	t.Helper()

	schema, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "id", Name: "ID", Type: reflect.TypeFor[int]()},
		{Key: "name", Name: "Name", Type: reflect.TypeFor[string]()},
	})
	require.NoError(t, err, "Schema construction should succeed")

	return schema
}

// TestBuildHeaderMappingMatchesNames covers the common happy path where headers
// line up with schema names.
func TestBuildHeaderMappingMatchesNames(t *testing.T) {
	schema := newMappingSchema(t)

	mapping, err := BuildHeaderMapping([]string{"ID", "Name"}, schema, MappingOptions{})
	require.NoError(t, err, "Mapping should succeed for a well-formed header row")
	assert.Equal(t, map[int]int{0: 0, 1: 1}, mapping, "Mapping should align headers with columns")
}

// TestBuildHeaderMappingTrimsSpace toggles TrimSpace and documents the effect.
func TestBuildHeaderMappingTrimsSpace(t *testing.T) {
	schema := newMappingSchema(t)

	t.Run("Enabled", func(t *testing.T) {
		mapping, err := BuildHeaderMapping([]string{" ID", "Name "}, schema, MappingOptions{TrimSpace: true})
		require.NoError(t, err, "Mapping should succeed when TrimSpace is enabled")
		assert.Equal(t, map[int]int{0: 0, 1: 1}, mapping, "Trimmed headers should still map")
	})

	t.Run("Disabled", func(t *testing.T) {
		mapping, err := BuildHeaderMapping([]string{" ID", "Name "}, schema, MappingOptions{})
		require.NoError(t, err, "Mapping should succeed but skip untrimmed headers")
		assert.Empty(t, mapping, "Untrimmed headers should miss the schema names")
	})
}

// TestBuildHeaderMappingIgnoresUnknownHeaders verifies that extra source
// columns do not error out and are simply dropped.
func TestBuildHeaderMappingIgnoresUnknownHeaders(t *testing.T) {
	schema := newMappingSchema(t)

	mapping, err := BuildHeaderMapping([]string{"ID", "Unknown", "Name"}, schema, MappingOptions{})
	require.NoError(t, err, "Unknown headers should be ignored, not errored")
	assert.Equal(t, map[int]int{0: 0, 2: 1}, mapping, "Only known headers should be mapped")
}

// TestBuildHeaderMappingIgnoresEmptyHeaderCells ensures blank headers are
// silently dropped without colliding with each other.
func TestBuildHeaderMappingIgnoresEmptyHeaderCells(t *testing.T) {
	schema := newMappingSchema(t)

	mapping, err := BuildHeaderMapping([]string{"ID", "", "Name", ""}, schema, MappingOptions{})
	require.NoError(t, err, "Empty headers should not raise an error")
	assert.Equal(t, map[int]int{0: 0, 2: 1}, mapping, "Empty header cells should be skipped")
}

// TestBuildHeaderMappingRejectsDuplicateHeader ensures duplicate header cells
// produce ErrDuplicateColumnName.
func TestBuildHeaderMappingRejectsDuplicateHeader(t *testing.T) {
	schema := newMappingSchema(t)

	_, err := BuildHeaderMapping([]string{"ID", "ID"}, schema, MappingOptions{})
	require.Error(t, err, "Duplicate headers should fail")
	assert.ErrorIs(t, err, ErrDuplicateColumnName, "Error should wrap ErrDuplicateColumnName")
}

// TestDefaultPositionalMapping returns a 1:1 mapping matching the schema size.
func TestDefaultPositionalMapping(t *testing.T) {
	schema := newMappingSchema(t)
	mapping := DefaultPositionalMapping(schema)

	assert.Equal(t, map[int]int{0: 0, 1: 1}, mapping,
		"Default mapping should pair source index i with schema index i")
}
