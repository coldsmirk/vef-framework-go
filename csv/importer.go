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

// Import reads CSV data from an io.Reader.
func (i *importer) Import(reader io.Reader) (any, []tabular.ImportError, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = i.options.delimiter
	csvReader.TrimLeadingSpace = i.options.trimSpace
	csvReader.Comment = i.options.comment
	csvReader.FieldsPerRecord = -1

	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read CSV: %w", err)
	}

	minRows := i.options.skipRows
	if i.options.hasHeader {
		minRows++
	}

	if len(rows) <= minRows {
		return nil, nil, fmt.Errorf("%w (total rows: %d, skip rows: %d, has header: %v)",
			tabular.ErrNoDataRowsFound, len(rows), i.options.skipRows, i.options.hasHeader)
	}

	schema := i.adapter.Schema()
	dataStartIndex := i.options.skipRows

	var columnMapping map[int]int

	if i.options.hasHeader {
		mappingOpts := tabular.MappingOptions{TrimSpace: i.options.trimSpace}

		mapping, mappingErr := tabular.BuildHeaderMapping(rows[i.options.skipRows], schema, mappingOpts)
		if mappingErr != nil {
			return nil, nil, fmt.Errorf("build column mapping: %w", mappingErr)
		}

		columnMapping = mapping
		dataStartIndex++
	} else {
		columnMapping = tabular.DefaultPositionalMapping(schema)
	}

	dataRows := rows[dataStartIndex:]
	writer := i.adapter.Writer(len(dataRows))

	parseOpts := tabular.ParseRowOptions{TrimSpace: i.options.trimSpace}

	var importErrors []tabular.ImportError

	for rowIdx, row := range dataRows {
		csvRow := dataStartIndex + rowIdx + 1

		if tabular.IsEmptyRow(row, i.options.trimSpace) {
			continue
		}

		builder := writer.NewRow()

		rowErrors := tabular.ParseRow(row, columnMapping, schema, builder, i.parsers, csvRow, parseOpts)
		if len(rowErrors) > 0 {
			importErrors = append(importErrors, rowErrors...)

			continue
		}

		if err := writer.Commit(builder); err != nil {
			importErrors = append(importErrors, tabular.ImportError{
				Row: csvRow,
				Err: fmt.Errorf("validation failed: %w", err),
			})

			continue
		}
	}

	return writer.Build(), importErrors, nil
}
