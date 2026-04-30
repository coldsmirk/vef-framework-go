package excel

import (
	"fmt"
	"io"

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
		trimSpace:  true,
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
		if i.options.sheetIndex < 0 || i.options.sheetIndex >= len(sheets) {
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

	var columnMapping tabular.ColumnMapping

	if i.options.hasHeader {
		rawMapping, mappingErr := tabular.BuildHeaderMapping(
			rows[i.options.skipRows], schema, tabular.MappingOptions{TrimSpace: i.options.trimSpace},
		)
		if mappingErr != nil {
			return nil, nil, fmt.Errorf("build column mapping: %w", mappingErr)
		}

		columnMapping = tabular.NewColumnMapping(rawMapping)
		dataStartIndex++
	} else {
		columnMapping = tabular.NewColumnMapping(tabular.DefaultPositionalMapping(schema))
	}

	dataRows := rows[dataStartIndex:]
	writer := i.adapter.Writer(len(dataRows))

	var importErrors []tabular.ImportError

	for rowIndex, row := range dataRows {
		// 1-based row number that accounts for skipped rows and the header row,
		// matching what a user sees in a spreadsheet.
		excelRow := dataStartIndex + rowIndex + 1

		// Trim whitespace for empty-row detection so that rows containing only
		// spaces are skipped.
		if tabular.IsEmptyRow(row, i.options.trimSpace) {
			continue
		}

		builder := writer.NewRow()

		rowErrors := tabular.ParseRow(row, columnMapping, schema, builder, i.parsers, excelRow, tabular.ParseRowOptions{TrimSpace: i.options.trimSpace})
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
