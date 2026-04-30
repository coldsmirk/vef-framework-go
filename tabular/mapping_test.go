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

// TestBuildHeaderMapping covers the header-to-schema mapping behavior used by
// CSV and Excel importers.
func TestBuildHeaderMapping(t *testing.T) {
	t.Run("MatchesNames", func(t *testing.T) {
		schema := newMappingSchema(t)

		mapping, err := BuildHeaderMapping([]string{"ID", "Name"}, schema, MappingOptions{})
		require.NoError(t, err, "Mapping should succeed for a well-formed header row")
		assert.Equal(t, map[int]int{0: 0, 1: 1}, mapping, "Mapping should align headers with columns")
	})

	t.Run("TrimSpaceEnabled", func(t *testing.T) {
		schema := newMappingSchema(t)

		mapping, err := BuildHeaderMapping([]string{" ID", "Name "}, schema, MappingOptions{TrimSpace: true})
		require.NoError(t, err, "Mapping should succeed when TrimSpace is enabled")
		assert.Equal(t, map[int]int{0: 0, 1: 1}, mapping, "Trimmed headers should still map")
	})

	t.Run("TrimSpaceDisabled", func(t *testing.T) {
		schema := newMappingSchema(t)

		mapping, err := BuildHeaderMapping([]string{" ID", "Name "}, schema, MappingOptions{})
		require.NoError(t, err, "Mapping should succeed but skip untrimmed headers")
		assert.Empty(t, mapping, "Untrimmed headers should miss the schema names")
	})

	t.Run("IgnoresUnknownHeaders", func(t *testing.T) {
		schema := newMappingSchema(t)

		mapping, err := BuildHeaderMapping([]string{"ID", "Unknown", "Name"}, schema, MappingOptions{})
		require.NoError(t, err, "Unknown headers should be ignored, not errored")
		assert.Equal(t, map[int]int{0: 0, 2: 1}, mapping, "Only known headers should be mapped")
	})

	t.Run("IgnoresEmptyHeaderCells", func(t *testing.T) {
		schema := newMappingSchema(t)

		mapping, err := BuildHeaderMapping([]string{"ID", "", "Name", ""}, schema, MappingOptions{})
		require.NoError(t, err, "Empty headers should not raise an error")
		assert.Equal(t, map[int]int{0: 0, 2: 1}, mapping, "Empty header cells should be skipped")
	})

	t.Run("RejectsDuplicateHeader", func(t *testing.T) {
		schema := newMappingSchema(t)

		_, err := BuildHeaderMapping([]string{"ID", "ID"}, schema, MappingOptions{})
		require.Error(t, err, "Duplicate headers should fail")
		assert.ErrorIs(t, err, ErrDuplicateColumnName, "Error should wrap ErrDuplicateColumnName")
	})
}

// TestDefaultPositionalMapping returns a 1:1 mapping matching the schema size.
func TestDefaultPositionalMapping(t *testing.T) {
	schema := newMappingSchema(t)
	mapping := DefaultPositionalMapping(schema)

	assert.Equal(t, map[int]int{0: 0, 1: 1}, mapping,
		"Default mapping should pair source index i with schema index i")
}
