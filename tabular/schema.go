package tabular

import (
	"cmp"
	"reflect"
	"slices"
)

// CellValidator validates a single cell value within the context of a column.
// It is primarily used by dynamic (map-based) schemas; struct schemas typically
// rely on the standard `validate` tag via the framework validator.
type CellValidator func(column *Column, value any) error

// RowValidator validates a complete map-shaped row after all cells have been set.
// It is used by MapAdapter; struct schemas validate the whole struct instead.
type RowValidator func(row map[string]any) error

// Schema contains the pre-parsed tabular metadata describing the columns of
// a tabular row model. It is produced either from a struct type or from a
// []ColumnSpec describing dynamic columns.
type Schema struct {
	columns []*Column
	byKey   map[string]*Column
	byName  map[string]*Column
}

// Column represents metadata for a single column in tabular data. Columns are
// shared between struct-based and dynamic schemas.
//
// External code should construct Column instances through schema factories
// (NewSchema, NewSchemaFor, NewSchemaFromSpecs); fields are exported so that
// adapters can populate them, not for direct mutation by callers.
type Column struct {
	// Key is the logical identifier of the column. For struct schemas it is the
	// field name; for dynamic schemas it is the map key that addresses the cell.
	Key string
	// Name is the header text that appears in the exported file and is matched
	// against the header row during import. Defaults to Key when empty.
	Name string
	// Type is the target Go type used when parsing cell values. For struct
	// schemas it is derived from the struct field type; for dynamic schemas it
	// must be provided via ColumnSpec.
	Type reflect.Type
	// Order controls the column order. Columns are sorted stably by Order.
	Order int
	// Width hints the column width for formats that support it (Excel).
	Width float64
	// Default is the value used during import when the source cell is empty.
	Default string
	// Format is the format template consumed by the default Formatter/Parser,
	// e.g. "2006-01-02" for dates or "%.2f" for floats.
	Format string
	// Formatter is the name of a custom Formatter registered on the exporter.
	Formatter string
	// Parser is the name of a custom ValueParser registered on the importer.
	Parser string
	// FormatterFn is a Formatter instance bound directly on the column, taking
	// precedence over the named Formatter registry.
	FormatterFn Formatter
	// ParserFn is a ValueParser instance bound directly on the column, taking
	// precedence over the named ValueParser registry.
	ParserFn ValueParser
	// Required marks the column as mandatory; empty cells trigger a validation
	// error during import. Applies to dynamic schemas.
	Required bool
	// Validators is an ordered list of custom cell validators. Applies to
	// dynamic schemas.
	Validators []CellValidator
	// Index is the struct field index path used by StructAdapter. nil for
	// dynamic columns.
	Index []int
}

// NewSchema creates a Schema instance by parsing struct fields with tabular tags.
func NewSchema(typ reflect.Type) *Schema {
	return newSchema(parseStruct(typ))
}

// NewSchemaFor creates a Schema instance from type T.
func NewSchemaFor[T any]() *Schema {
	return NewSchema(reflect.TypeFor[T]())
}

// newSchema sorts the columns stably by Order and builds lookup indexes.
func newSchema(columns []*Column) *Schema {
	slices.SortStableFunc(columns, func(a, b *Column) int {
		return cmp.Compare(a.Order, b.Order)
	})

	byKey := make(map[string]*Column, len(columns))
	byName := make(map[string]*Column, len(columns))

	for _, column := range columns {
		if column.Key != "" {
			byKey[column.Key] = column
		}

		if column.Name != "" {
			byName[column.Name] = column
		}
	}

	return &Schema{columns: columns, byKey: byKey, byName: byName}
}

// Columns returns all columns in the schema.
func (s *Schema) Columns() []*Column {
	return s.columns
}

// ColumnCount returns the number of columns.
func (s *Schema) ColumnCount() int {
	return len(s.columns)
}

// ColumnNames returns all column header names.
func (s *Schema) ColumnNames() []string {
	names := make([]string, len(s.columns))
	for i, column := range s.columns {
		names[i] = column.Name
	}

	return names
}

// ColumnByKey returns the column addressed by the given logical key.
func (s *Schema) ColumnByKey(key string) (*Column, bool) {
	column, ok := s.byKey[key]

	return column, ok
}

// ColumnByName returns the column with the given header name.
func (s *Schema) ColumnByName(name string) (*Column, bool) {
	column, ok := s.byName[name]

	return column, ok
}
