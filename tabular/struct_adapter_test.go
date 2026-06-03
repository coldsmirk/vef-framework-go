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

// TestStructAdapter exercises the struct-backed RowAdapter end to end,
// covering schema population, reading, writing, validation and error guards.
func TestStructAdapter(t *testing.T) {
	t.Run("SchemaPopulatesKeyAndType", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()
		columns := adapter.Schema().Columns()

		require.Len(t, columns, 3, "Schema should contain ID, Name, Email columns")
		assert.Equal(t, "ID", columns[0].Key, "Key should mirror struct field name")
		assert.Equal(t, reflect.TypeFor[int](), columns[0].Type, "Type should be derived from field type")
		assert.Equal(t, "Name", columns[1].Key, "Key should mirror struct field name")
		assert.Equal(t, reflect.TypeFor[string](), columns[1].Type, "Type should be derived from field type")
	})

	t.Run("ReaderIteratesAllRows", func(t *testing.T) {
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

		values := make(map[int]StructAdapterUser, 2)
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
	})

	t.Run("ReaderRejectsNonSlice", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()

		_, err := adapter.Reader(StructAdapterUser{})
		require.Error(t, err, "Reader should reject non-slice input")
		assert.ErrorIs(t, err, ErrDataMustBeSlice, "Error should wrap ErrDataMustBeSlice")
	})

	t.Run("ReaderAcceptsNil", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()

		reader, err := adapter.Reader(nil)
		require.NoError(t, err, "Reader should accept nil")

		count := 0
		for range reader.All() {
			count++
		}

		assert.Equal(t, 0, count, "Iterator over nil data should yield no rows")
	})

	t.Run("WriterAccumulatesRows", func(t *testing.T) {
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
	})

	t.Run("CommitValidatesStruct", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()
		columns := adapter.Schema().Columns()
		writer := adapter.Writer(1)

		row := writer.NewRow()
		require.NoError(t, row.Set(columns[0], 1), "Set ID should succeed")
		require.NoError(t, row.Set(columns[1], ""), "Set Name to empty should succeed at the Set layer")
		require.NoError(t, row.Set(columns[2], "invalid"), "Set Email should succeed at the Set layer")

		err := writer.Commit(row)
		require.Error(t, err, "Commit should report struct validator failure")
	})

	t.Run("SetUnsettableField", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()
		writer := adapter.Writer(0)

		row := writer.NewRow()
		err := row.Set(&Column{Key: "bogus"}, 0)
		require.Error(t, err, "Set should fail when column has no Index")
		assert.ErrorIs(t, err, ErrSchemaMismatch, "Error should wrap ErrSchemaMismatch")
	})

	t.Run("SetConvertsAssignable", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()
		columns := adapter.Schema().Columns()
		writer := adapter.Writer(1)

		row := writer.NewRow()
		require.NoError(t, row.Set(columns[0], int64(42)), "Set should accept convertible int64 for int field")

		value := row.Value().(StructAdapterUser)
		assert.Equal(t, 42, value.ID, "Int64 input should be converted to int field")
	})

	t.Run("SetIncompatibleType", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()
		columns := adapter.Schema().Columns()
		writer := adapter.Writer(1)

		row := writer.NewRow()
		err := row.Set(columns[0], []byte{1, 2, 3})
		require.Error(t, err, "Set should reject incompatible types")
		assert.True(t, errors.Is(err, ErrSchemaMismatch), "Error should wrap ErrSchemaMismatch")
	})

	t.Run("ViewGetReturnsFieldValue", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterUser]()
		columns := adapter.Schema().Columns()

		reader, err := adapter.Reader([]StructAdapterUser{{ID: 7, Name: "Zed", Email: "z@example.com"}})
		require.NoError(t, err, "Reader should accept valid slice")

		for _, view := range reader.All() {
			id, err := view.Get(columns[0])
			require.NoError(t, err, "Get should succeed")
			assert.Equal(t, 7, id, "Get should return the field value")
		}
	})
}

type StructAdapterDiveProfile struct {
	City string `tabular:"City"`
	Zip  string `tabular:"Zip"`
}

type StructAdapterDiveValue struct {
	ID      int                      `tabular:"ID"`
	Profile StructAdapterDiveProfile `tabular:"dive"`
}

