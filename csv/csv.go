package csv

import (
	"reflect"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

func NewImporterFor[T any](opts ...ImportOption) tabular.Importer {
	return NewImporter(reflect.TypeFor[T](), opts...)
}

func NewExporterFor[T any](opts ...ExportOption) tabular.Exporter {
	return NewExporter(reflect.TypeFor[T](), opts...)
}
