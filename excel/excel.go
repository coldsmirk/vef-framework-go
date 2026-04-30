package excel

import (
	"github.com/coldsmirk/vef-framework-go/tabular"
)

// NewImporterFor creates an Excel importer bound to struct type T.
func NewImporterFor[T any](opts ...ImportOption) tabular.Importer {
	return NewImporter(tabular.NewStructAdapterFor[T](), opts...)
}

// NewExporterFor creates an Excel exporter bound to struct type T.
func NewExporterFor[T any](opts ...ExportOption) tabular.Exporter {
	return NewExporter(tabular.NewStructAdapterFor[T](), opts...)
}

// NewMapImporter creates an Excel importer that parses rows into
// []map[string]any using the provided dynamic column specs.
func NewMapImporter(
	specs []tabular.ColumnSpec, opts ...ImportOption,
) (tabular.Importer, error) {
	adapter, err := buildMapAdapter(specs, nil)
	if err != nil {
		return nil, err
	}

	return NewImporter(adapter, opts...), nil
}

// NewMapImporterWithOptions allows passing MapAdapter options (e.g. row validators).
func NewMapImporterWithOptions(
	specs []tabular.ColumnSpec, mapOpts []tabular.MapOption, opts ...ImportOption,
) (tabular.Importer, error) {
	adapter, err := buildMapAdapter(specs, mapOpts)
	if err != nil {
		return nil, err
	}

	return NewImporter(adapter, opts...), nil
}

// NewMapExporter creates an Excel exporter that writes []map[string]any rows
// using the provided dynamic column specs.
func NewMapExporter(
	specs []tabular.ColumnSpec, opts ...ExportOption,
) (tabular.Exporter, error) {
	adapter, err := buildMapAdapter(specs, nil)
	if err != nil {
		return nil, err
	}

	return NewExporter(adapter, opts...), nil
}

// buildMapAdapter constructs a schema from specs and wraps it in a MapAdapter.
func buildMapAdapter(specs []tabular.ColumnSpec, mapOpts []tabular.MapOption) (tabular.RowAdapter, error) {
	schema, err := tabular.NewSchemaFromSpecs(specs)
	if err != nil {
		return nil, err
	}

	return tabular.NewMapAdapter(schema, mapOpts...), nil
}