type StructAdapterDivePointer struct {
	ID      int                       `tabular:"ID"`
	Profile *StructAdapterDiveProfile `tabular:"dive"`
}

// TestStructAdapterDive verifies that dive paths through both value- and
// pointer-embedded structs read and write without panicking, including the
// nil-embedded-pointer cases that previously crashed FieldByIndex.
func TestStructAdapterDive(t *testing.T) {
	t.Run("ReadValueEmbedded", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterDiveValue]()
		columns := adapter.Schema().Columns()
		require.Len(t, columns, 3, "value dive should expose ID plus two nested columns")

		rows := []StructAdapterDiveValue{{ID: 1, Profile: StructAdapterDiveProfile{City: "Shanghai", Zip: "200000"}}}
		reader, err := adapter.Reader(rows)
		require.NoError(t, err, "Reader should accept []T with a value-embedded struct")

		for _, view := range reader.All() {
			city, err := view.Get(columns[1])
			require.NoError(t, err, "Get should read a dived value field")
			assert.Equal(t, "Shanghai", city, "Dived value field should return the nested value")
		}
	})

	t.Run("ReadNilPointerEmbedded", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterDivePointer]()
		columns := adapter.Schema().Columns()
		require.Len(t, columns, 3, "pointer dive should expose ID plus two nested columns")

		// Profile is nil: reading a dived field must not panic.
		rows := []StructAdapterDivePointer{{ID: 1}}
		reader, err := adapter.Reader(rows)
		require.NoError(t, err, "Reader should accept []T with a nil pointer-embedded struct")

		for _, view := range reader.All() {
			city, err := view.Get(columns[1])
			require.NoError(t, err, "Get through a nil embedded pointer should not error")
			assert.Nil(t, city, "Dived field of a nil embedded pointer should read as nil")
		}
	})

	t.Run("ReadPopulatedPointerEmbedded", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterDivePointer]()
		columns := adapter.Schema().Columns()

		rows := []StructAdapterDivePointer{{ID: 1, Profile: &StructAdapterDiveProfile{City: "Beijing", Zip: "100000"}}}
		reader, err := adapter.Reader(rows)
		require.NoError(t, err, "Reader should accept a populated pointer-embedded struct")

		for _, view := range reader.All() {
			city, err := view.Get(columns[1])
			require.NoError(t, err, "Get should read a dived field of a populated embedded pointer")
			assert.Equal(t, "Beijing", city, "Dived field should return the nested pointer value")
		}
	})

	t.Run("WriteValueEmbedded", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterDiveValue]()
		columns := adapter.Schema().Columns()
		writer := adapter.Writer(1)

		row := writer.NewRow()
		require.NoError(t, row.Set(columns[0], 5), "Set top-level field should succeed")
		require.NoError(t, row.Set(columns[1], "Shenzhen"), "Set dived value field should succeed")
		require.NoError(t, writer.Commit(row), "Commit should succeed for a value dive row")

		result, ok := writer.Build().([]StructAdapterDiveValue)
		require.True(t, ok, "Build should return []StructAdapterDiveValue")
		require.Len(t, result, 1, "One committed row should produce one output")
		assert.Equal(t, "Shenzhen", result[0].Profile.City, "Dived value field should be written into the nested struct")
	})

	t.Run("WriteNilPointerEmbeddedAllocates", func(t *testing.T) {
		adapter := NewStructAdapterFor[StructAdapterDivePointer]()
		columns := adapter.Schema().Columns()
		writer := adapter.Writer(1)

		// NewRow starts with a nil *Profile; Set into a dived field must allocate
		// the intermediate pointer rather than panic.
		row := writer.NewRow()
		require.NoError(t, row.Set(columns[0], 9), "Set top-level field should succeed")
		require.NoError(t, row.Set(columns[1], "Guangzhou"), "Set into a nil embedded pointer should allocate and succeed")
		require.NoError(t, writer.Commit(row), "Commit should succeed for a pointer dive row")

		result, ok := writer.Build().([]StructAdapterDivePointer)
		require.True(t, ok, "Build should return []StructAdapterDivePointer")
		require.Len(t, result, 1, "One committed row should produce one output")
		require.NotNil(t, result[0].Profile, "Set should have allocated the embedded pointer")
		assert.Equal(t, "Guangzhou", result[0].Profile.City, "Dived field should be written into the allocated nested struct")
	})
}
