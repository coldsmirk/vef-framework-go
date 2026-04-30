package tabular

import (
	"fmt"
	"reflect"

	"github.com/coldsmirk/go-collections"
)

// ColumnSpec describes a dynamic column at runtime. It is converted to a
// Column by NewSchemaFromSpecs.
type ColumnSpec struct {
	// Key is the logical identifier and the map key used to read/write cell
	// values. Required and must be unique within the schema.
	Key string
	// Name is the header text. Defaults to Key when empty.
	Name string
	// Type is the Go target type used for parsing. Required.
	Type reflect.Type
	// Order controls the column order; columns are sorted stably.
	Order int
	// Width hints the column width for Excel export.
	Width float64
	// Default is used during import when the cell is empty.
	Default string
	// Format template consumed by the default Formatter/Parser.
	Format string
	// Formatter / Parser names resolved against the exporter / importer
	// registries.
	Formatter string
	Parser    string
	// FormatterFn / ParserFn bind implementations directly; they take
	// precedence over the named lookups.
	FormatterFn Formatter
	ParserFn    ValueParser
	// Required marks the column as mandatory during import.
	Required bool
	// Validators runs additional cell validation after parsing.
	Validators []CellValidator
}

// NewMapAdapterFromSpecs is a convenience helper that turns dynamic column
// specs directly into a MapAdapter, applying any MapAdapter options.
func NewMapAdapterFromSpecs(specs []ColumnSpec, opts ...MapOption) (RowAdapter, error) {
	schema, err := NewSchemaFromSpecs(specs)
	if err != nil {
		return nil, err
	}

	return NewMapAdapter(schema, opts...), nil
}

// NewSchemaFromSpecs builds a Schema from a slice of dynamic column specs.
// It validates that every spec has a non-empty Key and Type, and that keys
// are unique.
func NewSchemaFromSpecs(specs []ColumnSpec) (*Schema, error) {
	columns := make([]*Column, len(specs))
	seen := collections.NewHashSet[string]()

	for i, spec := range specs {
		if spec.Key == "" {
			return nil, fmt.Errorf("%w: spec #%d", ErrMissingColumnKey, i)
		}

		if spec.Type == nil {
			return nil, fmt.Errorf("%w: %s", ErrMissingColumnType, spec.Key)
		}

		if seen.Contains(spec.Key) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateColumnName, spec.Key)
		}

		seen.Add(spec.Key)

		name := spec.Name
		if name == "" {
			name = spec.Key
		}

		columns[i] = &Column{
			Key:         spec.Key,
			Name:        name,
			Type:        spec.Type,
			Order:       spec.Order,
			Width:       spec.Width,
			Default:     spec.Default,
			Format:      spec.Format,
			Formatter:   spec.Formatter,
			Parser:      spec.Parser,
			FormatterFn: spec.FormatterFn,
			ParserFn:    spec.ParserFn,
			Required:    spec.Required,
			Validators:  spec.Validators,
		}
	}

	return newSchema(columns), nil
}
