package tabular

import (
	"errors"
	"fmt"
	"iter"
	"reflect"
)

// MapOption customizes MapAdapter behavior.
type MapOption func(*mapAdapter)

// WithRowValidator registers a validator that runs after all cells have been
// populated for a map-shaped row.
func WithRowValidator(validator RowValidator) MapOption {
	return func(a *mapAdapter) {
		a.rowValidator = validator
	}
}

// mapAdapter bridges []map[string]any rows with the tabular engine. Columns
// are addressed via their Key.
type mapAdapter struct {
	schema       *Schema
	rowValidator RowValidator
}

// NewMapAdapter creates a RowAdapter driven by the supplied Schema. The schema
// is typically built from a []ColumnSpec via NewSchemaFromSpecs.
func NewMapAdapter(schema *Schema, opts ...MapOption) RowAdapter {
	a := &mapAdapter{schema: schema}
	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Schema returns the columns the adapter operates on.
func (a *mapAdapter) Schema() *Schema {
	return a.schema
}

// Reader accepts []map[string]any (or compatible wrappers) and yields row views.
func (*mapAdapter) Reader(data any) (RowReader, error) {
	if data == nil {
		return &mapReader{}, nil
	}

	// Fast path: concrete slice of maps.
	if rows, ok := data.([]map[string]any); ok {
		return &mapReader{rows: rows}, nil
	}

	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("%w, got %s", ErrDataMustBeSlice, dataValue.Kind())
	}

	rows := make([]map[string]any, 0, dataValue.Len())

	for i := range dataValue.Len() {
		elem := dataValue.Index(i).Interface()
		m, ok := elem.(map[string]any)

		if !ok {
			return nil, fmt.Errorf("%w: element %d is %T, want map[string]any",
				ErrSchemaMismatch, i, elem)
		}

		rows = append(rows, m)
	}

	return &mapReader{rows: rows}, nil
}

// Writer allocates an empty []map[string]any accumulator.
func (a *mapAdapter) Writer(capacity int) RowWriter {
	if capacity < 0 {
		capacity = 0
	}

	return &mapWriter{
		schema:       a.schema,
		rowValidator: a.rowValidator,
		result:       make([]map[string]any, 0, capacity),
	}
}

// mapReader iterates the pre-collected slice of maps.
type mapReader struct {
	rows []map[string]any
}

// All yields each map wrapped as a mapRowView.
func (r *mapReader) All() iter.Seq2[int, RowView] {
	return func(yield func(int, RowView) bool) {
		for i, row := range r.rows {
			if !yield(i, &mapRowView{row: row}) {
				return
			}
		}
	}
}

// mapRowView reads cells from a map using Column.Key.
type mapRowView struct {
	row map[string]any
}

// Get returns the map entry for col.Key or nil when the key is missing.
func (v *mapRowView) Get(col *Column) (any, error) {
	if col.Key == "" {
		return nil, fmt.Errorf("%w: dynamic column has empty Key", ErrSchemaMismatch)
	}

	value, ok := v.row[col.Key]
	if !ok {
		return nil, nil
	}

	return value, nil
}

// mapWriter accumulates []map[string]any rows.
type mapWriter struct {
	schema       *Schema
	rowValidator RowValidator
	result       []map[string]any
}

// NewRow allocates an empty map, sized for schema capacity.
func (w *mapWriter) NewRow() RowBuilder {
	return &mapRowBuilder{
		schema:       w.schema,
		rowValidator: w.rowValidator,
		row:          make(map[string]any, len(w.schema.Columns())),
	}
}

// Commit validates and appends the row.
func (w *mapWriter) Commit(row RowBuilder) error {
	if err := row.Validate(); err != nil {
		return err
	}

	b, ok := row.(*mapRowBuilder)
	if !ok {
		return fmt.Errorf("%w: expected *mapRowBuilder", ErrSchemaMismatch)
	}

	w.result = append(w.result, b.row)

	return nil
}

// Build returns the accumulated slice.
func (w *mapWriter) Build() any {
	return w.result
}

// mapRowBuilder writes cells into a map and runs column / row validators.
type mapRowBuilder struct {
	schema       *Schema
	rowValidator RowValidator
	row          map[string]any
}

// Set writes value into the map at col.Key.
func (b *mapRowBuilder) Set(col *Column, value any) error {
	if col.Key == "" {
		return fmt.Errorf("%w: dynamic column has empty Key", ErrSchemaMismatch)
	}

	b.row[col.Key] = value

	return nil
}

// Validate enforces Required / Validators for every schema column and runs the
// optional row-level validator. All errors are joined together.
func (b *mapRowBuilder) Validate() error {
	var errs []error

	for _, col := range b.schema.Columns() {
		value, present := b.row[col.Key]

		if col.Required && isEmptyValue(value, present) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrRequiredMissing, col.Key))

			continue
		}

		for _, validate := range col.Validators {
			if err := validate(col, value); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", col.Key, err))
			}
		}
	}

	if b.rowValidator != nil {
		if err := b.rowValidator(b.row); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Value returns the map currently held by the builder.
func (b *mapRowBuilder) Value() any {
	return b.row
}

// isEmptyValue reports whether a cell should be considered empty for required
// checks. It treats missing map keys, nil values and zero strings as empty.
func isEmptyValue(value any, present bool) bool {
	if !present || value == nil {
		return true
	}

	if s, ok := value.(string); ok && s == "" {
		return true
	}

	return false
}
