package tabular

import (
	"fmt"
	"iter"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/validator"
)

// structAdapter bridges []T data (where T is a struct) with the tabular engine.
// The adapter derives its Schema from the struct's `tabular` tags.
type structAdapter struct {
	schema *Schema
	typ    reflect.Type
}

// NewStructAdapter creates a RowAdapter bound to the given struct type.
// Non-struct types produce a schema with no columns; the resulting adapter
// still satisfies the interface but Reader/Writer calls will return zero rows.
func NewStructAdapter(typ reflect.Type) RowAdapter {
	return &structAdapter{
		schema: NewSchema(typ),
		typ:    typ,
	}
}

// NewStructAdapterFor creates a StructAdapter for the concrete type T.
func NewStructAdapterFor[T any]() RowAdapter {
	return NewStructAdapter(reflect.TypeFor[T]())
}

// Schema returns the columns parsed from struct tags.
func (a *structAdapter) Schema() *Schema {
	return a.schema
}

// Reader iterates a slice of struct values, yielding structRowView for each row.
// Element types must match the adapter's struct type, a pointer to it, or be an
// interface (in which case per-row types are validated during iteration).
func (a *structAdapter) Reader(data any) (RowReader, error) {
	if data == nil {
		return &structReader{typ: a.typ}, nil
	}

	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("%w, got %s", ErrDataMustBeSlice, dataValue.Kind())
	}

	elemType := dataValue.Type().Elem()
	switch {
	case elemType == a.typ:
	case elemType.Kind() == reflect.Pointer && elemType.Elem() == a.typ:
	case elemType.Kind() == reflect.Interface:
		// []any — element types are validated per row when Get is called.
	default:
		return nil, fmt.Errorf("%w: slice element type %s, want %s or *%s",
			ErrSchemaMismatch, elemType, a.typ, a.typ)
	}

	return &structReader{values: dataValue, typ: a.typ}, nil
}

// Writer creates a builder-backed slice accumulator.
func (a *structAdapter) Writer(capacity int) RowWriter {
	if capacity < 0 {
		capacity = 0
	}

	sliceType := reflect.SliceOf(a.typ)

	return &structWriter{
		typ:    a.typ,
		result: reflect.MakeSlice(sliceType, 0, capacity),
	}
}

// structReader iterates a reflect.Value slice.
type structReader struct {
	values reflect.Value
	typ    reflect.Type
}

// All yields each element as a structRowView.
func (r *structReader) All() iter.Seq2[int, RowView] {
	return func(yield func(int, RowView) bool) {
		if !r.values.IsValid() {
			return
		}

		for i := range r.values.Len() {
			elem := r.values.Index(i)
			// Dereference interface / pointer wrappers so FieldByIndex can work.
			for elem.Kind() == reflect.Interface || elem.Kind() == reflect.Pointer {
				if elem.IsNil() {
					break
				}

				elem = elem.Elem()
			}

			// Skip nil interface / pointer elements rather than yielding a
			// zero-valued row that callers cannot distinguish from real data.
			if !elem.IsValid() || elem.Kind() != reflect.Struct {
				continue
			}

			if !yield(i, &structRowView{elem: elem, typ: r.typ}) {
				return
			}
		}
	}
}

// structRowView exposes struct fields using Column.Index.
type structRowView struct {
	elem reflect.Value
	typ  reflect.Type
}

// Get reads the struct field addressed by col.Index.
func (v *structRowView) Get(column *Column) (any, error) {
	if !v.elem.IsValid() {
		return nil, nil
	}

	if v.elem.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: row element kind %s, want struct",
			ErrSchemaMismatch, v.elem.Kind())
	}

	if v.typ != nil && v.elem.Type() != v.typ {
		return nil, fmt.Errorf("%w: row element type %s, want %s",
			ErrSchemaMismatch, v.elem.Type(), v.typ)
	}

	if len(column.Index) == 0 {
		return nil, fmt.Errorf("%w: struct column %q has no Index",
			ErrSchemaMismatch, column.Key)
	}

	// FieldByIndexErr (instead of FieldByIndex) returns an error rather than
	// panicking when the path steps through a nil embedded pointer.
	field, err := v.elem.FieldByIndexErr(column.Index)
	if err != nil {
		// A nil pointer along a dive path means the nested value is absent;
		// surface it as an empty cell rather than an error so export keeps
		// working for partially-populated rows.
		//nolint:nilerr // absent nested value is an empty cell, not an error
		return nil, nil
	}

	return field.Interface(), nil
}

// structWriter accumulates a reflect slice of struct rows.
type structWriter struct {
	typ    reflect.Type
	result reflect.Value
}

// NewRow allocates a zero-valued struct element that can be addressed via FieldByIndex.
func (w *structWriter) NewRow() RowBuilder {
	return &structRowBuilder{typ: w.typ, value: reflect.New(w.typ).Elem()}
}

// Commit validates the row with the framework validator and appends it to the result.
func (w *structWriter) Commit(row RowBuilder) error {
	if err := row.Validate(); err != nil {
		return err
	}

	b, ok := row.(*structRowBuilder)
	if !ok {
		return fmt.Errorf("%w: expected *structRowBuilder", ErrSchemaMismatch)
	}

	w.result = reflect.Append(w.result, b.value)

	return nil
}

// Build returns the accumulated slice as []T.
func (w *structWriter) Build() any {
	return w.result.Interface()
}

// structRowBuilder wraps a single addressable struct value during import.
type structRowBuilder struct {
	typ   reflect.Type
	value reflect.Value
}

// Set assigns the parsed value to the field addressed by col.Index. Passing
// nil is a no-op because new rows already start at the zero value.
func (b *structRowBuilder) Set(column *Column, value any) error {
	if len(column.Index) == 0 {
		return fmt.Errorf("%w: struct column %q has no Index", ErrSchemaMismatch, column.Key)
	}

	// fieldByIndexAlloc allocates intermediate nil embedded pointers along the
	// dive path so that Set into a pointer-embedded field does not panic the way
	// reflect.Value.FieldByIndex would.
	field := fieldByIndexAlloc(b.value, column.Index)
	if !field.CanSet() {
		return fmt.Errorf("%w: %s", ErrUnsetField, column.Key)
	}

	if value == nil {
		return nil
	}

	rv := reflect.ValueOf(value)
	if !rv.Type().AssignableTo(field.Type()) {
		if !rv.Type().ConvertibleTo(field.Type()) {
			return fmt.Errorf("%w: cannot assign %s to %s", ErrSchemaMismatch, rv.Type(), field.Type())
		}

		rv = rv.Convert(field.Type())
	}

	field.Set(rv)

	return nil
}

// Validate delegates to the framework validator (honoring `validate` tags).
func (b *structRowBuilder) Validate() error {
	return validator.Validate(b.value.Interface())
}

// Value returns the underlying struct value.
func (b *structRowBuilder) Value() any {
	return b.value.Interface()
}

// fieldByIndexAlloc resolves the field addressed by a (possibly multi-segment)
// index path, allocating any nil embedded pointers encountered along the way.
// It mirrors reflect.Value.FieldByIndex for single-segment paths but, unlike
// the stdlib, never panics when a dive descends through a nil pointer because
// it initializes those pointers before stepping into them.
func fieldByIndexAlloc(v reflect.Value, index []int) reflect.Value {
	for i, idx := range index {
		if i > 0 {
			for v.Kind() == reflect.Pointer {
				if v.IsNil() {
					v.Set(reflect.New(v.Type().Elem()))
				}

				v = v.Elem()
			}
		}

		v = v.Field(idx)
	}

	return v
}
