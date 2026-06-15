package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/tabular"
)

var logger = logx.Named("csv")

// importer reads CSV rows into values produced by a tabular.RowAdapter.
type importer struct {
	adapter tabular.RowAdapter
	parsers map[string]tabular.ValueParser
	options importConfig
}

// NewImporter creates a CSV importer driven by the provided RowAdapter.
func NewImporter(adapter tabular.RowAdapter, opts ...ImportOption) tabular.Importer {
	options := importConfig{
		delimiter: ',',
		hasHeader: true,
		skipRows:  0,
		trimSpace: true,
		comment:   0,
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &importer{
		adapter: adapter,
		parsers: make(map[string]tabular.ValueParser),
		options: options,
	}
}

// RegisterParser registers a named parser referenced by Column.Parser.
func (i *importer) RegisterParser(name string, parser tabular.ValueParser) {
	i.parsers[name] = parser
}

// ImportFromFile reads CSV data from a file and parses it via the adapter.
func (i *importer) ImportFromFile(filename string) (any, []tabular.ImportError, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("open CSV file %s: %w", filename, err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close CSV file %s: %v", filename, closeErr)
		}
	}()

	return i.Import(f)
}

// Import reads CSV data from an io.Reader. The entire input is read into memory
// before processing, so peak memory scales with the file size in addition to
// the materialized result slice.
func (i *importer) Import(reader io.Reader) (any, []tabular.ImportError, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = i.options.delimiter
	csvReader.Comment = i.options.comment
	csvReader.FieldsPerRecord = -1

	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read CSV: %w", err)
	}

	return tabular.ImportRows(rows, i.adapter, i.parsers, tabular.ImportRowsOptions{
		SkipRows:  i.options.skipRows,
		HasHeader: i.options.hasHeader,
		TrimSpace: i.options.trimSpace,
	})
}
