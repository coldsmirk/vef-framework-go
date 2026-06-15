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

// Import reads Excel data from an io.Reader. excelize loads the whole workbook
// into memory and all sheet rows are read up front, so peak memory scales with
// the file size in addition to the materialized result slice.
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

	return tabular.ImportRows(rows, i.adapter, i.parsers, tabular.ImportRowsOptions{
		SkipRows:  i.options.skipRows,
		HasHeader: i.options.hasHeader,
		TrimSpace: i.options.trimSpace,
	})
}
