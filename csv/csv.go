package csv

import (
	"github.com/coldsmirk/vef-framework-go/tabular"
)

// NewImporterFor creates a CSV importer bound to struct type T.
func NewImporterFor[T any](opts ...ImportOption) tabular.Importer {
	return NewImporter(tabular.NewStructAdapterFor[T](), opts...)
}

// NewExporterFor creates a CSV exporter bound to struct type T.
func NewExporterFor[T any](opts ...ExportOption) tabular.Exporter {
	return NewExporter(tabular.NewStructAdapterFor[T](), opts...)
}

// NewTypedImporterFor creates a CSV importer bound to struct type T that
// returns []T directly, eliminating the need for a type assertion.
func NewTypedImporterFor[T any](opts ...ImportOption) tabular.TypedImporter[T] {
	return tabular.NewTypedImporter[T](NewImporterFor[T](opts...))
}

// NewTypedExporterFor creates a CSV exporter bound to struct type T that
// accepts []T directly, eliminating the need for an any-typed argument.
func NewTypedExporterFor[T any](opts ...ExportOption) tabular.TypedExporter[T] {
	return tabular.NewTypedExporter[T](NewExporterFor[T](opts...))
}

// NewMapImporter creates a CSV importer that parses rows into []map[string]any
// using the provided dynamic column specs. Pass nil for mapOpts when no
// MapAdapter options (e.g. row validators) are needed.
func NewMapImporter(
	specs []tabular.ColumnSpec, mapOpts []tabular.MapOption, opts ...ImportOption,
) (tabular.Importer, error) {
	adapter, err := tabular.NewMapAdapterFromSpecs(specs, mapOpts...)
	if err != nil {
		return nil, err
	}

	return NewImporter(adapter, opts...), nil
}

// NewMapExporter creates a CSV exporter that writes []map[string]any rows
// using the provided dynamic column specs.
func NewMapExporter(
	specs []tabular.ColumnSpec, opts ...ExportOption,
) (tabular.Exporter, error) {
	adapter, err := tabular.NewMapAdapterFromSpecs(specs)
	if err != nil {
		return nil, err
	}

	return NewExporter(adapter, opts...), nil
}
