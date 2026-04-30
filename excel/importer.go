package excel

import (
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/tabular"
)

var logger = logx.Named("excel")

// importer reads Excel rows into values produced by a tabular.RowAdapter.
type importer struct {
	adapter tabular.RowAdapter
	parsers map[string]tabular.ValueParser
	options importConfig
}

// NewImporter creates an Excel importer driven by the provided RowAdapter.
func NewImporter(adapter tabular.RowAdapter, opts ...ImportOption) tabular.Importer {
	options := importConfig{
		sheetIndex: 0,
		hasHeader:  true,
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

// ImportFromFile reads Excel data from a file and parses it via the adapter.
func (i *importer) ImportFromFile(filename string) (any, []tabular.ImportError, error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("open Excel file %s: %w", filename, err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file %s: %v", filename, closeErr)
		}
	}()

	return i.doImport(f)
}

// Import reads Excel data from an io.Reader.
func (i *importer) Import(reader io.Reader) (any, []tabular.ImportError, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("open Excel from reader: %w", err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file from reader: %v", closeErr)
		}
	}()

	return i.doImport(f)
}

func (i *importer) doImport(f *excelize.File) (any, []tabular.ImportError, error) {
	sheetName := i.options.sheetName
	if sheetName == "" {
		sheets := f.GetSheetList()
		if i.options.sheetIndex >= len(sheets) {
			return nil, nil, fmt.Errorf("%w: %d (total sheets: %d)",
				ErrSheetIndexOutOfRange, i.options.sheetIndex, len(sheets))
		}

		sheetName = sheets[i.options.sheetIndex]
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("get rows: %w", err)
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
		mapping, mappingErr := tabular.BuildHeaderMapping(
			rows[i.options.skipRows], schema, tabular.MappingOptions{TrimSpace: true},
		)
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

	var importErrors []tabular.ImportError

	for rowIdx, row := range dataRows {
		excelRow := dataStartIndex + rowIdx + 1

		if i.isEmptyRow(row) {
			continue
		}

		builder := writer.NewRow()

		rowErrors := i.parseRow(row, columnMapping, schema, builder, excelRow)
		if len(rowErrors) > 0 {
			importErrors = append(importErrors, rowErrors...)

			continue
		}

		if err := writer.Commit(builder); err != nil {
			importErrors = append(importErrors, tabular.ImportError{
				Row: excelRow,
				Err: fmt.Errorf("validation failed: %w", err),
			})

			continue
		}
	}

	return writer.Build(), importErrors, nil
}

func (i *importer) parseRow(
	row []string, columnMapping map[int]int, schema *tabular.Schema,
	builder tabular.RowBuilder, excelRow int,
) []tabular.ImportError {
	var errors []tabular.ImportError

	columns := schema.Columns()

	// Iterate by sorted source index so per-row error order is deterministic.
	for _, excelIndex := range slices.Sorted(maps.Keys(columnMapping)) {
		schemaIndex := columnMapping[excelIndex]
		column := columns[schemaIndex]

		var cellValue string
		if excelIndex < len(row) {
			cellValue = row[excelIndex]
		}

		if cellValue == "" && column.Default != "" {
			cellValue = column.Default
		}

		// Skip truly empty cells so adapters (e.g. MapAdapter) can distinguish
		// absent values from explicitly zero ones and enforce Required.
		if cellValue == "" {
			continue
		}

		value, err := tabular.ResolveParser(column, i.parsers).Parse(cellValue, column.Type)
		if err != nil {
			errors = append(errors, tabular.ImportError{
				Row:    excelRow,
				Column: column.Name,
				Field:  column.Key,
				Err:    fmt.Errorf("parse value: %w", err),
			})

			continue
		}

		if err := builder.Set(column, value); err != nil {
			errors = append(errors, tabular.ImportError{
				Row:    excelRow,
				Column: column.Name,
				Field:  column.Key,
				Err:    err,
			})

			continue
		}
	}

	return errors
}

func (*importer) isEmptyRow(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}

	return true
}
