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
func (a *structAdapter) Reader(data any) (RowReader, error) {
	if data == nil {
		return &structReader{}, nil
	}

	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("%w, got %s", ErrDataMustBeSlice, dataValue.Kind())
	}

	// Element type must be assignable to the adapter's struct type (or a
	// pointer to it). We allow []any when the underlying element values can
	// still be resolved via reflection at read time.
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

			if !yield(i, &structRowView{elem: elem}) {
				return
			}
		}
	}
}

// structRowView exposes struct fields using Column.Index.
type structRowView struct {
	elem reflect.Value
}

// Get reads the struct field addressed by col.Index.
func (v *structRowView) Get(column *Column) (any, error) {
	if !v.elem.IsValid() {
		return nil, nil
	}

	if len(column.Index) == 0 {
		return nil, fmt.Errorf("%w: struct column %q has no Index", ErrSchemaMismatch, column.Key)
	}

	field := v.elem.FieldByIndex(column.Index)

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

// Set assigns the parsed value to the field addressed by col.Index.
func (b *structRowBuilder) Set(column *Column, value any) error {
	if len(column.Index) == 0 {
		return fmt.Errorf("%w: struct column %q has no Index", ErrSchemaMismatch, column.Key)
	}

	field := b.value.FieldByIndex(column.Index)
	if !field.CanSet() {
		return fmt.Errorf("%w: %s", ErrUnsetField, column.Key)
	}

	if value == nil {
		field.Set(reflect.Zero(field.Type()))

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
