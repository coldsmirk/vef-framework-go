package orm

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// Test models with different PK types for PKField testing.
type StringPKModel struct {
	bun.BaseModel `bun:"table:test_string_pk"`

	ID   string `bun:"id,pk"`
	Name string `bun:"name"`
}

type IntPKModel struct {
	bun.BaseModel `bun:"table:test_int_pk"`

	ID   int64  `bun:"id,pk"`
	Name string `bun:"name"`
}

type Int32PKModel struct {
	bun.BaseModel `bun:"table:test_int32_pk"`

	ID   int32  `bun:"id,pk"`
	Name string `bun:"name"`
}

type FloatPKModel struct {
	bun.BaseModel `bun:"table:test_float_pk"`

	ID   float64 `bun:"id,pk"`
	Name string  `bun:"name"`
}

func newTestBunDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	require.NoError(t, err, "Should open SQLite in-memory database")

	t.Cleanup(func() {
		require.NoError(t, sqldb.Close(), "Should close SQLite in-memory database")
	})

	return bun.NewDB(sqldb, sqlitedialect.New())
}

func newPKFieldForModel(t *testing.T, db *bun.DB, model any) *PKField {
	t.Helper()

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}

	table := db.Table(modelType)
	require.True(t, len(table.PKs) > 0, "Model must have at least one primary key")

	return NewPKField(table.PKs[0])
}

// TestPKFieldValidateModel tests PK field validate model scenarios.
func TestPKFieldValidateModel(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*StringPKModel)(nil))

	t.Run("ValidPointerToStruct", func(t *testing.T) {
		model := &StringPKModel{ID: "test-id"}

		val, err := pkField.Value(model)
		assert.NoError(t, err, "Should extract value from valid pointer model")
		assert.Equal(t, "test-id", val, "Should return correct PK value")
	})

	t.Run("NonPointerModel", func(t *testing.T) {
		_, err := pkField.Value(StringPKModel{ID: "test"})
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-pointer model")
	})

	t.Run("PointerToNonStruct", func(t *testing.T) {
		s := "not-a-struct"

		_, err := pkField.Value(&s)
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject pointer to non-struct")
	})

	t.Run("ReflectValuePointerToStruct", func(t *testing.T) {
		model := &StringPKModel{ID: "reflect-test"}

		val, err := pkField.Value(reflect.ValueOf(model))
		assert.NoError(t, err, "Should accept reflect.Value pointer to struct")
		assert.Equal(t, "reflect-test", val, "Should return correct PK value")
	})

	t.Run("ReflectValueNonPointerNonAddressable", func(t *testing.T) {
		model := StringPKModel{ID: "test"}

		_, err := pkField.Value(reflect.ValueOf(model))
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-addressable reflect.Value")
	})

	t.Run("ReflectValuePointerToNonStruct", func(t *testing.T) {
		s := "not-a-struct"

		_, err := pkField.Value(reflect.ValueOf(&s))
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject reflect.Value pointer to non-struct")
	})

	t.Run("ReflectValueAddressableStruct", func(t *testing.T) {
		model := StringPKModel{ID: "addressable-test"}
		// Create an addressable reflect.Value by using reflect.New
		rv := reflect.New(reflect.TypeFor[StringPKModel]()).Elem()
		rv.Set(reflect.ValueOf(model))

		val, err := pkField.Value(rv)
		assert.NoError(t, err, "Should accept addressable reflect.Value struct")
		assert.Equal(t, "addressable-test", val, "Should return correct PK value")
	})
}

// TestPKFieldSetStringType verifies PKField.Set for string-typed primary keys.
func TestPKFieldSetStringType(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*StringPKModel)(nil))

	t.Run("SetStringValue", func(t *testing.T) {
		model := &StringPKModel{}

		err := pkField.Set(model, "new-id")
		assert.NoError(t, err, "Should set string PK value")
		assert.Equal(t, "new-id", model.ID, "Should update model ID field")
	})

	t.Run("SetIntAsString", func(t *testing.T) {
		model := &StringPKModel{}

		err := pkField.Set(model, 42)
		assert.NoError(t, err, "Should convert int to string for string PK")
		assert.Equal(t, "42", model.ID, "Should store converted string value")
	})

	t.Run("InvalidModel", func(t *testing.T) {
		err := pkField.Set(StringPKModel{}, "value")
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-pointer model")
	})
}

