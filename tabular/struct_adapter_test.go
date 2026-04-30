package tabular

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StructAdapterUser struct {
	ID    int    `tabular:"ID"    validate:"required"`
	Name  string `tabular:"Name"  validate:"required"`
	Email string `tabular:"Email" validate:"email"`
}

// TestStructAdapterSchemaPopulatesKeyAndType verifies that struct-derived
// schemas populate Key and Type for every column.
func TestStructAdapterSchemaPopulatesKeyAndType(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()
	columns := adapter.Schema().Columns()

	require.Len(t, columns, 3, "Schema should contain ID, Name, Email columns")

	assert.Equal(t, "ID", columns[0].Key, "Key should mirror struct field name")
	assert.Equal(t, reflect.TypeFor[int](), columns[0].Type, "Type should be parsed from field type")
	assert.Equal(t, "Name", columns[1].Key, "Key should mirror struct field name")
	assert.Equal(t, reflect.TypeFor[string](), columns[1].Type, "Type should be parsed from field type")
}

// TestStructAdapterReaderIteratesAllRows ensures Reader walks every element
// of the slice while exposing the correct underlying values.
func TestStructAdapterReaderIteratesAllRows(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()

	users := []StructAdapterUser{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}

	reader, err := adapter.Reader(users)
	require.NoError(t, err, "Reader should accept []T input")

	columns := adapter.Schema().Columns()
	idCol := columns[0]
	nameCol := columns[1]

	values := make(map[int]StructAdapterUser)

	for idx, view := range reader.All() {
		id, err := view.Get(idCol)
		require.NoError(t, err, "Get should not fail for a valid column")

		name, err := view.Get(nameCol)
		require.NoError(t, err, "Get should not fail for a valid column")

		values[idx] = StructAdapterUser{ID: id.(int), Name: name.(string)}
	}

	assert.Equal(t, 1, values[0].ID, "First row ID should match input")
	assert.Equal(t, "Alice", values[0].Name, "First row Name should match input")
	assert.Equal(t, 2, values[1].ID, "Second row ID should match input")
	assert.Equal(t, "Bob", values[1].Name, "Second row Name should match input")
}

// TestStructAdapterReaderRejectsNonSlice asserts non-slice inputs produce
// ErrDataMustBeSlice to fail fast for obvious mistakes.
func TestStructAdapterReaderRejectsNonSlice(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()

	_, err := adapter.Reader(StructAdapterUser{})
	require.Error(t, err, "Reader should reject non-slice input")
	assert.ErrorIs(t, err, ErrDataMustBeSlice, "Error should wrap ErrDataMustBeSlice")
}

// TestStructAdapterReaderAcceptsNil ensures nil data produces an empty iterator.
func TestStructAdapterReaderAcceptsNil(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()

	reader, err := adapter.Reader(nil)
	require.NoError(t, err, "Reader should accept nil")

	count := 0
	for range reader.All() {
		count++
	}

	assert.Equal(t, 0, count, "Iterator over nil data should yield no rows")
}

// TestStructAdapterWriterAccumulatesRows verifies the writer collects rows via
// Commit and returns a typed []T from Build.
func TestStructAdapterWriterAccumulatesRows(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()
	columns := adapter.Schema().Columns()
	writer := adapter.Writer(2)

	first := writer.NewRow()
	require.NoError(t, first.Set(columns[0], 1), "Set ID should succeed")
	require.NoError(t, first.Set(columns[1], "Alice"), "Set Name should succeed")
	require.NoError(t, first.Set(columns[2], "alice@example.com"), "Set Email should succeed")
	require.NoError(t, writer.Commit(first), "Commit should succeed for a valid row")

	second := writer.NewRow()
	require.NoError(t, second.Set(columns[0], 2), "Set ID should succeed")
	require.NoError(t, second.Set(columns[1], "Bob"), "Set Name should succeed")
	require.NoError(t, second.Set(columns[2], "bob@example.com"), "Set Email should succeed")
	require.NoError(t, writer.Commit(second), "Commit should succeed for a valid row")

	result, ok := writer.Build().([]StructAdapterUser)
	require.True(t, ok, "Build should return []T")
	require.Len(t, result, 2, "Two committed rows should produce two outputs")
	assert.Equal(t, "Alice", result[0].Name, "First committed row should be first output")
	assert.Equal(t, "Bob", result[1].Name, "Second committed row should be second output")
}

// TestStructAdapterCommitValidatesStruct exercises the struct validator path.
// An invalid row (missing required fields, invalid email) is rejected via Commit.
func TestStructAdapterCommitValidatesStruct(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()
	columns := adapter.Schema().Columns()
	writer := adapter.Writer(1)

	row := writer.NewRow()
	require.NoError(t, row.Set(columns[0], 1), "Set ID should succeed")
	require.NoError(t, row.Set(columns[1], ""), "Set Name to empty string should succeed for Set")
	require.NoError(t, row.Set(columns[2], "invalid"), "Set Email should succeed for Set")

	err := writer.Commit(row)
	require.Error(t, err, "Commit should report struct validation failure")
}

// TestStructRowBuilderSetUnsettableField confirms we surface a clear error
// when the caller tries to set an unsettable path.
func TestStructRowBuilderSetUnsettableField(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()
	writer := adapter.Writer(0)

	row := writer.NewRow()
	// Use a bogus column with no Index to trigger the schema-mismatch guard.
	err := row.Set(&Column{Key: "bogus"}, 0)
	require.Error(t, err, "Set should fail when column has no Index")
	assert.ErrorIs(t, err, ErrSchemaMismatch, "Error should wrap ErrSchemaMismatch")
}

// TestStructRowBuilderSetConvertsAssignable validates that the builder
// converts assignable / convertible types when the parser returns a slightly
// different type (e.g. int64 -> int).
func TestStructRowBuilderSetConvertsAssignable(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()
	columns := adapter.Schema().Columns()
	writer := adapter.Writer(1)

	row := writer.NewRow()
	require.NoError(t, row.Set(columns[0], int64(42)), "Set should accept convertible int64 for int field")

	value := row.Value().(StructAdapterUser)
	assert.Equal(t, 42, value.ID, "int64 input should be converted to int field")
}

// TestStructRowBuilderSetIncompatibleType ensures that an actually
// incompatible value is rejected.
func TestStructRowBuilderSetIncompatibleType(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()
	columns := adapter.Schema().Columns()
	writer := adapter.Writer(1)

	row := writer.NewRow()
	err := row.Set(columns[0], []byte{1, 2, 3})
	require.Error(t, err, "Set should reject incompatible types")
	assert.True(t, errors.Is(err, ErrSchemaMismatch), "Error should wrap ErrSchemaMismatch")
}

// TestStructRowViewGetReturnsFieldValue checks the read path using FieldByIndex.
func TestStructRowViewGetReturnsFieldValue(t *testing.T) {
	adapter := NewStructAdapterFor[StructAdapterUser]()
	columns := adapter.Schema().Columns()

	reader, err := adapter.Reader([]StructAdapterUser{{ID: 7, Name: "Zed", Email: "z@example.com"}})
	require.NoError(t, err, "Reader should accept valid slice")

	for _, view := range reader.All() {
		id, err := view.Get(columns[0])
		require.NoError(t, err, "Get should succeed")
		assert.Equal(t, 7, id, "Get should return the field value")
	}
}
