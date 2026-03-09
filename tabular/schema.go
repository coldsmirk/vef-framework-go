package tabular

import (
	"bytes"
	"cmp"
	"io"
	"reflect"
	"slices"

	"github.com/coldsmirk/vef-framework-go/internal/log"
)

// Importer defines the interface for importing tabular data.
// It provides methods to read and parse tabular data from various sources.
type Importer interface {
	// RegisterParser registers a custom parser with the given name.
	RegisterParser(name string, parser ValueParser)
	// ImportFromFile imports data from an Excel file.
	ImportFromFile(filename string) (any, []ImportError, error)
	// Import imports data from an io.Reader.
	Import(reader io.Reader) (any, []ImportError, error)
}

// Exporter defines the interface for exporting tabular data.
// It provides methods to write and format tabular data to various destinations.
type Exporter interface {
	// RegisterFormatter registers a custom formatter with the given name.
	RegisterFormatter(name string, formatter Formatter)
	// ExportToFile exports data to an Excel file.
	ExportToFile(data any, filename string) error
	// Export exports data to a bytes.Buffer.
	Export(data any) (*bytes.Buffer, error)
}

var logger = log.Named("tabular")

// Schema contains the pre-parsed tabular metadata for a struct type.
type Schema struct {
	columns []*Column
}

// Column represents metadata for a single column in tabular data.
type Column struct {
	Index     []int   // Field index path in the struct
	Name      string  // Column name (header)
	Width     float64 // Column width hint (for display/export)
	Order     int     // Column order (for sorting)
	Default   string  // Default value used during import when cell is empty
	Format    string  // Format template (e.g., date format, number format)
	Formatter string  // Custom formatter name for export
	Parser    string  // Custom parser name for import
}

// NewSchema creates a Schema instance by parsing struct fields with tabular tags.
func NewSchema(typ reflect.Type) *Schema {
	columns := parseStruct(typ)

	slices.SortStableFunc(columns, func(a, b *Column) int {
		return cmp.Compare(a.Order, b.Order)
	})

	return &Schema{columns: columns}
}

// NewSchemaFor creates a Schema instance from type T.
func NewSchemaFor[T any]() *Schema {
	return NewSchema(reflect.TypeFor[T]())
}

// Columns returns all columns in the schema.
func (s *Schema) Columns() []*Column {
	return s.columns
}

// ColumnCount returns the number of columns.
func (s *Schema) ColumnCount() int {
	return len(s.columns)
}

// ColumnNames returns all column names.
func (s *Schema) ColumnNames() []string {
	names := make([]string, len(s.columns))
	for i, col := range s.columns {
		names[i] = col.Name
	}

	return names
}