// TestPKFieldSetIntTypes verifies PKField.Set for integer-typed primary keys.
func TestPKFieldSetIntTypes(t *testing.T) {
	db := newTestBunDB(t)

	t.Run("Int64PK", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*IntPKModel)(nil))
		model := &IntPKModel{}

		err := pkField.Set(model, 42)
		assert.NoError(t, err, "Should set int64 PK value")
		assert.Equal(t, int64(42), model.ID, "Should store correct int64 value")
	})

	t.Run("Int64PKFromString", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*IntPKModel)(nil))
		model := &IntPKModel{}

		err := pkField.Set(model, "123")
		assert.NoError(t, err, "Should parse string to int64 PK")
		assert.Equal(t, int64(123), model.ID, "Should store parsed int64 value")
	})

	t.Run("Int32PK", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*Int32PKModel)(nil))
		model := &Int32PKModel{}

		err := pkField.Set(model, 99)
		assert.NoError(t, err, "Should set int32 PK value")
		assert.Equal(t, int32(99), model.ID, "Should store correct int32 value")
	})

	t.Run("Int64PKInvalidValue", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*IntPKModel)(nil))
		model := &IntPKModel{}

		err := pkField.Set(model, "not-a-number")
		assert.Error(t, err, "Should reject non-numeric string for int PK")
	})
}

// TestPKFieldSetUnsupportedType verifies PKField.Set rejects unsupported PK types.
func TestPKFieldSetUnsupportedType(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*FloatPKModel)(nil))
	model := &FloatPKModel{}

	err := pkField.Set(model, 3.14)
	assert.ErrorIs(t, err, ErrPrimaryKeyUnsupportedType, "Should reject unsupported PK type")
}

// TestPKFieldValueErrors verifies PKField.Value returns errors for invalid models.
func TestPKFieldValueErrors(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*StringPKModel)(nil))

	t.Run("NonPointerModel", func(t *testing.T) {
		_, err := pkField.Value(StringPKModel{})
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-pointer model")
	})

	t.Run("PointerToNonStruct", func(t *testing.T) {
		n := 42

		_, err := pkField.Value(&n)
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject pointer to non-struct")
	})
}

// TestNewPKField verifies NewPKField populates Field, Column, and Name correctly.
func TestNewPKField(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*StringPKModel)(nil))

	assert.Equal(t, "ID", pkField.Field, "Should use struct field name")
	assert.Equal(t, "id", pkField.Column, "Should use bun column tag")
	assert.Equal(t, "id", pkField.Name, "Should use column name as display name")
}

// ---------------------------------------------------------------------------
// parsePKValues — unit tests for PK value normalization logic.
// ---------------------------------------------------------------------------

