package excel

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

// exporter writes rows into an Excel workbook via a tabular.RowAdapter.
type exporter struct {
	adapter    tabular.RowAdapter
	formatters map[string]tabular.Formatter
	options    exportConfig
}

// NewExporter creates an Excel exporter driven by the provided RowAdapter.
func NewExporter(adapter tabular.RowAdapter, opts ...ExportOption) tabular.Exporter {
	options := exportConfig{
		sheetName: "Sheet1",
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &exporter{
		adapter:    adapter,
		formatters: make(map[string]tabular.Formatter),
		options:    options,
	}
}

// RegisterFormatter registers a named formatter referenced by Column.Formatter.
func (e *exporter) RegisterFormatter(name string, formatter tabular.Formatter) {
	e.formatters[name] = formatter
}

// ExportToFile saves the exported Excel workbook to the given file path.
func (e *exporter) ExportToFile(data any, filename string) error {
	f, err := e.doExport(data)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file: %v", closeErr)
		}
	}()

	if err := f.SaveAs(filename); err != nil {
		return fmt.Errorf("save file %s: %w", filename, err)
	}

	return nil
}

// Export writes the workbook to a bytes.Buffer.
func (e *exporter) Export(data any) (*bytes.Buffer, error) {
	f, err := e.doExport(data)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file after export: %v", closeErr)
		}
	}()

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write to buffer: %w", err)
	}

	return buf, nil
}

func (e *exporter) doExport(data any) (*excelize.File, error) {
	f := excelize.NewFile()

	sheetIndex, err := f.GetSheetIndex(e.options.sheetName)
	if err != nil {
		return nil, fmt.Errorf("get sheet index: %w", err)
	}

	if sheetIndex == -1 {
		sheetIndex, err = f.NewSheet(e.options.sheetName)
		if err != nil {
			return nil, fmt.Errorf("create sheet: %w", err)
		}
	}

	if err := e.writeHeader(f, e.options.sheetName); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	if err := e.writeData(f, e.options.sheetName, data); err != nil {
		return nil, fmt.Errorf("write data: %w", err)
	}

	f.SetActiveSheet(sheetIndex)

	return f, nil
}

func (e *exporter) writeHeader(f *excelize.File, sheetName string) error {
	columns := e.adapter.Schema().Columns()

	for colIdx, col := range columns {
		colLetter, err := excelize.ColumnNumberToName(colIdx + 1)
		if err != nil {
			return fmt.Errorf("convert column number to name: %w", err)
		}

		cell := fmt.Sprintf("%s1", colLetter)
		if err := f.SetCellValue(sheetName, cell, col.Name); err != nil {
			return fmt.Errorf("set header cell %s: %w", cell, err)
		}

		if col.Width > 0 {
			if err := f.SetColWidth(sheetName, colLetter, colLetter, col.Width); err != nil {
				return fmt.Errorf("set column width for %s: %w", colLetter, err)
			}
		}
	}

	return nil
}

func (e *exporter) writeData(f *excelize.File, sheetName string, data any) error {
	columns := e.adapter.Schema().Columns()

	reader, err := e.adapter.Reader(data)
	if err != nil {
		return err
	}

	for rowIdx, view := range reader.All() {
		excelRow := rowIdx + 2

		for colIdx, col := range columns {
			raw, err := view.Get(col)
			if err != nil {
				return tabular.ExportError{
					Row:    rowIdx,
					Column: col.Name,
					Field:  col.Key,
					Err:    fmt.Errorf("read cell: %w", err),
				}
			}

			cellValue, err := tabular.ResolveFormatter(col, e.formatters).Format(raw)
			if err != nil {
				return tabular.ExportError{
					Row:    rowIdx,
					Column: col.Name,
					Field:  col.Key,
					Err:    fmt.Errorf("format value: %w", err),
				}
			}

			colLetter, err := excelize.ColumnNumberToName(colIdx + 1)
			if err != nil {
				return fmt.Errorf("convert column number to name: %w", err)
			}

			cell := fmt.Sprintf("%s%d", colLetter, excelRow)
			if err := f.SetCellValue(sheetName, cell, cellValue); err != nil {
				return fmt.Errorf("set cell %s: %w", cell, err)
			}
		}
	}

	return nil
}
