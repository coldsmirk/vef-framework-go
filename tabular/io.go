package tabular

import (
	"bytes"
	"io"
)

// Importer reads tabular data (CSV, Excel, ...) Into a slice of rows produced
// by a RowAdapter. The shape of the returned data depends on the adapter:
// struct adapters return []T, map adapters return []map[string]any.
type Importer interface {
	// RegisterParser registers a custom parser referenced by Column.Parser.
	RegisterParser(name string, parser ValueParser)
	// ImportFromFile imports data from a tabular file on disk.
	ImportFromFile(filename string) (any, []ImportError, error)
	// Import imports data from an io.Reader carrying tabular content.
	Import(reader io.Reader) (any, []ImportError, error)
}

// Exporter writes a slice of rows (struct or map) to a tabular destination
// (CSV, Excel, ...) Using a RowAdapter to read from the input.
type Exporter interface {
	// RegisterFormatter registers a custom formatter referenced by Column.Formatter.
	RegisterFormatter(name string, formatter Formatter)
	// ExportToFile exports data to a tabular file on disk.
	ExportToFile(data any, filename string) error
	// Export exports data to a bytes.Buffer carrying tabular content.
	Export(data any) (*bytes.Buffer, error)
}
