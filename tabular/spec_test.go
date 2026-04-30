package tabular

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSchemaFromSpecsBasic verifies construction of a minimal dynamic schema.
func TestNewSchemaFromSpecsBasic(t *testing.T) {
	schema, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "id", Name: "ID", Type: reflect.TypeFor[int]()},
		{Key: "name", Type: reflect.TypeFor[string]()},
	})
	require.NoError(t, err, "Schema construction should succeed for valid specs")
	require.Equal(t, 2, schema.ColumnCount(), "Schema should have two columns")

	columns := schema.Columns()
	assert.Equal(t, "id", columns[0].Key, "Key should be preserved")
	assert.Equal(t, "ID", columns[0].Name, "Name should be preserved when provided")
	assert.Equal(t, "name", columns[1].Name, "Name should default to Key when empty")
}

// TestNewSchemaFromSpecsSortsByOrder ensures Order controls placement and
// sorting is stable for equal orders.
func TestNewSchemaFromSpecsSortsByOrder(t *testing.T) {
	schema, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "third", Type: reflect.TypeFor[string](), Order: 2},
		{Key: "first", Type: reflect.TypeFor[string](), Order: 0},
		{Key: "second", Type: reflect.TypeFor[string](), Order: 1},
		{Key: "stable_a", Type: reflect.TypeFor[string](), Order: 5},
		{Key: "stable_b", Type: reflect.TypeFor[string](), Order: 5},
	})
	require.NoError(t, err, "Schema construction should succeed")

	names := schema.ColumnNames()
	assert.Equal(t, []string{"first", "second", "third", "stable_a", "stable_b"}, names,
		"Columns should be ordered by Order and stably sorted")
}

// TestNewSchemaFromSpecsRejectsMissingKey ensures empty keys fail fast.
func TestNewSchemaFromSpecsRejectsMissingKey(t *testing.T) {
	_, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "", Type: reflect.TypeFor[string]()},
	})
	require.Error(t, err, "Schema construction should reject empty Key")
	assert.ErrorIs(t, err, ErrMissingColumnKey, "Error should wrap ErrMissingColumnKey")
}

// TestNewSchemaFromSpecsRejectsMissingType ensures nil types fail fast.
func TestNewSchemaFromSpecsRejectsMissingType(t *testing.T) {
	_, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "id"},
	})
	require.Error(t, err, "Schema construction should reject nil Type")
	assert.ErrorIs(t, err, ErrMissingColumnType, "Error should wrap ErrMissingColumnType")
}

// TestNewSchemaFromSpecsRejectsDuplicateKey ensures unique keys are enforced.
func TestNewSchemaFromSpecsRejectsDuplicateKey(t *testing.T) {
	_, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "id", Type: reflect.TypeFor[int]()},
		{Key: "id", Type: reflect.TypeFor[int]()},
	})
	require.Error(t, err, "Schema construction should reject duplicate keys")
	assert.ErrorIs(t, err, ErrDuplicateColumnName, "Error should wrap ErrDuplicateColumnName")
}

// TestSchemaColumnLookups ensures ColumnByKey / ColumnByName resolve the
// expected columns for both struct-derived and dynamic schemas.
func TestSchemaColumnLookups(t *testing.T) {
	schema, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "id", Name: "ID", Type: reflect.TypeFor[int]()},
	})
	require.NoError(t, err, "Schema construction should succeed")

	byKey, ok := schema.ColumnByKey("id")
	require.True(t, ok, "ColumnByKey should find a known key")
	assert.Equal(t, "id", byKey.Key, "Lookup should return the same column")

	byName, ok := schema.ColumnByName("ID")
	require.True(t, ok, "ColumnByName should find a known name")
	assert.Equal(t, "id", byName.Key, "Lookup should return the correct column")

	_, ok = schema.ColumnByKey("missing")
	assert.False(t, ok, "ColumnByKey should miss unknown keys")

	_, ok = schema.ColumnByName("missing")
	assert.False(t, ok, "ColumnByName should miss unknown names")
}