func TestParsePKValues(t *testing.T) {
	t.Run("SinglePK", func(t *testing.T) {
		t.Run("ScalarString", func(t *testing.T) {
			result := parsePKValues("test", "id1", 1)

			assert.Equal(t, []any{"id1"}, result, "Scalar should be wrapped in single-element slice")
		})

		t.Run("ScalarInt", func(t *testing.T) {
			result := parsePKValues("test", 42, 1)

			assert.Equal(t, []any{42}, result, "Int scalar should be wrapped in single-element slice")
		})

		t.Run("SingleElementSlice", func(t *testing.T) {
			// Regression: was incorrectly wrapped as []any{[]string{"id1"}}
			result := parsePKValues("test", []string{"id1"}, 1)

			assert.Len(t, result, 1, "Should have one element")
			assert.Equal(t, "id1", result[0], "Element should be the string value, not the slice")
		})

		t.Run("MultiElementSlice", func(t *testing.T) {
			result := parsePKValues("test", []string{"id1", "id2", "id3"}, 1)

			assert.Equal(t, []any{"id1", "id2", "id3"}, result, "Should expand slice elements for IN operation")
		})

		t.Run("IntSlice", func(t *testing.T) {
			result := parsePKValues("test", []int{1, 2, 3}, 1)

			assert.Len(t, result, 3, "Should expand int slice for IN operation")
			assert.Equal(t, 1, result[0], "First element should be 1")
			assert.Equal(t, 2, result[1], "Second element should be 2")
			assert.Equal(t, 3, result[2], "Third element should be 3")
		})
	})

	t.Run("CompositePK", func(t *testing.T) {
		t.Run("SingleTuple", func(t *testing.T) {
			result := parsePKValues("test", []any{"usr001", "post001"}, 2)

			require.Len(t, result, 1, "Single composite tuple should be wrapped")
			assert.Equal(t, []any{"usr001", "post001"}, result[0], "Element should be the original tuple slice")
		})

		t.Run("MultipleTuples", func(t *testing.T) {
			result := parsePKValues("test", [][]any{
				{"usr001", "post001"},
				{"usr002", "post002"},
			}, 2)

			require.Len(t, result, 2, "Should expand to two tuple elements")
			assert.Equal(t, []any{"usr001", "post001"}, result[0], "First tuple should match")
			assert.Equal(t, []any{"usr002", "post002"}, result[1], "Second tuple should match")
		})

		t.Run("SliceOfSlicesMatchingPKCount", func(t *testing.T) {
			// When first element IS a slice, it's treated as a list of tuples even if n == pkCount
			result := parsePKValues("test", [][]any{
				{"usr001", "post001"},
				{"usr002", "post002"},
			}, 2)

			require.Len(t, result, 2, "Slice-of-slices should be expanded regardless of pkCount match")
		})
	})

	t.Run("EmptySlicePanics", func(t *testing.T) {
		assert.Panics(t, func() {
			parsePKValues("test", []string{}, 1)
		}, "Empty slice should panic")
	})
}

// ---------------------------------------------------------------------------
// pkValues.AppendQuery — unit tests for SQL value rendering.
// ---------------------------------------------------------------------------

func TestPKValuesAppendQuery(t *testing.T) {
	gen := newTestQueryGen()

	t.Run("SingleScalarValue", func(t *testing.T) {
		pv := &pkValues{values: []any{"id1"}}

		b, err := pv.AppendQuery(gen, nil)

		require.NoError(t, err, "Should append without error")
		assert.Equal(t, "'id1'", string(b), "Should render single quoted value")
	})

	t.Run("MultipleScalarValues", func(t *testing.T) {
		pv := &pkValues{values: []any{"id1", "id2"}}

		b, err := pv.AppendQuery(gen, nil)

		require.NoError(t, err, "Should append without error")
		assert.Equal(t, "'id1', 'id2'", string(b), "Should render comma-separated values")
	})

	t.Run("SingleCompositeTuple", func(t *testing.T) {
		// Regression: was rendered as '["usr001","post001"]' (JSON string)
		pv := &pkValues{values: []any{[]any{"usr001", "post001"}}}

		b, err := pv.AppendQuery(gen, nil)

		require.NoError(t, err, "Should append without error")
		assert.Equal(t, "('usr001', 'post001')", string(b), "Should render as SQL tuple")
	})

	t.Run("MultipleCompositeTuples", func(t *testing.T) {
		pv := &pkValues{values: []any{
			[]any{"usr001", "post001"},
			[]any{"usr002", "post002"},
		}}

		b, err := pv.AppendQuery(gen, nil)

		require.NoError(t, err, "Should append without error")
		assert.Equal(t, "('usr001', 'post001'), ('usr002', 'post002')", string(b), "Should render multiple tuples")
	})

	t.Run("IntegerValues", func(t *testing.T) {
		pv := &pkValues{values: []any{1, 2, 3}}

		b, err := pv.AppendQuery(gen, nil)

		require.NoError(t, err, "Should append without error")
		assert.Equal(t, "1, 2, 3", string(b), "Should render integer values without quotes")
	})

	t.Run("MixedTypeTuple", func(t *testing.T) {
		pv := &pkValues{values: []any{[]any{"usr001", 42}}}

		b, err := pv.AppendQuery(gen, nil)

		require.NoError(t, err, "Should append without error")
		assert.Equal(t, "('usr001', 42)", string(b), "Should render mixed-type tuple")
	})
}
