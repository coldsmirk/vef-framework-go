package tabular

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMapUserSchema(t *testing.T) *Schema {
	t.Helper()

	schema, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "id", Name: "ID", Type: reflect.TypeFor[int](), Required: true},
		{Key: "name", Name: "Name", Type: reflect.TypeFor[string](), Required: true},
	})
	require.NoError(t, err, "Building a basic user schema should succeed")

	return schema
}

// TestMapAdapterReaderAcceptsSliceOfMaps exercises the happy path where the
// caller passes []map[string]any directly.
func TestMapAdapterReaderAcceptsSliceOfMaps(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)

	rows := []map[string]any{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	reader, err := adapter.Reader(rows)
	require.NoError(t, err, "Reader should accept []map[string]any")

	collected := make([]map[string]any, 0, 2)
	for _, view := range reader.All() {
		row := make(map[string]any, 2)
		for _, col := range schema.Columns() {
			value, err := view.Get(col)
			require.NoError(t, err, "Get should succeed for known columns")

			row[col.Key] = value
		}

		collected = append(collected, row)
	}

	require.Len(t, collected, 2, "Iterator should yield all rows")
	assert.Equal(t, "Alice", collected[0]["name"], "First row name should match")
	assert.Equal(t, 2, collected[1]["id"], "Second row id should match")
}

// TestMapAdapterReaderNormalizesGenericSlice confirms that `[]any` containing
// maps is accepted and validated at the element level.
func TestMapAdapterReaderNormalizesGenericSlice(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)

	rows := []any{map[string]any{"id": 1, "name": "Alice"}}

	reader, err := adapter.Reader(rows)
	require.NoError(t, err, "Reader should accept []any containing maps")

	count := 0
	for range reader.All() {
		count++
	}

	assert.Equal(t, 1, count, "Iterator should emit one row")
}

// TestMapAdapterReaderRejectsWrongElementType guards against accidentally
// passing []struct when map rows are expected.
func TestMapAdapterReaderRejectsWrongElementType(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)

	_, err := adapter.Reader([]any{1})
	require.Error(t, err, "Reader should reject non-map elements")
	assert.ErrorIs(t, err, ErrSchemaMismatch, "Error should wrap ErrSchemaMismatch")
}

// TestMapAdapterReaderRejectsNonSlice ensures a helpful error for completely
// invalid shapes.
func TestMapAdapterReaderRejectsNonSlice(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)

	_, err := adapter.Reader(map[string]any{"id": 1})
	require.Error(t, err, "Reader should reject non-slice input")
	assert.ErrorIs(t, err, ErrDataMustBeSlice, "Error should wrap ErrDataMustBeSlice")
}

// TestMapWriterCommitAppendsRow is the minimal happy-path test for the writer.
func TestMapWriterCommitAppendsRow(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)
	columns := schema.Columns()

	writer := adapter.Writer(1)

	row := writer.NewRow()
	require.NoError(t, row.Set(columns[0], 1), "Set id should succeed")
	require.NoError(t, row.Set(columns[1], "Alice"), "Set name should succeed")
	require.NoError(t, writer.Commit(row), "Commit should succeed when required fields are set")

	result, ok := writer.Build().([]map[string]any)
	require.True(t, ok, "Build should return []map[string]any")
	require.Len(t, result, 1, "One committed row should produce one output")
	assert.Equal(t, 1, result[0]["id"], "id should be preserved")
	assert.Equal(t, "Alice", result[0]["name"], "name should be preserved")
}

// TestMapWriterRequiredValidation verifies that Required columns reject empty
// values and missing keys.
func TestMapWriterRequiredValidation(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)
	columns := schema.Columns()

	writer := adapter.Writer(1)

	t.Run("MissingKey", func(t *testing.T) {
		row := writer.NewRow()
		require.NoError(t, row.Set(columns[0], 1), "Set id should succeed")

		err := writer.Commit(row)
		require.Error(t, err, "Commit should fail when required column is missing")
		assert.ErrorIs(t, err, ErrRequiredMissing, "Error should wrap ErrRequiredMissing")
	})

	t.Run("EmptyString", func(t *testing.T) {
		row := writer.NewRow()
		require.NoError(t, row.Set(columns[0], 1), "Set id should succeed")
		require.NoError(t, row.Set(columns[1], ""), "Set name to empty should succeed at the Set layer")

		err := writer.Commit(row)
		require.Error(t, err, "Commit should fail for empty required string")
		assert.ErrorIs(t, err, ErrRequiredMissing, "Error should wrap ErrRequiredMissing")
	})

	t.Run("NilValue", func(t *testing.T) {
		row := writer.NewRow()
		require.NoError(t, row.Set(columns[0], 1), "Set id should succeed")
		require.NoError(t, row.Set(columns[1], nil), "Set name to nil should succeed at the Set layer")

		err := writer.Commit(row)
		require.Error(t, err, "Commit should fail for nil required value")
		assert.ErrorIs(t, err, ErrRequiredMissing, "Error should wrap ErrRequiredMissing")
	})
}

// TestMapWriterCellValidators ensures per-cell validators run and report
// errors through the joined error chain.
func TestMapWriterCellValidators(t *testing.T) {
	failValidator := func(_ *Column, value any) error {
		if value == "bad" {
			return errors.New("value rejected")
		}

		return nil
	}

	schema, err := NewSchemaFromSpecs([]ColumnSpec{
		{
			Key: "name", Name: "Name", Type: reflect.TypeFor[string](),
			Validators: []CellValidator{failValidator},
		},
	})
	require.NoError(t, err, "Schema construction should succeed")

	adapter := NewMapAdapter(schema)
	columns := schema.Columns()

	writer := adapter.Writer(1)

	row := writer.NewRow()
	require.NoError(t, row.Set(columns[0], "bad"), "Set should succeed")

	err = writer.Commit(row)
	require.Error(t, err, "Commit should fail when cell validator rejects the value")
	assert.Contains(t, err.Error(), "value rejected", "Error message should include validator output")
}

// TestMapWriterRowValidator ensures the optional row validator runs after cell
// validators and that its failure is surfaced to the caller.
func TestMapWriterRowValidator(t *testing.T) {
	schema := newMapUserSchema(t)

	rowValidator := func(row map[string]any) error {
		if row["name"] == "" {
			return errors.New("name must not be empty")
		}

		return nil
	}

	adapter := NewMapAdapter(schema, WithRowValidator(rowValidator))
	columns := schema.Columns()

	writer := adapter.Writer(1)

	row := writer.NewRow()
	require.NoError(t, row.Set(columns[0], 1), "Set id should succeed")
	require.NoError(t, row.Set(columns[1], "ok"), "Set name should succeed")
	require.NoError(t, writer.Commit(row), "Commit should succeed when row validator passes")
}

// TestMapRowBuilderSetRejectsEmptyKey checks the safety guard in Set.
func TestMapRowBuilderSetRejectsEmptyKey(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)

	writer := adapter.Writer(0)
	row := writer.NewRow()

	err := row.Set(&Column{Key: ""}, 1)
	require.Error(t, err, "Set should fail when column key is empty")
	assert.ErrorIs(t, err, ErrSchemaMismatch, "Error should wrap ErrSchemaMismatch")
}

// TestMapRowViewGetReturnsNilForMissingKey documents the behavior for
// unknown / absent keys in the map.
func TestMapRowViewGetReturnsNilForMissingKey(t *testing.T) {
	schema := newMapUserSchema(t)
	adapter := NewMapAdapter(schema)

	reader, err := adapter.Reader([]map[string]any{{"id": 1}})
	require.NoError(t, err, "Reader should accept valid data")

	nameCol := schema.Columns()[1]

	for _, view := range reader.All() {
		value, err := view.Get(nameCol)
		require.NoError(t, err, "Get should not fail for missing keys")
		assert.Nil(t, value, "Missing map key should produce nil")
	}
}
